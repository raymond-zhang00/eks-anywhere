package management_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/aws/eks-anywhere/internal/test"
	"github.com/aws/eks-anywhere/pkg/api/v1alpha1"
	"github.com/aws/eks-anywhere/pkg/cluster"
	"github.com/aws/eks-anywhere/pkg/features"
	writermocks "github.com/aws/eks-anywhere/pkg/filewriter/mocks"
	"github.com/aws/eks-anywhere/pkg/providers"
	providermocks "github.com/aws/eks-anywhere/pkg/providers/mocks"
	"github.com/aws/eks-anywhere/pkg/types"
	"github.com/aws/eks-anywhere/pkg/workflows/interfaces/mocks"
	"github.com/aws/eks-anywhere/pkg/workflows/management"
)

type createTestSetup struct {
	t                *testing.T
	packageInstaller *mocks.MockPackageInstaller
	clusterManager   *mocks.MockClusterManager
	bootstrapper     *mocks.MockBootstrapper
	gitOpsManager    *mocks.MockGitOpsManager
	provider         *providermocks.MockProvider
	writer           *writermocks.MockFileWriter
	validator        *mocks.MockValidator
	eksdInstaller    *mocks.MockEksdInstaller
	clusterCreator   *mocks.MockClusterCreator
	datacenterConfig providers.DatacenterConfig
	machineConfigs   []providers.MachineConfig
	ctx              context.Context
	clusterSpec      *cluster.Spec
	bootstrapCluster *types.Cluster
	workloadCluster  *types.Cluster
	workflow         *management.Create
}

func newCreateTest(t *testing.T) *createTestSetup {
	featureEnvVars := []string{}
	featureEnvVars = append(featureEnvVars, features.UseControllerForCli)
	mockCtrl := gomock.NewController(t)
	bootstrapper := mocks.NewMockBootstrapper(mockCtrl)
	clusterManager := mocks.NewMockClusterManager(mockCtrl)
	gitOpsManager := mocks.NewMockGitOpsManager(mockCtrl)
	provider := providermocks.NewMockProvider(mockCtrl)
	writer := writermocks.NewMockFileWriter(mockCtrl)
	eksdInstaller := mocks.NewMockEksdInstaller(mockCtrl)
	packageInstaller := mocks.NewMockPackageInstaller(mockCtrl)

	datacenterConfig := &v1alpha1.VSphereDatacenterConfig{}
	machineConfigs := []providers.MachineConfig{&v1alpha1.VSphereMachineConfig{}}
	clusterCreator := mocks.NewMockClusterCreator(mockCtrl)
	validator := mocks.NewMockValidator(mockCtrl)

	workflow := management.NewCreate(
		bootstrapper,
		provider,
		clusterManager,
		gitOpsManager,
		writer,
		eksdInstaller,
		packageInstaller,
		clusterCreator,
	)

	for _, e := range featureEnvVars {
		t.Setenv(e, "true")
	}

	return &createTestSetup{
		t:                t,
		bootstrapper:     bootstrapper,
		clusterManager:   clusterManager,
		gitOpsManager:    gitOpsManager,
		provider:         provider,
		writer:           writer,
		validator:        validator,
		eksdInstaller:    eksdInstaller,
		packageInstaller: packageInstaller,
		clusterCreator:   clusterCreator,
		datacenterConfig: datacenterConfig,
		machineConfigs:   machineConfigs,
		workflow:         workflow,
		ctx:              context.Background(),
		bootstrapCluster: &types.Cluster{Name: "bootstrap"},
		workloadCluster:  &types.Cluster{Name: "workload"},
		clusterSpec:      test.NewClusterSpec(func(s *cluster.Spec) { s.Cluster.Name = "cluster-name" }),
	}
}

func (c *createTestSetup) expectSetup() {
	c.provider.EXPECT().SetupAndValidateCreateCluster(c.ctx, c.clusterSpec)
	c.provider.EXPECT().Name()
	c.gitOpsManager.EXPECT().Validations(c.ctx, c.clusterSpec)
}

func (c *createTestSetup) run() error {
	return c.workflow.Run(c.ctx, c.clusterSpec, c.validator)
}

func (c *createTestSetup) expectPreflightValidationsToPass() {
	c.validator.EXPECT().PreflightValidations(c.ctx)
}

func TestCreateRunSuccess(t *testing.T) {
	test := newCreateTest(t)
	test.expectSetup()
	test.expectPreflightValidationsToPass()

	err := test.run()
	if err != nil {
		t.Fatalf("Create.Run() err = %v, want err = nil", err)
	}
}
