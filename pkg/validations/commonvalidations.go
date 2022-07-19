package validations

import (
	"context"
	"fmt"
	"runtime"

	"github.com/aws/eks-anywhere/pkg/api/v1alpha1"
	"github.com/aws/eks-anywhere/pkg/executables"
)

// Moved from cmd/validations.go

func CommonValidation(ctx context.Context, clusterConfigFile string) (*v1alpha1.Cluster, error) {

	// Create cluster runs config validations before docker, everything else runs docker first

	docker := executables.BuildDockerExecutable()
	err := CheckMinimumDockerVersion(ctx, docker)
	if err != nil {
		return nil, fmt.Errorf("failed to validate docker: %v", err)
	}
	if runtime.GOOS == "darwin" {
		err = CheckDockerDesktopVersion(ctx, docker)
		if err != nil {
			return nil, fmt.Errorf("failed to validate docker desktop: %v", err)
		}
	}
	CheckDockerAllocatedMemory(ctx, docker)
	clusterConfigFileExist := FileExists(clusterConfigFile)
	if !clusterConfigFileExist {
		return nil, fmt.Errorf("the cluster config file %s does not exist", clusterConfigFile)
	}
	clusterConfig, err := v1alpha1.GetAndValidateClusterConfig(clusterConfigFile)
	if err != nil {
		return nil, fmt.Errorf("the cluster config file provided is invalid: %v", err)
	}
	return clusterConfig, nil
}
