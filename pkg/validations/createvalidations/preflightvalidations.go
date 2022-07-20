package createvalidations

import (
	"context"
	"fmt"

	anywherev1 "github.com/aws/eks-anywhere/pkg/api/v1alpha1"
	"github.com/aws/eks-anywhere/pkg/features"
	"github.com/aws/eks-anywhere/pkg/types"
	"github.com/aws/eks-anywhere/pkg/validations"
)

func (u *CreateValidations) PreflightValidations(ctx context.Context) (err error) {
	k := u.Opts.Kubectl

	targetCluster := &types.Cluster{
		Name:           u.Opts.WorkloadCluster.Name,
		KubeconfigFile: u.Opts.ManagementCluster.KubeconfigFile,
	}

	createValidations := []validations.ValidationResult{
		{
			Name:        "validate certificate for registry mirror",
			Remediation: fmt.Sprintf("provide a valid certificate for you registry endpoint using %s env var", anywherev1.RegistryMirrorCAKey),
			Err:         validations.ValidateCertForRegistryMirror(u.Opts.Spec, u.Opts.TlsValidator),
		},
		{
			Name:        "validate kubernetes version 1.23 support",
			Remediation: fmt.Sprintf("ensure %v env variable is set", features.K8s123SupportEnvVar),
			Err:         validations.ValidateK8s123Support(u.Opts.Spec),
			Silent:      true,
		},
		{
			Name:        "Test Validation 1",
			Remediation: fmt.Sprintf("Test validation 1 failed"),
			Err:         testValidation(1),
		},
		{
			Name:        "Test Validation 2",
			Remediation: fmt.Sprintf("Test validation 1 failed"),
			Err:         testValidation(1),
		},
	}

	if u.Opts.Spec.Cluster.IsManaged() {
		createValidations = append(
			createValidations,
			validations.ValidationResult{
				Name:        "validate cluster name",
				Remediation: "",
				Err:         ValidateClusterNameIsUnique(ctx, k, targetCluster, u.Opts.Spec.Cluster.Name),
			},
			validations.ValidationResult{
				Name:        "validate gitops",
				Remediation: "",
				Err:         ValidateGitOps(ctx, k, u.Opts.ManagementCluster, u.Opts.Spec, u.Opts.CliConfig),
			},
			validations.ValidationResult{
				Name:        "validate identity providers' name",
				Remediation: "",
				Err:         ValidateIdentityProviderNameIsUnique(ctx, k, targetCluster, u.Opts.Spec),
			},
			validations.ValidationResult{
				Name:        "validate management cluster has eksa crds",
				Remediation: "",
				Err:         ValidateManagementCluster(ctx, k, targetCluster),
			},
		)
	}

	return validations.RunPreflightValidations(createValidations)
}

func testValidation(x int) error {
	if x == 0 {
		return fmt.Errorf("validating registry mirror endpoint: %v", err)
	}

	return nil
}
