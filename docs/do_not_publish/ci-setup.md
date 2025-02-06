## DPF Operator CI setup
TODO: Remove this doc before moving to github


The DPF operator is currently tested using gitlab runners on specific bare metal machines. The runners can be viewed at https://gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/-/settings/ci_cd under #Runners.
The bare metal machines are running Ubuntu 22.04.

Some jobs are run periodically as configured through [Gitlab scheduled pipelines](https://gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/-/pipeline_schedules)

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
    image = "golang:1.23.4"
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
    - OpenJDK JRE
    - Gitlab shell runner as a systemd service with a non-root user that is a member of the docker group
    - A go version matching the version in the go.mod
    - Secrets for any registry the release runner is required to push artifacts to e.g. harbour, nvstaging
    
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
to run the script you will need to provide the environment with the tokens listed below.
Note: in case of using a leased setup from Colossus you can use root user by using `sudo su`
after you logged in with the user who made the lease.
* use OS: ubuntu-22.04-x86_64-standard<br>
Link to colossus lease:<br>
- https://colossus.nvidia.com/web/index.html#/schedule

```shell
# Should have permissions for creating a gitlab runner.
export GITLAB_RUNNER_API_TOKEN="<GITLAB_RUNNER_API_TOKEN>"

# curl and run the gitlab_runner.sh from main
cat <<EOF | ssh "<ip address of runner>"
export GITLAB_RUNNER_API_TOKEN=$GITLAB_RUNNER_API_TOKEN
curl --header "PRIVATE-TOKEN: $GITLAB_RUNNER_API_TOKEN" "https://gitlab-master.nvidia.com/api/v4/projects/112105/repository/files/hack%2Fscripts%2Fgitlab_runner.sh/raw?ref=main" | bash --login
EOF
```
