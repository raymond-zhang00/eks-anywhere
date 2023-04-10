// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/aws/eks-anywhere/pkg/providers/cloudstack (interfaces: ProviderValidator)

// Package cloudstack is a generated GoMock package.
package cloudstack

import (
	context "context"
	reflect "reflect"

	v1alpha1 "github.com/aws/eks-anywhere/pkg/api/v1alpha1"
	cluster "github.com/aws/eks-anywhere/pkg/cluster"
	decoder "github.com/aws/eks-anywhere/pkg/providers/cloudstack/decoder"
	types "github.com/aws/eks-anywhere/pkg/types"
	gomock "github.com/golang/mock/gomock"
)

// MockProviderValidator is a mock of ProviderValidator interface.
type MockProviderValidator struct {
	ctrl     *gomock.Controller
	recorder *MockProviderValidatorMockRecorder
}

// MockProviderValidatorMockRecorder is the mock recorder for MockProviderValidator.
type MockProviderValidatorMockRecorder struct {
	mock *MockProviderValidator
}

// NewMockProviderValidator creates a new mock instance.
func NewMockProviderValidator(ctrl *gomock.Controller) *MockProviderValidator {
	mock := &MockProviderValidator{ctrl: ctrl}
	mock.recorder = &MockProviderValidatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockProviderValidator) EXPECT() *MockProviderValidatorMockRecorder {
	return m.recorder
}

// ValidateCloudStackDatacenterConfig mocks base method.
func (m *MockProviderValidator) ValidateCloudStackDatacenterConfig(arg0 context.Context, arg1 *v1alpha1.CloudStackDatacenterConfig) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidateCloudStackDatacenterConfig", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// ValidateCloudStackDatacenterConfig indicates an expected call of ValidateCloudStackDatacenterConfig.
func (mr *MockProviderValidatorMockRecorder) ValidateCloudStackDatacenterConfig(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateCloudStackDatacenterConfig", reflect.TypeOf((*MockProviderValidator)(nil).ValidateCloudStackDatacenterConfig), arg0, arg1)
}

// ValidateClusterMachineConfigs mocks base method.
func (m *MockProviderValidator) ValidateClusterMachineConfigs(arg0 context.Context, arg1 *Spec) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidateClusterMachineConfigs", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// ValidateClusterMachineConfigs indicates an expected call of ValidateClusterMachineConfigs.
func (mr *MockProviderValidatorMockRecorder) ValidateClusterMachineConfigs(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateClusterMachineConfigs", reflect.TypeOf((*MockProviderValidator)(nil).ValidateClusterMachineConfigs), arg0, arg1)
}

// ValidateControlPlaneDiskOfferingUnchanged mocks base method.
func (m *MockProviderValidator) ValidateControlPlaneDiskOfferingUnchanged(arg0 context.Context, arg1 *types.Cluster, arg2 *cluster.Spec, arg3 ProviderKubectlClient) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidateControlPlaneDiskOfferingUnchanged", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// ValidateControlPlaneDiskOfferingUnchanged indicates an expected call of ValidateControlPlaneDiskOfferingUnchanged.
func (mr *MockProviderValidatorMockRecorder) ValidateControlPlaneDiskOfferingUnchanged(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateControlPlaneDiskOfferingUnchanged", reflect.TypeOf((*MockProviderValidator)(nil).ValidateControlPlaneDiskOfferingUnchanged), arg0, arg1, arg2, arg3)
}

// ValidateControlPlaneEndpointUniqueness mocks base method.
func (m *MockProviderValidator) ValidateControlPlaneEndpointUniqueness(arg0 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidateControlPlaneEndpointUniqueness", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// ValidateControlPlaneEndpointUniqueness indicates an expected call of ValidateControlPlaneEndpointUniqueness.
func (mr *MockProviderValidatorMockRecorder) ValidateControlPlaneEndpointUniqueness(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateControlPlaneEndpointUniqueness", reflect.TypeOf((*MockProviderValidator)(nil).ValidateControlPlaneEndpointUniqueness), arg0)
}

// ValidateSecretsUnchanged mocks base method.
func (m *MockProviderValidator) ValidateSecretsUnchanged(arg0 context.Context, arg1 *types.Cluster, arg2 *decoder.CloudStackExecConfig, arg3 ProviderKubectlClient) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidateSecretsUnchanged", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// ValidateSecretsUnchanged indicates an expected call of ValidateSecretsUnchanged.
func (mr *MockProviderValidatorMockRecorder) ValidateSecretsUnchanged(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateSecretsUnchanged", reflect.TypeOf((*MockProviderValidator)(nil).ValidateSecretsUnchanged), arg0, arg1, arg2, arg3)
}
