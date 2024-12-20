/*
Copyright 2024 NVIDIA

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

package manager

import (
	"time"

	"github.com/nvidia/doca-platform/internal/storage/csi-plugin/handlers/controller"
	"github.com/nvidia/doca-platform/internal/storage/csi-plugin/handlers/identity"
	"github.com/nvidia/doca-platform/internal/storage/csi-plugin/handlers/node"
	"github.com/nvidia/doca-platform/internal/storage/csi-plugin/utils/runner"
	osWrapper "github.com/nvidia/doca-platform/internal/storage/csi-plugin/wrappers/os"

	"google.golang.org/grpc"
)

// NewGRPCServerFunc function to initialize GRPC server which will be used by manager
type NewGRPCServerFunc = func(opt ...grpc.ServerOption) *grpc.Server

// Option configure options for manager
type Option interface {
	set(*managerOptions)
}

type setFunc struct {
	f func(*managerOptions)
}

func (sf *setFunc) set(do *managerOptions) {
	sf.f(do)
}

// managerOptions contains options for manager.
type managerOptions struct {
	runner runner.Runner
	// new funcs
	newGRPCServerFunc NewGRPCServerFunc

	// wrappers
	osWrapper osWrapper.PkgWrapper

	// grpc handlers
	nodeHandler       node.Handler
	controllerHandler controller.Handler
	identityHandler   identity.Handler

	dependenciesWaitTimeout *time.Duration
}

// WithOSPkgWrapper set osPkgWrapper dependency for manager
func WithOSPkgWrapper(w osWrapper.PkgWrapper) Option {
	return &setFunc{f: func(o *managerOptions) {
		o.osWrapper = w
	}}
}

// WithNodeHandler set grpc node handler for manager
func WithNodeHandler(h node.Handler) Option {
	return &setFunc{f: func(o *managerOptions) {
		o.nodeHandler = h
	}}
}

// WithControllerHandler set grpc controller handler for manager
func WithControllerHandler(h controller.Handler) Option {
	return &setFunc{f: func(o *managerOptions) {
		o.controllerHandler = h
	}}
}

// WithIdentityHandler set grpc identity handler for manager
func WithIdentityHandler(h identity.Handler) Option {
	return &setFunc{f: func(o *managerOptions) {
		o.identityHandler = h
	}}
}

// WithNewGRPCServerFunc set function for GRPC server creation
func WithNewGRPCServerFunc(f NewGRPCServerFunc) Option {
	return &setFunc{f: func(o *managerOptions) {
		o.newGRPCServerFunc = f
	}}
}

// WithRunner configure runner which will be used by manager to start dependencies
func WithRunner(r runner.Runner) Option {
	return &setFunc{f: func(o *managerOptions) {
		o.runner = r
	}}
}

// WithDependenciesWaitTimeout configure timeout for dependencies waiting
func WithDependenciesWaitTimeout(t time.Duration) Option {
	return &setFunc{f: func(o *managerOptions) {
		o.dependenciesWaitTimeout = &t
	}}
}
