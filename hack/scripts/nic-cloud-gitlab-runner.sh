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


SETUP_DESCRIPTION="b2b x86-64 bf2"
TAG="dpf/dpu"
MAX_VM_RUNNERS="${MAX_VM_RUNNERS:-2}"
TARGET_HOUR="${TARGET_HOUR:-4}"

check_and_unregister_offline_runners() {
    TAG="$1"

    # Get the list of runners with the specified tag
    response=$(curl --silent --header "PRIVATE-TOKEN: $GITLAB_REGISTRY_TOKEN" "https://gitlab-master.nvidia.com/api/v4/projects/112105/runners?tag_list[]=$TAG")

    # Check if the curl command succeeded
    if [[ $? -ne 0 ]]; then
        echo "Failed to fetch runners. Exiting..."
        return 1
    fi

    # Parse the response to check if any runner is offline
    offline_runners=$(echo "$response" | jq '[.[] | select(.status == "offline")]')

    # Check if there are any offline runners
    if [[ -z "$offline_runners" ]]; then
        echo "No offline runners found with tag '$TAG'."
    else
        echo "Offline runners found:"
        echo "$offline_runners"

        # Loop through offline runners and unregister them
        echo "$offline_runners" | jq -c '.[]' | while read -r runner; do
            echo $runner
            runner_id=$(echo "$runner" | jq -r '.id')
            runner_desc=$(echo "$runner" | jq -r '.description')

            echo "Unregistering offline runner: $runner_desc (ID: $runner_id)"

            # Unregister the offline runner
            curl --silent --header "PRIVATE-TOKEN: $GITLAB_REGISTRY_TOKEN" --request DELETE "https://gitlab-master.nvidia.com/api/v4/runners/$runner_id"

            if [[ $? -eq 0 ]]; then
                echo "Successfully unregistered runner: $runner_desc (ID: $runner_id)"
            else
                echo "Failed to unregister runner: $runner_desc (ID: $runner_id)"
            fi
        done
    fi

}

check_and_unregister_offline_runners "$TAG"

set -e

# After unregistering offline runners, make a new API call to get the updated list
updated_response=$(curl --silent --header "PRIVATE-TOKEN: $GITLAB_REGISTRY_TOKEN" "https://gitlab-master.nvidia.com/api/v4/projects/112105/runners?tag_list[]=$TAG")

# Count the number of runners with the specified tag again
runner_count_after=$(echo "$updated_response" | jq '. | length')

# Check if there are more than 2 runners
if [[ $runner_count_after -gt $MAX_VM_RUNNERS ]]; then # make that number a variable
    echo "There are more than $MAX_VM_RUNNERS runners with tag '$TAG'. Script exiting"
    exit 0
fi

# Get the current hour
current_hour=$(date +%-H)

target_hour=$TARGET_HOUR

# Calculate how many hours left until target hour.
if [ "$current_hour" -lt "$target_hour" ]; then
    # If it's before target time, calculate the difference
    hours_left=$((target_hour - current_hour))
else
    # If it's after target time, calculate the difference until the next day's target time
    hours_left=$((24 - current_hour + target_hour))
fi

# Ensure a minimum of 2 hours
if [ "$hours_left" -lt 2 ]; then
    hours_left=2
fi

echo  "Ordering $SETUP_DESCRIPTION VMs from NIC Cloud for $hours_left hours."

# Order a machine
RESPONSE=$(curl --silent -X 'POST' \
  'http://linux-cloud.mellanox.com/cloud-api/v1/order' \
  -H 'accept: application/json' \
  -H "Authorization: Bearer $NIC_CLOUD_AUTH_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{
  \"user\": \"$NIC_CLOUD_USERNAME\",
  \"description\": \"$SETUP_DESCRIPTION\",
  \"hca_protocol\": \"ETH\",
  \"image\": \"linux/inbox-ubuntu24.04-x86_64\",
  \"fw_version\": \"last_stable\",
  \"mlxconfig\": \"SRIOV_EN=1 NUM_OF_VFS=46\",
  \"time\": $hours_left
}")

# Extract job_id and status
JOB_ID=$(echo "$RESPONSE" | jq -r '.job_id')
STATUS=$(echo "$RESPONSE" | jq -r '.status')

echo "Session ID: $JOB_ID"
echo "Status: $STATUS"

# Check if the status is not "success"
if [ "$STATUS" != "success" ]; then
  echo "Error: Status is $STATUS. Exiting..." >&2
  exit 1
fi

# Loop to check order status
echo "Waiting for NIC cloud machine to provision (Can be 15+ minutes)..."
while true; do
  ORDER_STATUS_RESPONSE=$(curl  -s -X 'GET' \
    "http://linux-cloud.mellanox.com/cloud-api/v1/get_order_status/${JOB_ID}" \
    -H 'accept: application/json' \
    -H "Authorization: Bearer $NIC_CLOUD_AUTH_TOKEN")


  # Extract job_summary and current status
  JOB_SUMMARY=$(echo "$ORDER_STATUS_RESPONSE" | jq -r '.job_summary')
  CURRENT_STATUS=$(echo "$ORDER_STATUS_RESPONSE" | jq -r '.status')

  echo -n "."

  # Check if job_summary is 'active'
  if [[ "$JOB_SUMMARY" == "active" ]]; then
    echo -e " \n---- Machine is active!----\n"
    break
  fi

  # Check for failure status
  if [[ "$CURRENT_STATUS" != "success" ]]; then
    echo -e "\nError: Order failed, Status is $CURRENT_STATUS. Exiting..." >&2
    echo -e "Response from Server: $ORDER_STATUS_RESPONSE"  >&2
    exit 1
  fi

  # Wait for a few seconds before the next check
  sleep 30
done

# Extract the host ip field from ORDER_STATUS_RESPONSE as HOST_IP
IP_1=$(echo "$ORDER_STATUS_RESPONSE" | jq -r '.setup_info.players[0].ip')
IP_2=$(echo "$ORDER_STATUS_RESPONSE" | jq -r '.setup_info.players[1].ip')

echo -e "---- Host_1 IP: $IP_1 ----"
echo -e "---- Host_2 IP: $IP_2 ----"
echo -e "---- Machine is active and ready to use ----"


# Create the file `gitlab_env.sh` locally
echo "export GITLAB_RUNNER_USER=root" > gitlab_env.sh
echo "export REGISTER_DOCKER_RUNNER=false" >> gitlab_env.sh
echo "export INSTALL_DOCKER=false" >> gitlab_env.sh
echo "export SHELL_TAG_LIST=$TAG" >> gitlab_env.sh
echo "export GITLAB_RUNNER_API_TOKEN=$GITLAB_RUNNER_TOKEN" >> gitlab_env.sh
echo "export GITLAB_REGISTRY_TOKEN=$GITLAB_REGISTRY_TOKEN" >> gitlab_env.sh
chmod +777 gitlab_env.sh

# Copy the environment file to the remote server
sshpass -p "$VM_PASSWORD" scp -o StrictHostKeyChecking=no gitlab_env.sh root@"$IP_1":/root/gitlab_env.sh
sshpass -p "$VM_PASSWORD" scp -o StrictHostKeyChecking=no gitlab_env.sh root@"$IP_2":/root/gitlab_env.sh

echo -e "------------------ Initiating gitlab-runner on $IP_1 --------------------"

sshpass -p "$VM_PASSWORD" ssh -o StrictHostKeyChecking=no root@"$IP_1" << 'EOF'
source /root/gitlab_env.sh
curl --header "PRIVATE-TOKEN: $GITLAB_REGISTRY_TOKEN" "https://gitlab-master.nvidia.com/api/v4/projects/112105/repository/files/hack%2Fscripts%2Fgitlab_runner.sh/raw?ref=main" > gitlab-runner.sh
chmod +777 gitlab-runner.sh
./gitlab-runner.sh
EOF

echo -e "------------------ Initiating gitlab-runner on $IP_2 --------------------"

sshpass -p "$VM_PASSWORD" ssh -o StrictHostKeyChecking=no root@"$IP_2" << 'EOF'
source /root/gitlab_env.sh
curl --header "PRIVATE-TOKEN: $GITLAB_REGISTRY_TOKEN" "https://gitlab-master.nvidia.com/api/v4/projects/112105/repository/files/hack%2Fscripts%2Fgitlab_runner.sh/raw?ref=main" > gitlab-runner.sh
chmod +777 gitlab-runner.sh
./gitlab-runner.sh
EOF
