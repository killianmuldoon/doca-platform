#!/bin/bash

#  2025 NVIDIA CORPORATION & AFFILIATES
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

: ${BLACK_DUCK_API_TOKEN:?env not set}
: ${TAG:?env not set}
: ${GERRIT_USERNAME:?env not set}
: ${GERRIT_PASSWORD:?env not set}

export SPRING_APPLICATION_JSON='{"blackduck.url":"https://blackduck.mellanox.com/","blackduck.api.token":"'${BLACK_DUCK_API_TOKEN}'"}'
export PROJECT_NAME="DOCA-Platform-Foundation"
export PROJECT_VERSION="${TAG}"

# This script assumes that is called by the root of the project
export PROJECT_SRC_PATH="$(pwd)"

tmp_dir=$(mktemp -d)
trap "rm -rf ${tmp_dir}" EXIT

# https://confluence.nvidia.com/pages/viewpage.action?spaceKey=SW&title=Blackduck
git clone "https://${GERRIT_USERNAME}:${GERRIT_PASSWORD}@git-nbu.nvidia.com/r/a/DevOps/Tools/blackduck" ${tmp_dir}

cd ${tmp_dir}

echo "Running Black Duck scan, this may take a while.."

./run_bd_scan.sh | sed "s|${BLACK_DUCK_API_TOKEN}|REDACTED|g"

# Print all the output produced by the script for debugging purposes
find ${tmp_dir}/log -type f -print -exec cat {} \;
