package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
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
	"github.com/aws/eks-anywhere/pkg/validations/createvalidations"
	"github.com/aws/eks-anywhere/pkg/workflows"
)

type validateOptions struct {
	clusterOptions
	forceClean            bool
	skipIpCheck           bool
	hardwareCSVPath       string
	tinkerbellBootstrapIP string
	installPackages       string
}

var valOpt = &validateOptions{}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration",
	Long:  "This command is used to validate eksctl anywhere configurations",
	RunE:  valOpt.validateCluster,
}

func init() {
	expCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringVarP(&valOpt.fileName, "filename", "f", "", "Filename that contains EKS-A cluster configuration")

	if err := validateCmd.MarkFlagRequired("filename"); err != nil {
		log.Fatalf("Error marking flag as required: %v", err)
	}
}

func (valOpt *validateOptions) validateCluster(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	clusterConfig, err := workflows.CommonValidation(ctx, valOpt.fileName)
	if err != nil {
		return err
	}

	// Run cluster config validation
	clusterConfigRunner := validations.NewRunner()
	clusterConfigRunner.Register(clusterConfigValidation(valOpt.fileName)...)
	err = clusterConfigRunner.Run()
	if err != nil {
		return err
	}

	// This can be bundled together as a validation
	if clusterConfig.Spec.DatacenterRef.Kind == v1alpha1.TinkerbellDatacenterKind {
		flag := cmd.Flags().Lookup(TinkerbellHardwareCSVFlagName)

		// If no flag was returned there is a developer error as the flag has been removed
		// from the program rendering it invalid.
		if flag == nil {
			panic("'hardwarefile' flag not configured")
		}

		if !viper.IsSet(TinkerbellHardwareCSVFlagName) || viper.GetString(TinkerbellHardwareCSVFlagName) == "" {
			return fmt.Errorf("required flag \"%v\" not set", TinkerbellHardwareCSVFlagName)
		}

		if !validations.FileExists(valOpt.hardwareCSVPath) {
			return fmt.Errorf("hardware config file %s does not exist", valOpt.hardwareCSVPath)
		}
	}

	kubeconfigPath := kubeconfig.FromClusterName(clusterConfig.Name)
	if validations.FileExistsAndIsNotEmpty(kubeconfigPath) {
		return fmt.Errorf(
			"old cluster config file exists under %s, please use a different clusterName to proceed",
			clusterConfig.Name,
		)
	}

	// Run kubeconfig validation
	kubeconfigRunner := validations.NewRunner()
	kubeconfigRunner.Register(kubeconfigValidation(clusterConfig.Name)...)
	err = kubeconfigRunner.Run()
	if err != nil {
		return err
	}

	clusterSpec, err := newClusterSpec(valOpt.clusterOptions)
	if err != nil {
		return err
	}

	// Run cluster spec validation
	clusterSpecRunner := validations.NewRunner()
	clusterSpecRunner.Register(clusterSpecValidation(clusterSpec)...)
	err = clusterSpecRunner.Run()
	if err != nil {
		return err
	}

	cliConfig := buildCliConfig(clusterSpec)
	dirs, err := valOpt.directoriesToMount(clusterSpec, cliConfig)
	if err != nil {
		return err
	}

	deps, err := dependencies.ForSpec(ctx, clusterSpec).WithExecutableMountDirs(dirs...).
		WithCliConfig(cliConfig).
		WithProvider(valOpt.fileName, clusterSpec.Cluster, valOpt.skipIpCheck, valOpt.hardwareCSVPath, valOpt.forceClean, valOpt.tinkerbellBootstrapIP).
		WithFluxAddonClient(clusterSpec.Cluster, clusterSpec.FluxConfig, cliConfig).
		Build(ctx)
	if err != nil {
		return err
	}
	defer close(ctx, deps)

	// Can this be bundled up via a map of map[provider.name] = feature(interface)
	if !features.IsActive(features.CloudStackProvider()) && deps.Provider.Name() == constants.CloudStackProviderName {
		return fmt.Errorf("provider cloudstack is not supported in this release")
	}

	if !features.IsActive(features.SnowProvider()) && deps.Provider.Name() == constants.SnowProviderName {
		return fmt.Errorf("provider snow is not supported in this release")
	}

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

	createValidator := workflows.NewValidate(deps.Provider, deps.FluxAddonClient, createValidations)

	err = createValidator.Run(ctx, clusterSpec)

	cleanup(deps, &err)
	return err
}

// Creates validations but I don't think they're run until later
func (valOpt *validateOptions) directoriesToMount(clusterSpec *cluster.Spec, cliConfig *config.CliConfig) ([]string, error) {
	dirs := valOpt.mountDirs()
	fluxConfig := clusterSpec.FluxConfig
	if fluxConfig != nil && fluxConfig.Spec.Git != nil {
		dirs = append(dirs, filepath.Dir(cliConfig.GitPrivateKeyFile))
		dirs = append(dirs, filepath.Dir(cliConfig.GitKnownHostsFile))
		dirs = append(dirs, filepath.Dir(valOpt.installPackages))
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

// Define cluster config validation
func clusterConfigValidation(clusterfile string) []validations.Validation {

	// This will need to be better organized
	clusterConfig, _ := v1alpha1.GetClusterConfig(clusterfile)

	return []validations.Validation{
		func() *validations.ValidationResult {
			return &validations.ValidationResult{
				Name: "Cluster config is valid",
				Err:  v1alpha1.ValidateClusterConfigContent(clusterConfig),
			}
		},
	}
}

// Define cluster spec validation
// Could add the validations individually into a single func
func clusterSpecValidation(c *cluster.Spec) []validations.Validation {

	manager, _ := cluster.NewDefaultConfigManager()

	return []validations.Validation{
		func() *validations.ValidationResult {
			return &validations.ValidationResult{
				Name: "Cluster spec is valid",
				Err:  manager.Validate(c.Config),
			}
		},
	}
}

// Kubeconfig validation
func kubeconfigValidation(clusterName string) []validations.Validation {
	return []validations.Validation{
		func() *validations.ValidationResult {
			return &validations.ValidationResult{
				Name: "No conflicting cluster config file",
				Err:  kubeconfigCheck(clusterName),
			}
		},
	}
}

func kubeconfigCheck(clusterName string) error {
	kubeconfigPath := kubeconfig.FromClusterName(clusterName)
	if validations.FileExistsAndIsNotEmpty(kubeconfigPath) {
		return fmt.Errorf(
			"old cluster config file exists under %s, please use a different clusterName to proceed",
			clusterName,
		)
	}
	return nil
}
