package cmd

import (
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
	"github.com/aws/eks-anywhere/pkg/kubeconfig"
	"github.com/aws/eks-anywhere/pkg/providers/cloudstack/decoder"
	"github.com/aws/eks-anywhere/pkg/types"
	"github.com/aws/eks-anywhere/pkg/validations"
	"github.com/aws/eks-anywhere/pkg/workflows"
)

type preflightOptions struct {
	clusterOptions
	forceClean            bool
	skipIpCheck           bool
	hardwareCSVPath       string
	tinkerbellBootstrapIP string
	installPackages       string
}

var preOpt = &preflightOptions{}

var preflightCmd = &cobra.Command{
	Use:     "preflight -f <cluster-config-file> [flags]",
	Short:   "Perform preflight checks to validate cluster config",
	Long:    "This command performs preflight validation checks before cluster creation",
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

	// Keeping all input flags in case they affect the command
	// Updated the force-cleanup to true to clean up artifacts
	preflightCmd.Flags().StringVar(&preOpt.tinkerbellBootstrapIP, "tinkerbell-bootstrap-ip", "", "Override the local tinkerbell IP in the bootstrap cluster")
	preflightCmd.Flags().BoolVar(&preOpt.forceClean, "force-cleanup", true, "Force deletion of previously created bootstrap cluster")
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

	// Packaged initial validations and cluster config
	clusterConfig, err := validations.CommonValidation(ctx, preOpt.fileName)

	// Directly calls validations for path
	kubeconfigPath := kubeconfig.FromClusterName(clusterConfig.Name)
	if validations.FileExistsAndIsNotEmpty(kubeconfigPath) {
		return fmt.Errorf(
			"old cluster config file exists under %s, please use a different clusterName to proceed",
			clusterConfig.Name,
		)
	}

	// Generates spec from cluster from options
	// Performs cluster validations
	clusterSpec, err := newClusterSpec(preOpt.clusterOptions)
	if err != nil {
		return err
	}

	// This looks like it does some maxwaitpermachine config and git checks
	//Could be moved locally if preflight specific
	cliConfig := buildCliConfig(clusterSpec)
	// Then this does some flux and cloudstack additional config and gets additional directories
	dirs, err := preOpt.directoriesToMount(clusterSpec, cliConfig)
	if err != nil {
		return err
	}

	// Factory dependency production
	deps, err := dependencies.ForSpec(ctx, clusterSpec).WithExecutableMountDirs(dirs...).
		WithBootstrapper().
		WithCliConfig(cliConfig).
		WithClusterManager(clusterSpec.Cluster).
		WithProvider(preOpt.fileName, clusterSpec.Cluster, preOpt.skipIpCheck, preOpt.hardwareCSVPath, preOpt.forceClean, preOpt.tinkerbellBootstrapIP).
		WithFluxAddonClient(clusterSpec.Cluster, clusterSpec.FluxConfig, cliConfig).
		WithWriter().
		WithEksdInstaller().
		WithPackageInstaller(clusterSpec, preOpt.installPackages).
		Build(ctx)
	if err != nil {
		return err
	}
	defer close(ctx, deps)

	// Check currently supported providers - maybe this should be broken out into something separate
	// so that preflight and create cluster have a single source?
	// Could be added to provider
	if !features.IsActive(features.CloudStackProvider()) && deps.Provider.Name() == constants.CloudStackProviderName {
		return fmt.Errorf("provider cloudstack is not supported in this release")
	}

	if !features.IsActive(features.SnowProvider()) && deps.Provider.Name() == constants.SnowProviderName {
		return fmt.Errorf("provider snow is not supported in this release")
	}

	// Create cluster - Define structure locally insted of using create.go

	// This now uses pkg/workflows/validate
	validateCluster := &workflows.CreateValidator{
		Bootstrapper:     deps.Bootstrapper,
		Provider:         deps.Provider,
		ClusterManager:   deps.ClusterManager,
		AddonManager:     deps.FluxAddonClient,
		Writer:           deps.Writer,
		EksdInstaller:    deps.EksdInstaller,
		PackageInstaller: deps.PackageInstaller,
		Kubectl:          deps.Kubectl,
		CliConfig:        *cliConfig,
	}

	// Specify management cluster config
	var cluster *types.Cluster
	if clusterSpec.ManagementCluster == nil {
		cluster = &types.Cluster{
			Name:           clusterSpec.Cluster.Name,
			KubeconfigFile: kubeconfig.FromClusterName(clusterSpec.Cluster.Name),
		}
	} else {
		cluster = &types.Cluster{
			Name:           clusterSpec.ManagementCluster.Name,
			KubeconfigFile: clusterSpec.ManagementCluster.KubeconfigFile,
		}
	}

	// Update cluster information
	validateCluster.Cluster = *cluster

	// Runs Validations
	_, err = validateCluster.CreateValidations(ctx, clusterSpec, preOpt.forceClean)
	//err = createCluster.Run(ctx, clusterSpec, createValidations, preOpt.forceClean)

	cleanup(deps, &err)
	return err
}

// Mount directories - from cmd/createcluster
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
