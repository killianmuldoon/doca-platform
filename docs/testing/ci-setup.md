## DPF Operator CI setup

The DPF operator is currently tested using gitlab runners on specific bare metal machines. The runners can be viewed at https://gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/-/settings/ci_cd under #Runners.
The bare metal machines are running Ubuntu 22.04.

Some jobs are run periodically as configured through [Gitlab scheduled pipelines](https://gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/-/pipeline_schedules)

Jobs handled by the DPF can be viewed in the [gitlab-ci file](../../.gitlab-ci.yml)

This project uses three types of gitlab runners:
1) Docker runner
  - runs basic jobs such as unit tests and linters.
  - Dependencies:
    - Docker
  - home directory `/gitlab-runner` at root of host
  - user gitlab-runner with docker groups
  - cache directory at `/mnt/gitlab-runner/cache` which is mounted into the container through the config.toml.


The config.toml for the docker runner is located at /srv/gitlab-runner/config/config.toml on the host by default. It is shared between all docker containers which run jobs. It looks like:
```toml
concurrent = 10
check_interval = 0
shutdown_timeout = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = "cloud-dev-20"
  url = "https://gitlab-master.nvidia.com"
  id = 1173592
  token = "token"
  token_obtained_at = 2024-05-13T13:35:24Z
  token_expires_at = 0001-01-01T00:00:00Z
  executor = "docker"
  [runners.cache]
    MaxUploadedArchiveSize = 0
  [runners.docker]
    tls_verify = false
    image = "golang:1.22.0"
    privileged = false
    disable_entrypoint_overwrite = false
    oom_kill_disable = false
    disable_cache = false
    volumes = ["/mnt/gitlab-runner/cache:/cache"]
    shm_size = 0
    network_mtu = 0
    cpus = "4"
    memory = "4000000000"
```

2) e2e runner
  - A shell runner that runs end-to-end jobs that require spinning up a Kubernetes cluster.
  - home directory `/gitlab-runner` at root of host
  - user gitlab-runner with qemu, kvm and docker groups
  - Dependencies:
    - Docker
    - Kubectl
    - Gitlab shell runner as a systemd service with a non-root user that is a member of the docker group
    - A go version matching the version in the go.mod
The config.toml for the e2e runner is located at /etc/gitlab-runner/config.toml by default. It looks something like:
```toml
concurrent = 1
check_interval = 0
shutdown_timeout = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = "e2e-runner"
  url = "https://gitlab-master.nvidia.com"
  id = 1155778
  token = "token"
  token_obtained_at = 2024-04-19T09:27:17Z
  token_expires_at = 0001-01-01T00:00:00Z
  executor = "shell"
  [runners.custom_build_dir]
  [runners.cache]
    MaxUploadedArchiveSize = 0
```

3) release runner
  - A shell runner that runs jobs that build and push artifacts
  - home directory /gitlab-runner at root of host
  - user gitlab-runner with qemu, kvm and docker groups
  - Dependencies:
    - Docker 
    - Helm
    - Run the docker container from https://github.com/tonistiigi/binfmt to install necessary dependencies for building arm images. 
    - Gitlab shell runner as a systemd service with a non-root user that is a member of the docker group
    - Secrets for any registry the release runner is required to push artifacts to e.g. harbour, nvstaging
     
    
 The config.toml for the release runner is located at /etc/gitlab-runner/config.toml by default. It looks something like:
```toml
concurrent = 1
check_interval = 0
shutdown_timeout = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = "e2e-runner"
  url = "https://gitlab-master.nvidia.com"
  id = 1155778
  token = "token"
  token_obtained_at = 2024-04-19T09:27:17Z
  token_expires_at = 0001-01-01T00:00:00Z
  executor = "shell"
  [runners.custom_build_dir]
  [runners.cache]
    MaxUploadedArchiveSize = 0
```

## Hacks
### Pushing versioned images to local registry
The following images are pushed to a defined image registry to avoid docker pull limits
`clastix/kamaji:v0.4.1` -> `$(REGISTRY)/clastix/kamaji:v0.4.1`
`cfssl/cfssl:v1.6.5` -> `$(REGISTRY)cfssl/cfssl:v1.6.5`

This is done to avoid rate limiting of Docker pulls in the CI.
TODO:
- `clastix/kubectl` is still hosted on docker and should be moved - part of kamaji helm chart.
- `redis:7.0.14-alpine` is still hosted on docker and should be moved - part of argocd yaml.
- `docker.io/registry:2.8.3` is still hosted on docker and should be moved - part of minikube registry install.