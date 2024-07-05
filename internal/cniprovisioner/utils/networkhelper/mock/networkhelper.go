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
//	mockgen -copyright_file ../../../../hack/boilerplate.go.txt -destination mock/networkhelper.go -source types.go
//

// Package mock_networkhelper is a generated GoMock package.
package mock_networkhelper

import (
	net "net"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockNetworkHelper is a mock of NetworkHelper interface.
type MockNetworkHelper struct {
	ctrl     *gomock.Controller
	recorder *MockNetworkHelperMockRecorder
}

// MockNetworkHelperMockRecorder is the mock recorder for MockNetworkHelper.
type MockNetworkHelperMockRecorder struct {
	mock *MockNetworkHelper
}

// NewMockNetworkHelper creates a new mock instance.
func NewMockNetworkHelper(ctrl *gomock.Controller) *MockNetworkHelper {
	mock := &MockNetworkHelper{ctrl: ctrl}
	mock.recorder = &MockNetworkHelperMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNetworkHelper) EXPECT() *MockNetworkHelperMockRecorder {
	return m.recorder
}

// AddDummyLink mocks base method.
func (m *MockNetworkHelper) AddDummyLink(link string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddDummyLink", link)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddDummyLink indicates an expected call of AddDummyLink.
func (mr *MockNetworkHelperMockRecorder) AddDummyLink(link any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddDummyLink", reflect.TypeOf((*MockNetworkHelper)(nil).AddDummyLink), link)
}

// AddRoute mocks base method.
func (m *MockNetworkHelper) AddRoute(network *net.IPNet, gateway net.IP, device string, metric *int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddRoute", network, gateway, device, metric)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddRoute indicates an expected call of AddRoute.
func (mr *MockNetworkHelperMockRecorder) AddRoute(network, gateway, device, metric any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddRoute", reflect.TypeOf((*MockNetworkHelper)(nil).AddRoute), network, gateway, device, metric)
}

// DeleteLinkIPAddress mocks base method.
func (m *MockNetworkHelper) DeleteLinkIPAddress(link string, ipNet *net.IPNet) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteLinkIPAddress", link, ipNet)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteLinkIPAddress indicates an expected call of DeleteLinkIPAddress.
func (mr *MockNetworkHelperMockRecorder) DeleteLinkIPAddress(link, ipNet any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteLinkIPAddress", reflect.TypeOf((*MockNetworkHelper)(nil).DeleteLinkIPAddress), link, ipNet)
}

// DeleteNeighbour mocks base method.
func (m *MockNetworkHelper) DeleteNeighbour(ip net.IP, device string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteNeighbour", ip, device)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteNeighbour indicates an expected call of DeleteNeighbour.
func (mr *MockNetworkHelperMockRecorder) DeleteNeighbour(ip, device any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteNeighbour", reflect.TypeOf((*MockNetworkHelper)(nil).DeleteNeighbour), ip, device)
}

// DeleteRoute mocks base method.
func (m *MockNetworkHelper) DeleteRoute(network *net.IPNet, gateway net.IP, device string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteRoute", network, gateway, device)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteRoute indicates an expected call of DeleteRoute.
func (mr *MockNetworkHelperMockRecorder) DeleteRoute(network, gateway, device any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteRoute", reflect.TypeOf((*MockNetworkHelper)(nil).DeleteRoute), network, gateway, device)
}

// DummyLinkExists mocks base method.
func (m *MockNetworkHelper) DummyLinkExists(link string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DummyLinkExists", link)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DummyLinkExists indicates an expected call of DummyLinkExists.
func (mr *MockNetworkHelperMockRecorder) DummyLinkExists(link any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DummyLinkExists", reflect.TypeOf((*MockNetworkHelper)(nil).DummyLinkExists), link)
}

// GetPFRepMACAddress mocks base method.
func (m *MockNetworkHelper) GetPFRepMACAddress(device string) (net.HardwareAddr, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPFRepMACAddress", device)
	ret0, _ := ret[0].(net.HardwareAddr)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPFRepMACAddress indicates an expected call of GetPFRepMACAddress.
func (mr *MockNetworkHelperMockRecorder) GetPFRepMACAddress(device any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPFRepMACAddress", reflect.TypeOf((*MockNetworkHelper)(nil).GetPFRepMACAddress), device)
}

// LinkExists mocks base method.
func (m *MockNetworkHelper) LinkExists(link string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LinkExists", link)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LinkExists indicates an expected call of LinkExists.
func (mr *MockNetworkHelperMockRecorder) LinkExists(link any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LinkExists", reflect.TypeOf((*MockNetworkHelper)(nil).LinkExists), link)
}

// LinkIPAddressExists mocks base method.
func (m *MockNetworkHelper) LinkIPAddressExists(link string, ipNet *net.IPNet) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LinkIPAddressExists", link, ipNet)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LinkIPAddressExists indicates an expected call of LinkIPAddressExists.
func (mr *MockNetworkHelperMockRecorder) LinkIPAddressExists(link, ipNet any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LinkIPAddressExists", reflect.TypeOf((*MockNetworkHelper)(nil).LinkIPAddressExists), link, ipNet)
}

// NeighbourExists mocks base method.
func (m *MockNetworkHelper) NeighbourExists(ip net.IP, device string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NeighbourExists", ip, device)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NeighbourExists indicates an expected call of NeighbourExists.
func (mr *MockNetworkHelperMockRecorder) NeighbourExists(ip, device any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NeighbourExists", reflect.TypeOf((*MockNetworkHelper)(nil).NeighbourExists), ip, device)
}

// RenameLink mocks base method.
func (m *MockNetworkHelper) RenameLink(link, newName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RenameLink", link, newName)
	ret0, _ := ret[0].(error)
	return ret0
}

// RenameLink indicates an expected call of RenameLink.
func (mr *MockNetworkHelperMockRecorder) RenameLink(link, newName any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RenameLink", reflect.TypeOf((*MockNetworkHelper)(nil).RenameLink), link, newName)
}

// RouteExists mocks base method.
func (m *MockNetworkHelper) RouteExists(network *net.IPNet, gateway net.IP, device string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RouteExists", network, gateway, device)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RouteExists indicates an expected call of RouteExists.
func (mr *MockNetworkHelperMockRecorder) RouteExists(network, gateway, device any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RouteExists", reflect.TypeOf((*MockNetworkHelper)(nil).RouteExists), network, gateway, device)
}

// SetLinkDown mocks base method.
func (m *MockNetworkHelper) SetLinkDown(link string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetLinkDown", link)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetLinkDown indicates an expected call of SetLinkDown.
func (mr *MockNetworkHelperMockRecorder) SetLinkDown(link any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetLinkDown", reflect.TypeOf((*MockNetworkHelper)(nil).SetLinkDown), link)
}

// SetLinkIPAddress mocks base method.
func (m *MockNetworkHelper) SetLinkIPAddress(link string, ipNet *net.IPNet) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetLinkIPAddress", link, ipNet)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetLinkIPAddress indicates an expected call of SetLinkIPAddress.
func (mr *MockNetworkHelperMockRecorder) SetLinkIPAddress(link, ipNet any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetLinkIPAddress", reflect.TypeOf((*MockNetworkHelper)(nil).SetLinkIPAddress), link, ipNet)
}

// SetLinkUp mocks base method.
func (m *MockNetworkHelper) SetLinkUp(link string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetLinkUp", link)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetLinkUp indicates an expected call of SetLinkUp.
func (mr *MockNetworkHelperMockRecorder) SetLinkUp(link any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetLinkUp", reflect.TypeOf((*MockNetworkHelper)(nil).SetLinkUp), link)
}
