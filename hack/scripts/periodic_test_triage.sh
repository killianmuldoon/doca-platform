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

## This script looks at a number of previous CI runs - NUM_TESTS_CONSIDERED - and notifies the slack channel if a limit of failures - NOTIFICATION_FAILURE_THRESHOLD - is reached.
## This script needs to have curl and jq available.

set -o nounset
set -o pipefail
set -o errexit

GITLAB_API_TOKEN="${GITLAB_API_TOKEN:-""}"
NUM_TESTS_CONSIDERED="${NUM_TESTS_CONSIDERED:-24}"
NOTIFICATION_FAILURE_THRESHOLD="${NOTIFICATION_FAILURE_THRESHOLD:-3}"
TRIAGE_PIPELINE_NAME="${TRIAGE_PIPELINE_NAME:-""}"

# results pulls the last 100 results for pipelines from the gitlab api.
results=$(curl --request GET --header "PRIVATE-TOKEN: ${GITLAB_API_TOKEN}" "https://gitlab-master.nvidia.com/api/v4/projects/doca-platform-foundation%2Fdoca-platform-foundation/pipelines?per_page=100")

# filtered_results are the most recent $NUM_TEST_CONSIDERED pipeline runs that have "Periodic unit test" as their name.
filtered_results=$(echo ${results} | jq -c --argjson NUM_TESTS_CONSIDERED "${NUM_TESTS_CONSIDERED}" --arg TRIAGE_PIPELINE_NAME "${TRIAGE_PIPELINE_NAME}" '[.[]  | select ( .name == $TRIAGE_PIPELINE_NAME)] | limit($NUM_TESTS_CONSIDERED;.[])')

echo $filtered_results

# failed_runs filters the runs that failed from all runs.
failed_runs=$(echo ${filtered_results} | jq  '. |  select ( .status == "failed" ) ')

echo $failed_runs

# number_failed counts the number of runs that failed in filtered_results.
number_failed=$(echo ${failed_runs} | jq '.status' | jq -s  '. | length')

echo $number_failed

# Failed links results in an array of links to failed jobs.
failed_links=$(echo ${failed_runs} | jq -r ' .web_url ' )

echo $failed_links

# If the number of failures is below the threshold exit.
if ((number_failed < ${NOTIFICATION_FAILURE_THRESHOLD})); then
  exit 0
fi


notification_message="Periodic unit tests have failed more than ${NOTIFICATION_FAILURE_THRESHOLD} times in the last ${NUM_TESTS_CONSIDERED} runs.\n\nFailed runs:\n${failed_links}"
echo $notification_message

curl -X POST -H "Content-type: application/json" --data "{\"notification_message\":\"${notification_message}\"}" $SLACK_WEBHOOK_URL
