// /*
// Copyright 2024 NVIDIA.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

// Code generated by MockGen. DO NOT EDIT.
// Source: types.go
//
// Generated by this command:
//
//	mockgen -copyright_file ../../../../hack/boilerplate.go.txt -destination mock/ovsclient.go -source types.go
//

// Package mock_ovsclient is a generated GoMock package.
package mock_ovsclient

import (
	net "net"
	reflect "reflect"

	ovsclient "github.com/nvidia/doca-platform/internal/cniprovisioner/utils/ovsclient"
	gomock "go.uber.org/mock/gomock"
)

// MockOVSClient is a mock of OVSClient interface.
type MockOVSClient struct {
	ctrl     *gomock.Controller
	recorder *MockOVSClientMockRecorder
}

// MockOVSClientMockRecorder is the mock recorder for MockOVSClient.
type MockOVSClientMockRecorder struct {
	mock *MockOVSClient
}

// NewMockOVSClient creates a new mock instance.
func NewMockOVSClient(ctrl *gomock.Controller) *MockOVSClient {
	mock := &MockOVSClient{ctrl: ctrl}
	mock.recorder = &MockOVSClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOVSClient) EXPECT() *MockOVSClientMockRecorder {
	return m.recorder
}

// AddBridgeIfNotExists mocks base method.
func (m *MockOVSClient) AddBridgeIfNotExists(name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddBridgeIfNotExists", name)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddBridgeIfNotExists indicates an expected call of AddBridgeIfNotExists.
func (mr *MockOVSClientMockRecorder) AddBridgeIfNotExists(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddBridgeIfNotExists", reflect.TypeOf((*MockOVSClient)(nil).AddBridgeIfNotExists), name)
}

// AddPortIfNotExists mocks base method.
func (m *MockOVSClient) AddPortIfNotExists(bridge, port string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddPortIfNotExists", bridge, port)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddPortIfNotExists indicates an expected call of AddPortIfNotExists.
func (mr *MockOVSClientMockRecorder) AddPortIfNotExists(bridge, port any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddPortIfNotExists", reflect.TypeOf((*MockOVSClient)(nil).AddPortIfNotExists), bridge, port)
}

// BridgeExists mocks base method.
func (m *MockOVSClient) BridgeExists(name string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BridgeExists", name)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BridgeExists indicates an expected call of BridgeExists.
func (mr *MockOVSClientMockRecorder) BridgeExists(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BridgeExists", reflect.TypeOf((*MockOVSClient)(nil).BridgeExists), name)
}

// DeleteBridgeIfExists mocks base method.
func (m *MockOVSClient) DeleteBridgeIfExists(name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteBridgeIfExists", name)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteBridgeIfExists indicates an expected call of DeleteBridgeIfExists.
func (mr *MockOVSClientMockRecorder) DeleteBridgeIfExists(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteBridgeIfExists", reflect.TypeOf((*MockOVSClient)(nil).DeleteBridgeIfExists), name)
}

// SetBridgeController mocks base method.
func (m *MockOVSClient) SetBridgeController(bridge, controller string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetBridgeController", bridge, controller)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetBridgeController indicates an expected call of SetBridgeController.
func (mr *MockOVSClientMockRecorder) SetBridgeController(bridge, controller any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetBridgeController", reflect.TypeOf((*MockOVSClient)(nil).SetBridgeController), bridge, controller)
}

// SetBridgeDataPathType mocks base method.
func (m *MockOVSClient) SetBridgeDataPathType(bridge string, bridgeType ovsclient.BridgeDataPathType) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetBridgeDataPathType", bridge, bridgeType)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetBridgeDataPathType indicates an expected call of SetBridgeDataPathType.
func (mr *MockOVSClientMockRecorder) SetBridgeDataPathType(bridge, bridgeType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetBridgeDataPathType", reflect.TypeOf((*MockOVSClient)(nil).SetBridgeDataPathType), bridge, bridgeType)
}

// SetBridgeHostToServicePort mocks base method.
func (m *MockOVSClient) SetBridgeHostToServicePort(bridge, port string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetBridgeHostToServicePort", bridge, port)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetBridgeHostToServicePort indicates an expected call of SetBridgeHostToServicePort.
func (mr *MockOVSClientMockRecorder) SetBridgeHostToServicePort(bridge, port any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetBridgeHostToServicePort", reflect.TypeOf((*MockOVSClient)(nil).SetBridgeHostToServicePort), bridge, port)
}

// SetBridgeMAC mocks base method.
func (m *MockOVSClient) SetBridgeMAC(bridge string, mac net.HardwareAddr) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetBridgeMAC", bridge, mac)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetBridgeMAC indicates an expected call of SetBridgeMAC.
func (mr *MockOVSClientMockRecorder) SetBridgeMAC(bridge, mac any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetBridgeMAC", reflect.TypeOf((*MockOVSClient)(nil).SetBridgeMAC), bridge, mac)
}

// SetBridgeUplinkPort mocks base method.
func (m *MockOVSClient) SetBridgeUplinkPort(bridge, port string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetBridgeUplinkPort", bridge, port)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetBridgeUplinkPort indicates an expected call of SetBridgeUplinkPort.
func (mr *MockOVSClientMockRecorder) SetBridgeUplinkPort(bridge, port any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetBridgeUplinkPort", reflect.TypeOf((*MockOVSClient)(nil).SetBridgeUplinkPort), bridge, port)
}

// SetDOCAInit mocks base method.
func (m *MockOVSClient) SetDOCAInit(enable bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetDOCAInit", enable)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetDOCAInit indicates an expected call of SetDOCAInit.
func (mr *MockOVSClientMockRecorder) SetDOCAInit(enable any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetDOCAInit", reflect.TypeOf((*MockOVSClient)(nil).SetDOCAInit), enable)
}

// SetKubernetesHostNodeName mocks base method.
func (m *MockOVSClient) SetKubernetesHostNodeName(name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetKubernetesHostNodeName", name)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetKubernetesHostNodeName indicates an expected call of SetKubernetesHostNodeName.
func (mr *MockOVSClientMockRecorder) SetKubernetesHostNodeName(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetKubernetesHostNodeName", reflect.TypeOf((*MockOVSClient)(nil).SetKubernetesHostNodeName), name)
}

// SetOVNEncapIP mocks base method.
func (m *MockOVSClient) SetOVNEncapIP(ip net.IP) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetOVNEncapIP", ip)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetOVNEncapIP indicates an expected call of SetOVNEncapIP.
func (mr *MockOVSClientMockRecorder) SetOVNEncapIP(ip any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetOVNEncapIP", reflect.TypeOf((*MockOVSClient)(nil).SetOVNEncapIP), ip)
}

// SetPatchPortPeer mocks base method.
func (m *MockOVSClient) SetPatchPortPeer(port, peer string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetPatchPortPeer", port, peer)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetPatchPortPeer indicates an expected call of SetPatchPortPeer.
func (mr *MockOVSClientMockRecorder) SetPatchPortPeer(port, peer any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetPatchPortPeer", reflect.TypeOf((*MockOVSClient)(nil).SetPatchPortPeer), port, peer)
}

// SetPortType mocks base method.
func (m *MockOVSClient) SetPortType(port string, portType ovsclient.PortType) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetPortType", port, portType)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetPortType indicates an expected call of SetPortType.
func (mr *MockOVSClientMockRecorder) SetPortType(port, portType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetPortType", reflect.TypeOf((*MockOVSClient)(nil).SetPortType), port, portType)
}
