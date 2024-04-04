_## DPF Operator setup


The DPF operator is currently tested using gitlab runners on specific bare metal machines. The runners can be viewed at https://gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/-/settings/ci_cd under #Runners.
The bare metal machines are running Ubuntu 22.04.

Some jobs are run periodically as configured through [Gitlab scheduled pipelines](https://gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/-/pipeline_schedules)

Jobs handled by the DPF can be viewed in the [gitlab-ci file](../../.gitlab-ci.yml)

This project uses three types of gitlab runners:
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
- release runner
  - runs jobs that build and push artifacts
  - Dependencies:
    - Docker 
    - Helm
    - apt packages `qemu-user-static` `binfmt-support`
    - Gitlab shell runner as a systemd service with a non-root user that is a member of the docker group
    - Secrets for any registry the release runner is required to push artifacts to e.g. harbour, nvstaging
    

