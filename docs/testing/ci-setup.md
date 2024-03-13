## DPF Operator setup


The DPF operator is currently tested using gitlab runners on specific bare metal machines. The runners can be viewed at https://gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/-/settings/ci_cd under #Runners.
The bare metal machines are running Ubuntu 22.04.

Jobs handled by the DPF can be viewed in the [gitlab-ci file](../../.gitlab-ci.yml)

This project uses two types of gitlab runners:
- Docker runner
  - runs basic jobs such as unit tests and linters.
  - Dependencies:
    - Docker
- e2e runner
  - runs end-to-end jobs that require spinning up a Kubernetes cluster.
  - Dependencies:
    - Docker
    - Kubectl
    - Gitlab shell runner as a systemd service with a non-root user that is a member of the docker group
    - A go version matching the version in the go.mod
