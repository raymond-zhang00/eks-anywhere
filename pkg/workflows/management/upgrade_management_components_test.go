package management

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/aws/eks-anywhere/internal/test"
	writermocks "github.com/aws/eks-anywhere/pkg/filewriter/mocks"
	"github.com/aws/eks-anywhere/pkg/kubeconfig"
	providermocks "github.com/aws/eks-anywhere/pkg/providers/mocks"
	"github.com/aws/eks-anywhere/pkg/types"
	"github.com/aws/eks-anywhere/pkg/validations"
	"github.com/aws/eks-anywhere/pkg/workflows/interfaces/mocks"
)

var capiChangeDiff = types.NewChangeDiff(&types.ComponentChangeDiff{
	ComponentName: "vsphere",
	OldVersion:    "v0.0.1",
	NewVersion:    "v0.0.2",
})

var fluxChangeDiff = types.NewChangeDiff(&types.ComponentChangeDiff{
	ComponentName: "Flux",
	OldVersion:    "v0.0.1",
	NewVersion:    "v0.0.2",
})

var eksaChangeDiff = types.NewChangeDiff(&types.ComponentChangeDiff{
	ComponentName: "eks-a",
	OldVersion:    "v0.0.1",
	NewVersion:    "v0.0.2",
})

var eksdChangeDiff = types.NewChangeDiff(&types.ComponentChangeDiff{
	ComponentName: "eks-d",
	OldVersion:    "v0.0.1",
	NewVersion:    "v0.0.2",
})

type TestMocks struct {
	mockCtrl       *gomock.Controller
	clusterManager *mocks.MockClusterManager
	gitOpsManager  *mocks.MockGitOpsManager
	provider       *providermocks.MockProvider
	writer         *writermocks.MockFileWriter
	eksdInstaller  *mocks.MockEksdInstaller
	eksdUpgrader   *mocks.MockEksdUpgrader
	capiManager    *mocks.MockCAPIManager
	validator      *mocks.MockValidator
}

func NewTestMocks(t *testing.T) *TestMocks {
	mockCtrl := gomock.NewController(t)
	return &TestMocks{
		mockCtrl:       mockCtrl,
		clusterManager: mocks.NewMockClusterManager(mockCtrl),
		gitOpsManager:  mocks.NewMockGitOpsManager(mockCtrl),
		provider:       providermocks.NewMockProvider(mockCtrl),
		writer:         writermocks.NewMockFileWriter(mockCtrl),
		eksdInstaller:  mocks.NewMockEksdInstaller(mockCtrl),
		eksdUpgrader:   mocks.NewMockEksdUpgrader(mockCtrl),
		capiManager:    mocks.NewMockCAPIManager(mockCtrl),
		validator:      mocks.NewMockValidator(mockCtrl),
	}
}

func TestRunnerHappyPath(t *testing.T) {
	mocks := NewTestMocks(t)
	runner := NewUpgradeManagementComponentsRunner(
		mocks.provider,
		mocks.capiManager,
		mocks.clusterManager,
		mocks.gitOpsManager,
		mocks.writer,
		mocks.eksdUpgrader,
		mocks.eksdInstaller,
	)

	clusterSpec := test.NewClusterSpec()
	managementCluster := &types.Cluster{
		Name:           clusterSpec.Cluster.Name,
		KubeconfigFile: kubeconfig.FromClusterName(clusterSpec.Cluster.Name),
	}

	ctx := context.Background()
	curSpec := test.NewClusterSpec()
	newSpec := test.NewClusterSpec()

	mocks.clusterManager.EXPECT().GetCurrentClusterSpec(ctx, gomock.Any(), managementCluster.Name).Return(curSpec, nil)
	gomock.InOrder(
		mocks.validator.EXPECT().PreflightValidations(ctx).Return(nil),
		mocks.provider.EXPECT().Name(),
		mocks.provider.EXPECT().SetupAndValidateUpgradeCluster(ctx, gomock.Any(), newSpec, curSpec),
		mocks.provider.EXPECT().PreCoreComponentsUpgrade(gomock.Any(), gomock.Any(), gomock.Any()),
		mocks.capiManager.EXPECT().Upgrade(ctx, managementCluster, mocks.provider, curSpec, newSpec).Return(capiChangeDiff, nil),
		mocks.gitOpsManager.EXPECT().Install(ctx, managementCluster, curSpec, newSpec).Return(nil),
		mocks.gitOpsManager.EXPECT().Upgrade(ctx, managementCluster, curSpec, newSpec).Return(fluxChangeDiff, nil),
		mocks.clusterManager.EXPECT().Upgrade(ctx, managementCluster, curSpec, newSpec).Return(eksaChangeDiff, nil),
		mocks.eksdUpgrader.EXPECT().Upgrade(ctx, managementCluster, curSpec, newSpec).Return(eksdChangeDiff, nil),
		mocks.clusterManager.EXPECT().ApplyBundles(
			ctx, newSpec, managementCluster,
		).Return(nil),
		mocks.clusterManager.EXPECT().ApplyReleases(
			ctx, newSpec, managementCluster,
		).Return(nil),
		mocks.eksdInstaller.EXPECT().InstallEksdManifest(
			ctx, newSpec, managementCluster,
		).Return(nil),
	)

	err := runner.Run(ctx, newSpec, managementCluster, mocks.validator)
	if err != nil {
		t.Fatalf("UpgradeManagementComponents.Run() err = %v, want err = nil", err)
	}
}

func TestRunnerStopsWhenValidationFailed(t *testing.T) {
	mocks := NewTestMocks(t)
	runner := NewUpgradeManagementComponentsRunner(
		mocks.provider,
		mocks.capiManager,
		mocks.clusterManager,
		mocks.gitOpsManager,
		mocks.writer,
		mocks.eksdUpgrader,
		mocks.eksdInstaller,
	)

	clusterSpec := test.NewClusterSpec()
	managementCluster := &types.Cluster{
		Name:           clusterSpec.Cluster.Name,
		KubeconfigFile: kubeconfig.FromClusterName(clusterSpec.Cluster.Name),
	}

	ctx := context.Background()
	curSpec := test.NewClusterSpec()
	newSpec := test.NewClusterSpec()

	mocks.provider.EXPECT().Name()
	mocks.provider.EXPECT().SetupAndValidateUpgradeCluster(ctx, gomock.Any(), newSpec, curSpec)
	mocks.clusterManager.EXPECT().GetCurrentClusterSpec(ctx, gomock.Any(), managementCluster.Name).Return(curSpec, nil)
	mocks.validator.EXPECT().PreflightValidations(ctx).Return(
		[]validations.Validation{
			func() *validations.ValidationResult {
				return &validations.ValidationResult{
					Err: errors.New("validation failed"),
				}
			},
		})

	mocks.writer.EXPECT().Write(fmt.Sprintf("%s-checkpoint.yaml", newSpec.Cluster.Name), gomock.Any())
	err := runner.Run(ctx, newSpec, managementCluster, mocks.validator)
	if err == nil {
		t.Fatalf("UpgradeManagementComponents.Run() err == nil, want err != nil")
	}
}
