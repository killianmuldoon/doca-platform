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

PIPELINE_NAME="${PIPELINE_NAME:-"${CI_PIPELINE_NAME}"}"

# This limit is set with the DPF release job in mind. This job currently runs up to 3 times.
limit="${RETRY_LIMIT:-3}"

# Ignore if the pipeline is not from a schedule or not from a "push" - i.e. when a new commit is added to main.
if [[ "$CI_PIPELINE_SOURCE" != "schedule" && "$CI_PIPELINE_SOURCE" != "push" ]]; then
  echo "Skipping slack notification: this is not a scheduled or push pipeline"
  exit 0
fi

## $CI_COMMIT_BRANCH will not be available on merge request pipelines.
PIPELINE_NAME="$CI_COMMIT_BRANCH $PIPELINE_NAME"

## Exit if the job succeeded.
if [[ "$CI_JOB_STATUS" == "success" ]]; then
  echo "Skipping slack notification: this pipeline succeeded"
  exit 0
fi


## Exit if this is a release job and it is still being retried.
if [[ $CI_JOB_NAME == "release" ]]; then
  echo "looking at the retries now"
  pipeline_info=$(curl -sL --header "PRIVATE-TOKEN: $GITLAB_API_TOKEN" "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/pipelines/${CI_PIPELINE_ID}/jobs?include_retried=true")
  num_retries=$(echo "$pipeline_info" | jq --exit-status --arg name "$CI_JOB_NAME" 'map(.name) | map(select(. == $name)) | length')
  if [ ${num_retries} -lt ${limit} ]; then
    echo "Skipping slack notification: retries ${num_retries} do not exceed limit ${limit}."
    exit 0
  fi
fi

notification_message="Pipeline ${PIPELINE_NAME} has failed.\n\nDetails: ${CI_PIPELINE_URL}"

echo "{\"notification_message\":${notification_message}\"}"
curl -X POST -H "Content-type: application/json" --data "{\"notification_message\":\"${notification_message}\"}" $SLACK_WEBHOOK_URL
