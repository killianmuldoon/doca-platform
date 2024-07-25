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

# This script logs in to both nvcr and the gitlab registries for helm and docker using tokens taken from the environment.
if [ -z "$GITLAB_REGISTRY_TOKEN" ] || [ -z "$NGC_API_KEY" ]; then
    echo "GITLAB_REGISTRY_TOKEN and NGC_API_KEY variables are required"
    exit 1
fi

## log in to the docker registries
echo "$GITLAB_REGISTRY_TOKEN" | docker login --username \$oauthtoken --password-stdin gitlab-master.nvidia.com:5005
echo "$NGC_API_KEY" | docker login --username \$oauthtoken --password-stdin nvcr.io
## log in to helm registries
echo "$NGC_API_KEY" | helm registry login nvcr.io --username \$oauthtoken --password-stdin
echo "$GITLAB_REGISTRY_TOKEN" | helm registry login gitlab-master.nvidia.com:5005 --username \$oauthtoken --password-stdin
