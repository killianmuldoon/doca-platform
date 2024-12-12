#!/bin/bash

#  2024 NVIDIA CORPORATION & AFFILIATES
#
#  Licensed under the Apache License, Version 2.0 (the License);
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an AS IS BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

set -euo pipefail

LOGFILE="setup_log.txt"
exec > >(tee -i $LOGFILE)
exec 2>&1

log_and_exit() {
    echo -e "$1" >&2
    exit 1
}

## Verify if gitlab-runner is already running
if command -v gitlab-runner &>/dev/null && sudo gitlab-runner status |& grep -q "gitlab-runner: Service is running"; then
  log_and_exit "gitlab-runner already runner, please stop and uninstall it first:\n  sudo gitlab-runner stop\n  sudo gitlab-runner uninstall"
fi

## Check for required variables
if [ -z "$GITLAB_RUNNER_API_TOKEN" ]; then
    log_and_exit "A GITLAB_RUNNER_API_TOKEN variable is required"
fi

GITLAB_RUNNER_USER="${GITLAB_RUNNER_USER:-"gitlab-runner"}"
INSTALL_DOCKER="${INSTALL_DOCKER:-"true"}"
REGISTER_DOCKER_RUNNER="${REGISTER_DOCKER_RUNNER:-"true"}"


## GLOBAL VARS
PROJECT_ID="112105"
SHELL_TAG_LIST="${SHELL_TAG_LIST:-"type/shell,e2e,release"}"
DOCKER_TAG_LIST="type/docker"
GO_VERSION="1.22.3"
GITLAB_RUNNER_VERSION="17.1.0"

## Install JQ for json parsing
sudo apt-get install -y -qq jq > /dev/null || log_and_exit "Failed to download jq"

## Suspend the ubuntu needrestart from interrupting installs
export NEEDRESTART_SUSPEND=true

## Add the "${GITLAB_RUNNER_USER}" if it does not exist yet.
if ! id "${GITLAB_RUNNER_USER}" &>/dev/null; then
  sudo useradd -m -d /gitlab-runner "${GITLAB_RUNNER_USER}" || log_and_exit "Failed to add gitlab runner user"
fi

## Ensure this directory is always created.
sudo mkdir -p /gitlab-runner

## Install kubectl
LATEST_K8S_VERSION="$(curl -L -s https://dl.k8s.io/release/stable.txt)"
if ! command -v kubectl &>/dev/null || [[ "$(kubectl version --client -ojson | jq -r .clientVersion.gitVersion)" != "${LATEST_K8S_VERSION}" ]]; then
  curl -L --output /tmp/kubectl "https://dl.k8s.io/release/${LATEST_K8S_VERSION}/bin/linux/amd64/kubectl" > /dev/null || log_and_exit "Failed to download kubectl"
  sudo install -o root -g root -m 0755 /tmp/kubectl /usr/local/bin/kubectl || log_and_exit "Failed to install kubectl"
fi

## Install docker
if [[ "$INSTALL_DOCKER" == "true" ]]; then
  sudo apt-get update > /dev/null || log_and_exit "Failed to update package list"
  sudo apt-get install -y ca-certificates curl apt-transport-https lftp > /dev/null || log_and_exit "Failed to install required packages"
  sudo install -m 0755 -d /etc/apt/keyrings || log_and_exit "Failed to create /etc/apt/keyrings directory"
  sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc || log_and_exit "Failed to download Docker GPG key"
  sudo chmod a+r /etc/apt/keyrings/docker.asc || log_and_exit "Failed to set permissions on Docker GPG key"

  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null || log_and_exit "Failed to add Docker repository"
  sudo apt-get update > /dev/null || log_and_exit "Failed to update package list after adding Docker repository"
  sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin > /dev/null || log_and_exit "Failed to install Docker packages"
fi

## Install helm
curl https://baltocdn.com/helm/signing.asc | gpg --dearmor | sudo tee /usr/share/keyrings/helm.gpg > /dev/null || log_and_exit "Failed to add helm signing key"
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/helm.gpg] https://baltocdn.com/helm/stable/debian/ all main" | sudo tee /etc/apt/sources.list.d/helm-stable-debian.list || log_and_exit "Failed to add helm repository"
sudo apt-get update > /dev/null || log_and_exit "Failed to update package list after adding helm repository"
sudo apt-get install -y helm  || log_and_exit "Failed to install helm packages"

## Add the gitlab runner to the docker group
sudo usermod -aG docker "${GITLAB_RUNNER_USER}" || log_and_exit "Failed to add gitlab runner user to docker group"

## Install Go
if ! command -v go &>/dev/null || [[ "$(go version | awk '{print $3}')" != "go${GO_VERSION}" ]]; then
  curl -L --output /tmp/go${GO_VERSION}.linux-amd64.tar.gz https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz > /dev/null || log_and_exit "Failed to download Go"
  sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf /tmp/go${GO_VERSION}.linux-amd64.tar.gz || log_and_exit "Failed to extract Go archive"
  echo 'PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/bin:/usr/local/go/bin"' | sudo tee /etc/environment > /dev/null || log_and_exit "Failed to set Go path for gitlab-runner"
fi

## Set the inotify limits for the host
sudo sysctl fs.inotify.max_user_watches=1048576 > /dev/null || log_and_exit "Failed to set fs.inotify.max_user_watches"
sudo sysctl fs.inotify.max_user_instances=8192 > /dev/null || log_and_exit "Failed to set fs.inotify.max_user_instances"

## Enable Docker building with qemu for multi-arch
sudo docker run --privileged --rm tonistiigi/binfmt --install all > /dev/null || log_and_exit "Failed to enable Docker building with qemu"

## Delete .bash_logout
sudo rm -rf /"${GITLAB_RUNNER_USER}"/.bash_logout > /dev/null || log_and_exit "Failed to delete gitlab-runner $HOME/.bash_logout"

if ! command -v gitlab-runner &>/dev/null || [[ "$(gitlab-runner --version | awk '/Version/{print $2}')" != "${GITLAB_RUNNER_VERSION}" ]]; then
  ## Download the gitlab runner binary
  sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/v${GITLAB_RUNNER_VERSION}/binaries/gitlab-runner-linux-amd64" > /dev/null || log_and_exit "Failed to download gitlab-runner binary"
  sudo chmod +x /usr/local/bin/gitlab-runner || log_and_exit "Failed to make gitlab-runner binary executable"
fi

## Install gitlab runner and register
sudo gitlab-runner install --user="${GITLAB_RUNNER_USER}" --working-directory=/gitlab-runner || log_and_exit "Failed to install gitlab-runner"
sudo gitlab-runner start || log_and_exit "Failed to start gitlab-runner"

## Runner must be registered in the CI as a shell runner with command from gitlab UI.
## Please run: gitlab-runner register

## Register shell runner
register_token=$(curl --silent --request POST --url "https://gitlab-master.nvidia.com/api/v4/user/runners" \
  --data "runner_type=project_type" \
  --data "project_id=$PROJECT_ID" \
  --data "description=$(hostname) - $(ip route show default | awk '/^default/ {print $9}')" \
  --data "tag_list=$SHELL_TAG_LIST" \
  --data "run_untagged=false" \
  --header "PRIVATE-TOKEN: $GITLAB_RUNNER_API_TOKEN" | jq -r '.token')

sudo gitlab-runner register  --non-interactive \
  --url https://gitlab-master.nvidia.com \
  --token "$register_token" \
  --executor="shell" || log_and_exit "Failed to register shell gitlab-runner"


if [[ "$REGISTER_DOCKER_RUNNER" == "true" ]]; then
  ## Register docker runner
  register_token=$(curl --silent --request POST --url "https://gitlab-master.nvidia.com/api/v4/user/runners" \
    --data "runner_type=project_type" \
    --data "project_id=$PROJECT_ID" \
    --data "description=$(hostname) - $(ip route show default | awk '/^default/ {print $9}')" \
    --data "tag_list=$DOCKER_TAG_LIST" \
    --data "run_untagged=true" \
    --header "PRIVATE-TOKEN: $GITLAB_RUNNER_API_TOKEN" | jq -r '.token')

  sudo gitlab-runner register  --non-interactive \
    --url https://gitlab-master.nvidia.com \
    --token "$register_token" \
    --executor="docker" \
    --docker-tlsverify=false \
    --docker-image golang:1.22.0 \
    --docker-privileged=false \
    --docker-disable-entrypoint-overwrite=false \
    --docker-oom-kill-disable=false \
    --docker-disable-cache=false \
    --docker-volumes="/mnt/gitlab-runner/cache:/cache" \
    --docker-shm-size=0 \
    --docker-network-mtu=0 \
    --docker-memory="8000000000" || log_and_exit "Failed to register docker gitlab-runner"
fi
# Done
echo "GitLab runner setup completed successfully"
