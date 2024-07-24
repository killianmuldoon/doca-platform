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

## This script is tied to the implementation of the CI in GitLab.

set -o nounset
set -o pipefail
set -o errexit

E2E_TEST="${E2E_TEST:-""}"

## Ignore if the pipeline is not from a schedule or not from a "push" - i.e. when a new commit is added to main.
if [[ "$CI_PIPELINE_SOURCE" != "schedule" && "$CI_PIPELINE_SOURCE" != "push" ]]; then
  echo "Skipping slack notification: this is not a scheduled or push pipeline"
  exit 0
fi

## Exit if the job succeeded.
if [[ "$CI_JOB_STATUS" == "success" ]]; then
  echo "Skipping slack notification: this pipeline succeeded"
  exit 0
fi

notification_message="Pipeline ${CI_PIPELINE_NAME} has failed.\n\nDetails: ${CI_PIPELINE_URL}"

if [ "$E2E_TEST" == "true" ]; then
  notification_message="${notification_message}\n\nArtifacts download:\ncurl -L -o ${CI_JOB_ID}.zip --header \\\"PRIVATE-TOKEN: \$GITLAB_API_TOKEN\\\" https://gitlab-master.nvidia.com/api/v4/jobs/${CI_JOB_ID}/artifacts && unzip -d ${CI_JOB_ID} ${CI_JOB_ID}.zip && rm ${CI_JOB_ID}.zip"
fi

echo "{\"notification_message\":${notification_message}\"}"
curl -X POST -H "Content-type: application/json" --data "{\"notification_message\":\"${notification_message}\"}" $SLACK_WEBHOOK_URL
