package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/aws/eks-anywhere/pkg/api/v1alpha1"
	"github.com/aws/eks-anywhere/pkg/cluster"
	"github.com/aws/eks-anywhere/pkg/config"
	"github.com/aws/eks-anywhere/pkg/constants"
	"github.com/aws/eks-anywhere/pkg/dependencies"
	"github.com/aws/eks-anywhere/pkg/features"
	"github.com/aws/eks-anywhere/pkg/filewriter"
	"github.com/aws/eks-anywhere/pkg/kubeconfig"
	"github.com/aws/eks-anywhere/pkg/logger"
	"github.com/aws/eks-anywhere/pkg/providers"
	"github.com/aws/eks-anywhere/pkg/providers/cloudstack/decoder"
	"github.com/aws/eks-anywhere/pkg/task"
	"github.com/aws/eks-anywhere/pkg/types"
	"github.com/aws/eks-anywhere/pkg/validations"
	"github.com/aws/eks-anywhere/pkg/validations/createvalidations"
	"github.com/aws/eks-anywhere/pkg/workflows/interfaces"
)

type preflightOptions struct {
	clusterOptions
	forceClean            bool
	skipIpCheck           bool
	hardwareCSVPath       string
	tinkerbellBootstrapIP string
	installPackages       string
}

type Validate struct {
	bootstrapper     interfaces.Bootstrapper
	provider         providers.Provider
	clusterManager   interfaces.ClusterManager
	addonManager     interfaces.AddonManager
	writer           filewriter.FileWriter
	eksdInstaller    interfaces.EksdInstaller
	packageInstaller interfaces.PackageInstaller
}

var preOpt = &preflightOptions{}

var preflightCmd = &cobra.Command{
	Use:     "preflight -f <cluster-config-file> [flags]",
	Short:   "Perform preflight checks to validate cluster config",
	Long:    "This command performs preflight validation checks before initializng a cluster on vsphere",
	PreRunE: preRunPreflight,
	RunE:    preOpt.preflight,
}

func init() {
	rootCmd.AddCommand(preflightCmd)
	preflightCmd.Flags().StringVarP(&preOpt.fileName, "filename", "f", "", "Filename that contains EKS-A cluster configuration")
	preflightCmd.Flags().StringVarP(
		&preOpt.hardwareCSVPath,
		TinkerbellHardwareCSVFlagName,
		TinkerbellHardwareCSVFlagAlias,
		"",
		TinkerbellHardwareCSVFlagDescription,
	)
	preflightCmd.Flags().StringVar(&preOpt.tinkerbellBootstrapIP, "tinkerbell-bootstrap-ip", "", "Override the local tinkerbell IP in the bootstrap cluster")
	preflightCmd.Flags().BoolVar(&preOpt.forceClean, "force-cleanup", false, "Force deletion of previously created bootstrap cluster")
	preflightCmd.Flags().BoolVar(&preOpt.skipIpCheck, "skip-ip-check", false, "Skip check for whether cluster control plane ip is in use")
	preflightCmd.Flags().StringVar(&preOpt.bundlesOverride, "bundles-override", "", "Override default Bundles manifest (not recommended)")
	preflightCmd.Flags().StringVar(&preOpt.managementKubeconfig, "kubeconfig", "", "Management cluster kubeconfig file")
	preflightCmd.Flags().StringVar(&preOpt.installPackages, "install-packages", "", "Location of curated packages configuration files to install to the cluster")

	if err := preflightCmd.MarkFlagRequired("filename"); err != nil {
		log.Fatalf("Error marking flag as required: %v", err)
	}
}

func preRunPreflight(cmd *cobra.Command, args []string) error {
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		err := viper.BindPFlag(flag.Name, flag)
		if err != nil {
			log.Fatalf("Error initializing flags: %v", err)
		}
	})
	return nil
}

func (preOpt *preflightOptions) preflight(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	logger.V(0).Info("Beginning preflight validations")

	clusterConfigFileExist := validations.FileExists(preOpt.fileName)
	if !clusterConfigFileExist {
		return fmt.Errorf("the cluster config file %s does not exist", preOpt.fileName)
	}

	clusterConfig, err := v1alpha1.GetAndValidateClusterConfig(preOpt.fileName)
	if err != nil {
		return fmt.Errorf("the cluster config file provided is invalid: %v", err)
	}

	kubeconfigPath := kubeconfig.FromClusterName(clusterConfig.Name)
	if validations.FileExistsAndIsNotEmpty(kubeconfigPath) {
		return fmt.Errorf(
			"old cluster config file exists under %s, please use a different clusterName to proceed",
			clusterConfig.Name,
		)
	}

	// Need to check what this does, looks like it outputs a spec from the file
	clusterSpec, err := newClusterSpec(preOpt.clusterOptions)
	if err != nil {
		return err
	}

	// This looks like it does some maxwaitpermachine config and git checks
	cliConfig := buildCliConfig(clusterSpec)
	// Then this does some flux and cloudstack additional config and gets additional directories
	dirs, err := preOpt.directoriesToMount(clusterSpec, cliConfig)
	if err != nil {
		return err
	}

	// Factory dependency production
	deps, err := dependencies.ForSpec(ctx, clusterSpec).WithExecutableMountDirs(dirs...).
		//WithBootstrapper().
		WithCliConfig(cliConfig).
		WithClusterManager(clusterSpec.Cluster).
		WithProvider(preOpt.fileName, clusterSpec.Cluster, preOpt.skipIpCheck, preOpt.hardwareCSVPath, preOpt.forceClean, preOpt.tinkerbellBootstrapIP).
		//WithFluxAddonClient(clusterSpec.Cluster, clusterSpec.FluxConfig, cliConfig).
		//WithWriter().
		WithEksdInstaller().
		WithPackageInstaller(clusterSpec, preOpt.installPackages).
		SkipWriter(). // Added to skip file writing process
		Build(ctx)
	if err != nil {
		return err
	}
	defer close(ctx, deps)

	// Check Supported Providers
	if !features.IsActive(features.CloudStackProvider()) && deps.Provider.Name() == constants.CloudStackProviderName {
		return fmt.Errorf("provider cloudstack is not supported in this release")
	}

	if !features.IsActive(features.SnowProvider()) && deps.Provider.Name() == constants.SnowProviderName {
		return fmt.Errorf("provider snow is not supported in this release")
	}

	// Create cluster - Define cluster locally insted of using create.go
	createCluster := &Validate{
		//bootstrapper:     deps.Bootstrapper,
		provider:       deps.Provider,
		clusterManager: deps.ClusterManager,
		addonManager:   deps.FluxAddonClient,
		//writer:           deps.Writer,
		eksdInstaller:    deps.EksdInstaller,
		packageInstaller: deps.PackageInstaller,
	}

	// Perform additional checks for management config
	var cluster *types.Cluster
	if clusterSpec.ManagementCluster == nil {
		cluster = &types.Cluster{
			Name:           clusterSpec.Cluster.Name,
			KubeconfigFile: kubeconfig.FromClusterName(clusterSpec.Cluster.Name),
		}
	} else {
		cluster = &types.Cluster{
			Name:           clusterSpec.Cluster.Name,
			KubeconfigFile: clusterSpec.ManagementCluster.KubeconfigFile,
		}
	}

	// Set Validation Options
	validationOpts := &validations.Opts{
		Kubectl: deps.Kubectl,
		Spec:    clusterSpec,
		WorkloadCluster: &types.Cluster{
			Name:           clusterSpec.Cluster.Name,
			KubeconfigFile: kubeconfig.FromClusterName(clusterSpec.Cluster.Name),
		},
		ManagementCluster: cluster,
		Provider:          deps.Provider,
		CliConfig:         cliConfig,
	}

	createValidations := createvalidations.New(validationOpts)

	// Runs Validations
	err = createCluster.Run(ctx, clusterSpec, createValidations, preOpt.forceClean)

	cleanup(deps, &err)
	return err
}

func (preOpt *preflightOptions) directoriesToMount(clusterSpec *cluster.Spec, cliConfig *config.CliConfig) ([]string, error) {
	dirs := preOpt.mountDirs()
	fluxConfig := clusterSpec.FluxConfig
	if fluxConfig != nil && fluxConfig.Spec.Git != nil {
		dirs = append(dirs, filepath.Dir(cliConfig.GitPrivateKeyFile))
		dirs = append(dirs, filepath.Dir(cliConfig.GitKnownHostsFile))
		dirs = append(dirs, filepath.Dir(cc.installPackages))
	}

	if clusterSpec.Config.Cluster.Spec.DatacenterRef.Kind == v1alpha1.CloudStackDatacenterKind {
		env, found := os.LookupEnv(decoder.EksaCloudStackHostPathToMount)
		if found && len(env) > 0 {
			mountDirs := strings.Split(env, ",")
			for _, dir := range mountDirs {
				if _, err := os.Stat(dir); err != nil {
					return nil, fmt.Errorf("invalid host path to mount: %v", err)
				}
				dirs = append(dirs, dir)
			}
		}
	}

	return dirs, nil
}

// Define run function
func (v *Validate) Run(ctx context.Context, clusterSpec *cluster.Spec, validator interfaces.Validator, forceCleanup bool) error {
	if forceCleanup {
		if err := v.bootstrapper.DeleteBootstrapCluster(ctx, &types.Cluster{
			Name: clusterSpec.Cluster.Name,
		}, false); err != nil {
			return err
		}
	}
	commandContext := &task.CommandContext{
		//Bootstrapper:     v.bootstrapper,
		Provider:       v.provider,
		ClusterManager: v.clusterManager,
		AddonManager:   v.addonManager,
		ClusterSpec:    clusterSpec,
		//Writer:           v.writer,
		Validations:      validator,
		EksdInstaller:    v.eksdInstaller,
		PackageInstaller: v.packageInstaller,
	}

	if clusterSpec.ManagementCluster != nil {
		commandContext.BootstrapCluster = clusterSpec.ManagementCluster
	}

	err := task.NewTaskRunner(&ValidateTask{}, v.writer).RunTask(ctx, commandContext)

	return err
}

type ValidateTask struct{}

// This is the actual task called for validation - passed into run function
func (v *ValidateTask) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Performing validate task")
	runner := validations.NewRunner()
	runner.Register(v.providerValidation(ctx, commandContext)...)
	//runner.Register(commandContext.AddonManager.Validations(ctx, commandContext.ClusterSpec)...)
	runner.Register(v.validations(ctx, commandContext)...)

	//runner.PrintValidations()

	err := runner.Run()
	if err != nil {
		commandContext.SetError(err)
		return nil
	}
	return nil
}

// Preflight Validation
func (v *ValidateTask) validations(ctx context.Context, commandContext *task.CommandContext) []validations.Validation {
	return []validations.Validation{
		func() *validations.ValidationResult {
			return &validations.ValidationResult{
				Name: "create preflight validations pass",
				Err:  commandContext.Validations.PreflightValidations(ctx),
			}
		},
	}
}

// Provider Validation
func (v *ValidateTask) providerValidation(ctx context.Context, commandContext *task.CommandContext) []validations.Validation {
	return []validations.Validation{
		func() *validations.ValidationResult {
			return &validations.ValidationResult{
				Name: fmt.Sprintf("%s Provider setup is valid", commandContext.Provider.Name()),
				Err:  commandContext.Provider.SetupAndValidateCreateCluster(ctx, commandContext.ClusterSpec),
			}
		},
	}
}

// Additional required methods on Validate Task, called by runner
func (v *ValidateTask) Name() string {
	return "preflight-validate"
}

func (v *ValidateTask) Restore(ctx context.Context, commandContext *task.CommandContext, completedTask *task.CompletedTask) (task.Task, error) {
	return nil, nil
}

func (v *ValidateTask) Checkpoint() *task.CompletedTask {
	return nil
}
