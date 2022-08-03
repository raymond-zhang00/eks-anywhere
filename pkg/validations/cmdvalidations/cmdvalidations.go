package cmdvalidations

import (
	"context"
	"fmt"
	"runtime"

	"github.com/aws/eks-anywhere/pkg/executables"
	"github.com/aws/eks-anywhere/pkg/kubeconfig"
	"github.com/aws/eks-anywhere/pkg/validations"
)

func PackageDockerValidations(ctx context.Context) []validations.Validation {
	return []validations.Validation{
		func() *validations.ValidationResult {
			return &validations.ValidationResult{
				Name: "validate docker executable",
				Err:  validateDockerExecutable(ctx),
			}
		},
	}
}

func PackageKubeConfigPath(clusterName string) []validations.Validation {
	return []validations.Validation{
		func() *validations.ValidationResult {
			return &validations.ValidationResult{
				Name: "validate kubeconfig path",
				Err:  validateKubeConfigPath(clusterName),
			}
		},
	}
}

func validateDockerExecutable(ctx context.Context) error {
	docker := executables.BuildDockerExecutable()
	err := validations.CheckMinimumDockerVersion(ctx, docker)
	if err != nil {
		return fmt.Errorf("failed to validate docker: %v", err)
	}
	if runtime.GOOS == "darwin" {
		err = validations.CheckDockerDesktopVersion(ctx, docker)
		if err != nil {
			return fmt.Errorf("failed to validate docker desktop: %v", err)
		}
	}
	validations.CheckDockerAllocatedMemory(ctx, docker)

	return nil
}

func validateKubeConfigPath(clusterName string) error {
	kubeconfigPath := kubeconfig.FromClusterName(clusterName)
	if validations.FileExistsAndIsNotEmpty(kubeconfigPath) {
		return fmt.Errorf(
			"old cluster config file exists under %s, please use a different clusterName to proceed",
			clusterName,
		)
	}

	return nil
}
