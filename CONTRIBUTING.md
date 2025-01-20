# Contribution guide

This guide is intended for anyone working with the DPF code base, whether contributing
new code or documentation, or reviewing contributions. The purpose of this guide
is to assist new contributors in finding the information they need to get started.
It also aims to ensure the ongoing improvement of the code base by outlining expectations
for code quality, testing, and documentation.

# Understanding the project

A couple of resources exist to understand the [architecture](./docs/architecture) of DPF and an introductory demo is available.

The [API](./api) folder contains all the custom resources definitions.

You will find corresponding documentation on how to use each CR under [guides](./docs/guides).

## Understanding the code

We use [controller-runtime](https://pkg.go.dev/sigs.k8s.io/controller-runtime) to develop our controllers.

## Repository Structure

The following general principles should be taken into account when reviewing a PR for structure:
- Components share dependencies - be a single go module - unless there are specific reasons to introduce another module.
- Components share build components and general tooling - e.g. Dockerfile, Makefile unless there are specific reasons not to.
- Components define a directory structure with their component name under the top level directories where appropriate. e.g. `api/component-name`, `cmd/component-name`, and `config/component-name`.
- Components add code generation, build process and other development flow targets to the `Makefile`.
- Code is under the `internal` package unless an explicit decision is taken to share it.
- Controllers are in a `controllers` folder under their component name.
- A shared package is on the same level in the directory structure as its highest-level consumer.

  
The below is a representation of how multiple components are structured in the repo:
```
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
├── doc
├── hack
├── test
├── README.md
├── api
│      ├── controlplane
│      │         └── v1alpha1
│      │             ├── dpfcluster_types.go
│      └── dpuservice
│             └── v1alpha1
│                 ├── dpuservice_types.go
├── cmd
│      ├── controlplane
│      │         └── main.go
│      └── dpuservice
│          └── main.go
├── config
│         ├── controlplane
│         │         ├── crd
│         │         │         ├── bases
│         │         │         │         └── controlplane.dpu.nvidia.com_dpfclusters.yaml
│         └── dpuservice
│             ├── crd
│             │         ├── bases
│             │         │         └── svc.dpu.nvidia.com_dpuservices.yaml
│             │         ├── kustomization.yaml
│             │         └── kustomizeconfig.yaml
│             ├── default
│             │         ├── kustomization.yaml
│             │         └── manager_config_patch.yaml
│             ├── manager
│             │         ├── kustomization.yaml
│             │         └── manager.yaml
│             ├── prometheus
│             │         ├── kustomization.yaml
│             │         └── monitor.yaml
│             ├── rbac
│             │         ├── auth_proxy_client_clusterrole.yaml
│             │         ├── auth_proxy_role.yaml
│             │         ├── auth_proxy_role_binding.yaml
│             │         ├── auth_proxy_service.yaml
│             │         ├── controlplane.dpf_dpfcluster_editor_role.yaml
│             │         ├── controlplane.dpf_dpfcluster_viewer_role.yaml
│             │         ├── kustomization.yaml
│             │         ├── leader_election_role.yaml
│             │         ├── leader_election_role_binding.yaml
│             │         ├── role.yaml
│             │         ├── role_binding.yaml
│             │         ├── service_account.yaml
│             │         ├── svc.dpf_dpuservice_editor_role.yaml
│             │         └── svc.dpf_dpuservice_viewer_role.yaml
│             └── samples
│                 ├── controlplane.dpf_v1alpha1_dpfcluster.yaml
│                 ├── kustomization.yaml
│                 └── svc.dpf_v1alpha1_dpuservice.yaml
├── internal
│         ├── controlplane
│         │         └── controllers
│         │             ├── dpfcluster_controller.go
│         └── dpuservice
│             └── controllers
│                 ├── dpuservice_controller.go
```

# Documentation

If a contribution alters how users interact with, build, or test our product, the
documentation must be updated accordingly. API-related changes should include corresponding
updates in the [guide](./docs/guides/) directory. Specific use cases that involve
multiple changes and require extensive technical writing are beyond the scope of
this guide and should be addressed in its own separate change.

# Style

We use Golang as our primary language. The Go team has compiled a set of comments
from code reviews and made them available. All code must adhere to the
[Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) guidelines.

The [kubernetes api-conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
are a good practice to adopt.

There is also the [conditions](https://docs.google.com/document/d/1xD-dILlCL6NIRwfx86bSlmt9OD-M6--cFxPCAiWCWZA/edit?tab=t.0#heading=h.6h9i92rt5dee)
doc that list our convention in regards to `status` conditions.

## Log Convention

- `klog.V(0).InfoS` = `klog.InfoS` - Generally useful for this to always be visible to a cluster operator
- `klog.V(1).InfoS` - A reasonable default log level if you don't want verbosity.
- `klog.V(2).InfoS` - Useful steady state information about the service and important log messages that may correlate to significant changes in the system. This is the recommended default log level for most systems.
- `klog.V(3).InfoS` - Extended information about changes
- `klog.V(4).InfoS` - Debug level verbosity

xref: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#what-method-to-use

# Tests

We expect contributors to thoroughly test their contributions to ensure they work
correctly before reaching the code review stage. New contributions should include tests,
as this helps improve the robustness and overall health of the code base by checking
for correct behavior and preventing regressions. Tests can be either unit tests or
end-to-end tests, depending on the specific case.


# Developer Certificate of Origin

By contributing to this project you agree to the Developer Certificate of Origin (DCO).
This document was created by the Linux Kernel community and is a simple statement that you,
as a contributor, have the legal right to make the contribution.

All contributions comply with the [Developer Certificate of Origin](./DCO)


# Run and Release DPF

Prerequisites:
- go >= 1.22
- kubectl >= 1.29
- kustomize >= 5.4
- coreutils (on Mac OS)
- docker buildx
- ginkgo (optional)

## Run the test suite

You can run the unit tests with:

```shell
make test
```

Add specific test arguments:

```shell
GO_TEST_ARGS="-race" make test
```

We use `ginkgo`  in our controllers. To run a specific test:

``` shell
ginkgo -v --focus "should update the existing disruptive DPUService on update of the DPUServiceCo
nfiguration" internal/dpuservice/controllers/
```

A specific [guide](./docs/dev/minikube-local-dev.md) exists to run tests with `minikube`.

## Release DPF

In order to release the whole project for a specific architecture:

```shell
ARCH=amd64 REGISTRY=harbor.mellanox.com/<repo_namespace> TAG=v0.0.0-<xxx> make release
```

# Review Process

One approval from another reviewer and a positive CI signal is required to merge changes.
For substantial changes, or if a maintainer feels unqualified to review a specific part,
additional reviewers will be requested. This may involve more people in the review process,
and you might be asked to resubmit the pull request (PR) or divide the changes into multiple PRs.

The reviewer will evaluate the PR based on the following criteria:
- Is the functionality useful?
- Does the code perform as intended?
- Is the user experience satisfactory?
- Are tests included, correct, sensible, and useful?
- Is the documentation updated accordingly?

# Format of the Commit Message

We use the [conventional commit standard](https://www.conventionalcommits.org/en/v1.0.0/).
For more helpful advice on documenting your work, refer to the [following article](https://chris.beams.io/posts/git-commit/#seven-rules).
