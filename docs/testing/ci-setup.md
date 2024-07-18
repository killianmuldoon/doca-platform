## DPF Operator CI setup

The DPF operator is currently tested using gitlab runners on specific bare metal machines. The runners can be viewed at https://gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/-/settings/ci_cd under #Runners.
The bare metal machines are running Ubuntu 22.04.

Some jobs are run periodically as configured through [Gitlab scheduled pipelines](https://gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/-/pipeline_schedules)

Jobs handled by the DPF can be viewed in the [gitlab-ci file](../../.gitlab-ci.yml)

This project uses two types of gitlab runners:
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

2) e2e and release runner
  - A shell runner that runs end-to-end jobs that require spinning up a Kubernetes cluster.
  - home directory `/gitlab-runner` at root of host
  - user gitlab-runner with docker groups
  - Dependencies:
    - Docker
    - Kubectl
    - Gitlab shell runner as a systemd service with a non-root user that is a member of the docker group
    - A go version matching the version in the go.mod
    - Run the docker container from https://github.com/tonistiigi/binfmt to install necessary dependencies for building arm images.
    - Secrets for any registry the release runner is required to push artifacts to e.g. harbour, nvstaging
    - 
The config.toml for the e2e runner is located at /etc/gitlab-runner/config.toml by default. It looks like:
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

The below set of bash commands can be used to setup a new gitlab runner on ubuntu.
Note: The below is for illustration only and is not intended as a runnable script.
All commands are run as root from /root unless otherwise specified.

```shell
## Suspend the ubuntu needrestart from interrupting installs
export NEEDRESTART_SUSPEND=true

## Add the gitlab-runner user
sudo useradd -m -d /gitlab-runner gitlab-runner

## install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

## Install docker
sudo apt-get update
sudo apt-get install ca-certificates curl lftp
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update

sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

## Add the gitlab runner to the docker group
sudo usermod  -aG docker gitlab-runner

## Install go
wget https://go.dev/dl/go1.22.3.linux-amd64.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.3.linux-amd64.tar.gz
## Set the go path for the gitlab-runner
echo PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/games:/usr/local/games:/usr/local/bin:/usr/local/go/bin" > /gitlab-runner/.bashrc

## Enable docker building with qemu for multi-arch
docker run --privileged --rm tonistiigi/binfmt --install all


## Set the inotify limits for the host.
## This fixes a common "too many open files" error on some systems.
sudo sysctl fs.inotify.max_user_watches=1048576
sudo sysctl fs.inotify.max_user_instances=8192

## Delete .bash_logout
sudo rm -rf /gitlab-runner/.bash_logout

## Download the gitlab runner binary
sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/v17.1.0/binaries/gitlab-runner-linux-amd64"
sudo chmod +x /usr/local/bin/gitlab-runner

## install gitlab runner and register 
sudo gitlab-runner install --user=gitlab-runner --working-directory=/gitlab-runner

sudo gitlab-runner start

## Runner must be registered in the CI as a shell runner with command from gitlab UI.
gitlab-runner register XXXX 

## Must be done with credentials that allow pushing to the given registry.
## Must be done as the gitlab-runner user.
su gitlab-runner
docker login nvcr.io #Note. This is an interactive command and needs to be done manually.
docker login gitlab-master.nvidia.com:5005 #Note. This is an interactive command and needs to be done manually.

```