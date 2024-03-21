# Contribution guide
This project is in an early development phase and has a lightweight contribution process.

## Merging changes
One approval from another reviewer and a positive CI signal is required to merge changes. Changes should be merged by the author.

## Reviews
Reviews should be lightweight to not slow down contributions.

Reviews should focus on:
- Structure - Does the code meet the structure defined for the repo?
- Suitable testing  - Is the happy path covered in unit and integration tests?

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
│         │         │         │         └── controlplane.dpf.nvidia.com_dpfclusters.yaml
│         └── dpuservice
│             ├── crd
│             │         ├── bases
│             │         │         └── svc.dpf.nvidia.com_dpuservices.yaml
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