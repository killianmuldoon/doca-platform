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

## Ignore if the pipeline is not from a schedule.
if [ "$CI_PIPELINE_SOURCE" != "schedule" ]; then
  exit 0
fi

## Exit if the job succeeded.
if [ "$CI_JOB_STATUS" == "success" ]; then
  exit 0
fi

curl -X POST -H "Content-type: application/json" --data "{\"pipeline_name\":\"${CI_JOB_NAME}\", \"pipeline_url\":\"${CI_PIPELINE_URL}\"}" $SLACK_WEBHOOK_URL
