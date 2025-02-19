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
//

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ovn-org/libovsdb/client (interfaces: ConditionalAPI)
//
// Generated by this command:
//
//	mockgen -copyright_file ../../hack/boilerplate.go.txt --build_flags=--mod=mod -package ovsutils -destination mock_ovs_conditional_api.go github.com/ovn-org/libovsdb/client ConditionalAPI
//

// Package ovsutils is a generated GoMock package.
package ovsutils

import (
	context "context"
	reflect "reflect"

	model "github.com/ovn-org/libovsdb/model"
	ovsdb "github.com/ovn-org/libovsdb/ovsdb"
	gomock "go.uber.org/mock/gomock"
)

// MockConditionalAPI is a mock of ConditionalAPI interface.
type MockConditionalAPI struct {
	ctrl     *gomock.Controller
	recorder *MockConditionalAPIMockRecorder
	isgomock struct{}
}

// MockConditionalAPIMockRecorder is the mock recorder for MockConditionalAPI.
type MockConditionalAPIMockRecorder struct {
	mock *MockConditionalAPI
}

// NewMockConditionalAPI creates a new mock instance.
func NewMockConditionalAPI(ctrl *gomock.Controller) *MockConditionalAPI {
	mock := &MockConditionalAPI{ctrl: ctrl}
	mock.recorder = &MockConditionalAPIMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockConditionalAPI) EXPECT() *MockConditionalAPIMockRecorder {
	return m.recorder
}

// Delete mocks base method.
func (m *MockConditionalAPI) Delete() ([]ovsdb.Operation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete")
	ret0, _ := ret[0].([]ovsdb.Operation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Delete indicates an expected call of Delete.
func (mr *MockConditionalAPIMockRecorder) Delete() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockConditionalAPI)(nil).Delete))
}

// List mocks base method.
func (m *MockConditionalAPI) List(ctx context.Context, result any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", ctx, result)
	ret0, _ := ret[0].(error)
	return ret0
}

// List indicates an expected call of List.
func (mr *MockConditionalAPIMockRecorder) List(ctx, result any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockConditionalAPI)(nil).List), ctx, result)
}

// Mutate mocks base method.
func (m *MockConditionalAPI) Mutate(arg0 model.Model, arg1 ...model.Mutation) ([]ovsdb.Operation, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Mutate", varargs...)
	ret0, _ := ret[0].([]ovsdb.Operation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Mutate indicates an expected call of Mutate.
func (mr *MockConditionalAPIMockRecorder) Mutate(arg0 any, arg1 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Mutate", reflect.TypeOf((*MockConditionalAPI)(nil).Mutate), varargs...)
}

// Update mocks base method.
func (m *MockConditionalAPI) Update(arg0 model.Model, arg1 ...any) ([]ovsdb.Operation, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Update", varargs...)
	ret0, _ := ret[0].([]ovsdb.Operation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Update indicates an expected call of Update.
func (mr *MockConditionalAPIMockRecorder) Update(arg0 any, arg1 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockConditionalAPI)(nil).Update), varargs...)
}

// Wait mocks base method.
func (m *MockConditionalAPI) Wait(arg0 ovsdb.WaitCondition, arg1 *int, arg2 model.Model, arg3 ...any) ([]ovsdb.Operation, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Wait", varargs...)
	ret0, _ := ret[0].([]ovsdb.Operation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Wait indicates an expected call of Wait.
func (mr *MockConditionalAPIMockRecorder) Wait(arg0, arg1, arg2 any, arg3 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1, arg2}, arg3...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Wait", reflect.TypeOf((*MockConditionalAPI)(nil).Wait), varargs...)
}
