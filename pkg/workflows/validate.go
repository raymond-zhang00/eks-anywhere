package workflows

import (
	"context"
	"fmt"

	"github.com/aws/eks-anywhere/pkg/cluster"
	"github.com/aws/eks-anywhere/pkg/config"
	"github.com/aws/eks-anywhere/pkg/filewriter"
	"github.com/aws/eks-anywhere/pkg/kubeconfig"
	"github.com/aws/eks-anywhere/pkg/logger"
	"github.com/aws/eks-anywhere/pkg/providers"
	"github.com/aws/eks-anywhere/pkg/task"
	"github.com/aws/eks-anywhere/pkg/types"
	"github.com/aws/eks-anywhere/pkg/validations"
	"github.com/aws/eks-anywhere/pkg/validations/createvalidations"
	"github.com/aws/eks-anywhere/pkg/workflows/interfaces"
)

// Run validation logic

type CreateValidator struct {
	Bootstrapper     interfaces.Bootstrapper
	Provider         providers.Provider
	ClusterManager   interfaces.ClusterManager
	AddonManager     interfaces.AddonManager
	Writer           filewriter.FileWriter
	EksdInstaller    interfaces.EksdInstaller
	PackageInstaller interfaces.PackageInstaller
	Cluster          types.Cluster
	Kubectl          validations.KubectlClient
	CliConfig        config.CliConfig
}

func (v *CreateValidator) CreateValidations(ctx context.Context, clusterSpec *cluster.Spec, forceCleanup bool) error {

	// Setup validator
	validationOpts := &validations.Opts{
		Kubectl: v.Kubectl,
		Spec:    clusterSpec,
		WorkloadCluster: &types.Cluster{
			Name:           clusterSpec.Cluster.Name,
			KubeconfigFile: kubeconfig.FromClusterName(clusterSpec.Cluster.Name),
		},
		ManagementCluster: &v.Cluster,
		Provider:          v.Provider,
		CliConfig:         &v.CliConfig,
	}

	createValidations := createvalidations.New(validationOpts)

	// Maybe move this out of validate back to cmd?
	if forceCleanup {
		if err := v.Bootstrapper.DeleteBootstrapCluster(ctx, &types.Cluster{
			Name: clusterSpec.Cluster.Name,
		}, false); err != nil {
			return err
		}
	}

	commandContext := &task.CommandContext{
		Bootstrapper:     v.Bootstrapper,
		Provider:         v.Provider,
		ClusterManager:   v.ClusterManager,
		AddonManager:     v.AddonManager,
		ClusterSpec:      clusterSpec,
		Writer:           v.Writer,
		Validations:      createValidations,
		EksdInstaller:    v.EksdInstaller,
		PackageInstaller: v.PackageInstaller,
	}

	if clusterSpec.ManagementCluster != nil {
		commandContext.BootstrapCluster = clusterSpec.ManagementCluster
	}

	logger.Info("Performing validate task using the validate workflow")
	runner := validations.NewRunner()
	runner.Register(v.providerValidation(ctx, commandContext)...)
	runner.Register(commandContext.AddonManager.Validations(ctx, commandContext.ClusterSpec)...)
	runner.Register(v.validations(ctx, commandContext)...)

	err := runner.Run()
	if err != nil {
		commandContext.SetError(err)
		return nil
	}
	return nil

}

func (v *CreateValidator) providerValidation(ctx context.Context, commandContext *task.CommandContext) []validations.Validation {
	return []validations.Validation{
		func() *validations.ValidationResult {
			return &validations.ValidationResult{
				Name: fmt.Sprintf("%s Provider setup is valid", commandContext.Provider.Name()),
				Err:  commandContext.Provider.SetupAndValidateCreateCluster(ctx, commandContext.ClusterSpec),
			}
		},
	}
}

func (v *CreateValidator) validations(ctx context.Context, commandContext *task.CommandContext) []validations.Validation {
	return []validations.Validation{
		func() *validations.ValidationResult {
			return &validations.ValidationResult{
				Name: "create preflight validations pass",
				Err:  commandContext.Validations.PreflightValidations(ctx),
			}
		},
	}
}
