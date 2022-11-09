/*
Copyright ApeCloud Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/apecloud/kubeblocks/internal/loadbalancer/agent (interfaces: Node)

// Package mock_agent is a generated GoMock package.
package mock_agent

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"

	agent "github.com/apecloud/kubeblocks/internal/loadbalancer/agent"
	protocol "github.com/apecloud/kubeblocks/internal/loadbalancer/protocol"
)

// MockNode is a mock of Node interface.
type MockNode struct {
	ctrl     *gomock.Controller
	recorder *MockNodeMockRecorder
}

// MockNodeMockRecorder is the mock recorder for MockNode.
type MockNodeMockRecorder struct {
	mock *MockNode
}

// NewMockNode creates a new mock instance.
func NewMockNode(ctrl *gomock.Controller) *MockNode {
	mock := &MockNode{ctrl: ctrl}
	mock.recorder = &MockNodeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNode) EXPECT() *MockNodeMockRecorder {
	return m.recorder
}

// ChooseENI mocks base method.
func (m *MockNode) ChooseENI() (*protocol.ENIMetadata, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ChooseENI")
	ret0, _ := ret[0].(*protocol.ENIMetadata)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ChooseENI indicates an expected call of ChooseENI.
func (mr *MockNodeMockRecorder) ChooseENI() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChooseENI", reflect.TypeOf((*MockNode)(nil).ChooseENI))
}

// CleanNetworkForService mocks base method.
func (m *MockNode) CleanNetworkForService(arg0 string, arg1 *protocol.ENIMetadata) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CleanNetworkForService", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// CleanNetworkForService indicates an expected call of CleanNetworkForService.
func (mr *MockNodeMockRecorder) CleanNetworkForService(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CleanNetworkForService", reflect.TypeOf((*MockNode)(nil).CleanNetworkForService), arg0, arg1)
}

// GetIP mocks base method.
func (m *MockNode) GetIP() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetIP")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetIP indicates an expected call of GetIP.
func (mr *MockNodeMockRecorder) GetIP() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetIP", reflect.TypeOf((*MockNode)(nil).GetIP))
}

// GetManagedENIs mocks base method.
func (m *MockNode) GetManagedENIs() ([]*protocol.ENIMetadata, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetManagedENIs")
	ret0, _ := ret[0].([]*protocol.ENIMetadata)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetManagedENIs indicates an expected call of GetManagedENIs.
func (mr *MockNodeMockRecorder) GetManagedENIs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetManagedENIs", reflect.TypeOf((*MockNode)(nil).GetManagedENIs))
}

// GetNodeInfo mocks base method.
func (m *MockNode) GetNodeInfo() *protocol.InstanceInfo {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNodeInfo")
	ret0, _ := ret[0].(*protocol.InstanceInfo)
	return ret0
}

// GetNodeInfo indicates an expected call of GetNodeInfo.
func (mr *MockNodeMockRecorder) GetNodeInfo() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNodeInfo", reflect.TypeOf((*MockNode)(nil).GetNodeInfo))
}

// GetResource mocks base method.
func (m *MockNode) GetResource() *agent.NodeResource {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetResource")
	ret0, _ := ret[0].(*agent.NodeResource)
	return ret0
}

// GetResource indicates an expected call of GetResource.
func (mr *MockNodeMockRecorder) GetResource() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetResource", reflect.TypeOf((*MockNode)(nil).GetResource))
}

// SetupNetworkForService mocks base method.
func (m *MockNode) SetupNetworkForService(arg0 string, arg1 *protocol.ENIMetadata) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetupNetworkForService", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetupNetworkForService indicates an expected call of SetupNetworkForService.
func (mr *MockNodeMockRecorder) SetupNetworkForService(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetupNetworkForService", reflect.TypeOf((*MockNode)(nil).SetupNetworkForService), arg0, arg1)
}

// Start mocks base method.
func (m *MockNode) Start() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start")
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockNodeMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockNode)(nil).Start))
}

// Stop mocks base method.
func (m *MockNode) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop.
func (mr *MockNodeMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockNode)(nil).Stop))
}
