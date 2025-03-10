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

set -o nounset
set -o pipefail
set -o errexit

GITHUB_RELEASE_TOKEN="${GITHUB_RELEASE_TOKEN:-""}"

# This script logs in to both nvcr and the gitlab registries for helm and docker using tokens taken from the environment.
if [ -z "$GITLAB_REGISTRY_TOKEN" ] || [ -z "$NGC_API_KEY" ]; then
    echo "GITLAB_REGISTRY_TOKEN and NGC_API_KEY variables are required"
    exit 1
fi

echo "Pruning docker images and build cache older than 48 hours"
docker system prune --filter "until=48h" --all --force

## log in to the docker registries
echo "$GITLAB_REGISTRY_TOKEN" | docker login --username \$oauthtoken --password-stdin gitlab-master.nvidia.com:5005
echo "$NGC_API_KEY" | docker login --username \$oauthtoken --password-stdin nvcr.io

## log in to helm registries
echo "$NGC_API_KEY" | helm registry login nvcr.io --username \$oauthtoken --password-stdin
echo "$GITLAB_REGISTRY_TOKEN" | helm registry login gitlab-master.nvidia.com:5005 --username \$oauthtoken --password-stdin

## Always attempt to log out of this registry as this token is only included during releases.
docker logout ghcr.io/nvidia
## if this is a public release the GITLAB_RELEASE_TOKEN will be set and we should log in.
if [ -n "$GITHUB_RELEASE_TOKEN" ]; then
  echo "$GITHUB_RELEASE_TOKEN" | docker login --username USERNAME --password-stdin ghcr.io/nvidia
fi

# Run the binfmt container to enable multi-architecture builds
docker run --privileged --rm tonistiigi/binfmt --install all
