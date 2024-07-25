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

LOGFILE="setup_log.txt"
exec > >(tee -i $LOGFILE)
exec 2>&1

log_and_exit() {
    echo "$1" >&2
    exit 1
}

## Check for required variables
if [ -z "$GROUP_ACCESS_TOKEN" ]; then
    log_and_exit "All GROUP_ACCESS_TOKEN variable is required"
fi

## GLOBAL VARS
GROUP_ID="151554"
SHELL_TAG_LIST="type/shell,e2e,release"
DOCKER_TAG_LIST="type/docker"

## Install JQ for json parsing
sudo apt-get install -y -qq jq > /dev/null || log_and_exit "Failed to download jq"

## Suspend the ubuntu needrestart from interrupting installs
export NEEDRESTART_SUSPEND=true

## Add the gitlab-runner user
sudo useradd -m -d /gitlab-runner gitlab-runner || log_and_exit "Failed to add gitlab-runner user"

## Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" > /dev/null || log_and_exit "Failed to download kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl || log_and_exit "Failed to install kubectl"

## Install docker
sudo apt-get update > /dev/null || log_and_exit "Failed to update package list"
sudo apt-get install -y ca-certificates curl apt-transport-https lftp > /dev/null || log_and_exit "Failed to install required packages"
sudo install -m 0755 -d /etc/apt/keyrings || log_and_exit "Failed to create /etc/apt/keyrings directory"
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc || log_and_exit "Failed to download Docker GPG key"
sudo chmod a+r /etc/apt/keyrings/docker.asc || log_and_exit "Failed to set permissions on Docker GPG key"

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null || log_and_exit "Failed to add Docker repository"
sudo apt-get update > /dev/null || log_and_exit "Failed to update package list after adding Docker repository"
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin > /dev/null || log_and_exit "Failed to install Docker packages"

## Install helm
sudo curl https://baltocdn.com/helm/signing.asc | gpg --dearmor | sudo tee /usr/share/keyrings/helm.gpg > /dev/null || log_and_exit "Failed to add helm signing key"
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/helm.gpg] https://baltocdn.com/helm/stable/debian/ all main" | sudo tee /etc/apt/sources.list.d/helm-stable-debian.list || log_and_exit "Failed to add helm repository"
sudo apt-get update > /dev/null || log_and_exit "Failed to update package list after adding helm repository"
sudo apt-get install -y helm  || log_and_exit "Failed to install helm packages"

## Add the gitlab runner to the docker group
sudo usermod -aG docker gitlab-runner || log_and_exit "Failed to add gitlab-runner to docker group"

## Install Go
sudo wget https://go.dev/dl/go1.22.3.linux-amd64.tar.gz > /dev/null || log_and_exit "Failed to download Go"
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.22.3.linux-amd64.tar.gz || log_and_exit "Failed to extract Go archive"
echo 'PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/games:/usr/local/games:/usr/local/bin:/usr/local/go/bin"' | sudo tee /gitlab-runner/.bashrc > /dev/null || log_and_exit "Failed to set Go path for gitlab-runner"

## Enable Docker building with qemu for multi-arch
sudo docker run --privileged --rm tonistiigi/binfmt --install all > /dev/null || log_and_exit "Failed to enable Docker building with qemu"

## Set the inotify limits for the host
sudo sysctl fs.inotify.max_user_watches=1048576 > /dev/null || log_and_exit "Failed to set fs.inotify.max_user_watches"
sudo sysctl fs.inotify.max_user_instances=8192 > /dev/null || log_and_exit "Failed to set fs.inotify.max_user_instances"

## Delete .bash_logout
sudo rm -rf /gitlab-runner/.bash_logout > /dev/null || log_and_exit "Failed to delete /gitlab-runner/.bash_logout"

## Download the gitlab runner binary
sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/v17.1.0/binaries/gitlab-runner-linux-amd64" > /dev/null || log_and_exit "Failed to download gitlab-runner binary"
sudo chmod +x /usr/local/bin/gitlab-runner || log_and_exit "Failed to make gitlab-runner binary executable"

## Install gitlab runner and register
sudo gitlab-runner install --user=gitlab-runner --working-directory=/gitlab-runner || log_and_exit "Failed to install gitlab-runner"
sudo gitlab-runner start || log_and_exit "Failed to start gitlab-runner"

## Runner must be registered in the CI as a shell runner with command from gitlab UI.
## Please run: gitlab-runner register

## Register shell runner
register_token=$(curl --silent --request POST --url "https://gitlab-master.nvidia.com/api/v4/user/runners" \
  --data "runner_type=group_type" \
  --data "group_id=$GROUP_ID" \
  --data "description=$(hostname) - $(hostname -I | awk '{print $1}')" \
  --data "tag_list=$SHELL_TAG_LIST" \
  --data "run_untagged=false" \
  --header "PRIVATE-TOKEN: $GROUP_ACCESS_TOKEN" | jq -r '.token')

gitlab-runner register  --non-interactive \
  --url https://gitlab-master.nvidia.com \
  --token "$register_token" \
  --executor="shell" || log_and_exit "Failed to register shell gitlab-runner"


## Register docker runner
register_token=$(curl --silent --request POST --url "https://gitlab-master.nvidia.com/api/v4/user/runners" \
  --data "runner_type=group_type" \
  --data "group_id=$GROUP_ID" \
  --data "description=$(hostname) - $(hostname -I | awk '{print $1}')" \
  --data "tag_list=$DOCKER_TAG_LIST" \
  --data "run_untagged=true" \
  --header "PRIVATE-TOKEN: $GROUP_ACCESS_TOKEN" | jq -r '.token')

gitlab-runner register  --non-interactive \
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
  --docker-cpus="4" \
  --docker-memory="4000000000" || log_and_exit "Failed to register docker gitlab-runner"

# Done
echo "GitLab runner setup completed successfully"
