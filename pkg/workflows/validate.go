package workflows

import (
	"context"
	"fmt"
	"runtime"

	"github.com/aws/eks-anywhere/pkg/api/v1alpha1"
	"github.com/aws/eks-anywhere/pkg/cluster"
	"github.com/aws/eks-anywhere/pkg/executables"
	"github.com/aws/eks-anywhere/pkg/logger"
	"github.com/aws/eks-anywhere/pkg/providers"
	"github.com/aws/eks-anywhere/pkg/validations"
	"github.com/aws/eks-anywhere/pkg/workflows/interfaces"
)

type Validate struct {
	provider     providers.Provider
	addonManager interfaces.AddonManager
	validations  interfaces.Validator
}

func NewValidate(provider providers.Provider, addonManager interfaces.AddonManager,
	validations interfaces.Validator,
) *Validate {
	return &Validate{
		provider:     provider,
		addonManager: addonManager,
		validations:  validations,
	}
}

func (v *Validate) Run(ctx context.Context, clusterSpec *cluster.Spec) error {

	logger.Info("Performing validations")
	runner := validations.NewRunner()
	runner.Register(v.providerValidation(ctx, clusterSpec)...)
	runner.Register(v.addonManager.Validations(ctx, clusterSpec)...)
	runner.Register(v.createValidations(ctx)...)

	err := runner.Run()
	return err
}

func (v *Validate) providerValidation(ctx context.Context, clusterSpec *cluster.Spec) []validations.Validation {
	return []validations.Validation{
		func() *validations.ValidationResult {
			return &validations.ValidationResult{
				Name: fmt.Sprintf("%s Provider setup is valid", v.provider.Name()),
				Err:  v.provider.SetupAndValidateCreateCluster(ctx, clusterSpec),
			}
		},
	}
}

func (v *Validate) createValidations(ctx context.Context) []validations.Validation {
	return []validations.Validation{
		func() *validations.ValidationResult {
			return &validations.ValidationResult{
				Name: "create preflight validations pass",
				Err:  v.validations.PreflightValidations(ctx),
			}
		},
	}
}

func CommonValidation(ctx context.Context, clusterConfigFile string) (*v1alpha1.Cluster, error) {
	docker := executables.BuildDockerExecutable()
	err := validations.CheckMinimumDockerVersion(ctx, docker)
	if err != nil {
		return nil, fmt.Errorf("failed to validate docker: %v", err)
	}
	if runtime.GOOS == "darwin" {
		err = validations.CheckDockerDesktopVersion(ctx, docker)
		if err != nil {
			return nil, fmt.Errorf("failed to validate docker desktop: %v", err)
		}
	}
	validations.CheckDockerAllocatedMemory(ctx, docker)
	clusterConfigFileExist := validations.FileExists(clusterConfigFile)
	if !clusterConfigFileExist {
		return nil, fmt.Errorf("the cluster config file %s does not exist", clusterConfigFile)
	}
	clusterConfig, err := v1alpha1.GetAndValidateClusterConfig(clusterConfigFile)
	if err != nil {
		return nil, fmt.Errorf("the cluster config file provided is invalid: %v", err)
	}
	return clusterConfig, nil
}
