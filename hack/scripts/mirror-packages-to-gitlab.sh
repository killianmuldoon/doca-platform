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

# This script pulls files from a known location in the NBU NFS drive and pushes those files to the gitlab generic package registry.
SOURCE_URL="http://nbu-nfs.mellanox.com/auto/swgwork/dpf/build/."

# The ID assigned to the project in Gitlab.
PROJECT_ID=112105

# Directory on which to store the files locally before upload.
TMP_DIR=/tmp/dpf-build-files

mkdir -p $TMP_DIR
cd $TMP_DIR

lftp -c "mirror ${SOURCE_URL}"

for i in $(find $TMP_DIR -type f ); do
    path="${i#"$TMP_DIR"/}"
    echo "uploading ${i}\n"
    curl --fail-with-body --header "PRIVATE-TOKEN:${GITLAB_REGISTRY_TOKEN}" \
         --upload-file $i \
         "https://gitlab-master.nvidia.com/api/v4/projects/$PROJECT_ID/packages/generic/${path}"
done


 rm -rf $TMP_DIR