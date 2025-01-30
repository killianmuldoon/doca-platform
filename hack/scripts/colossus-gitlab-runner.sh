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

set -eou pipefail

: ${COLOSSUS_CLIENT_ID:?env not set}
: ${COLOSSUS_CLIENT_SECRET:?env not set}
: ${GITLAB_RUNNER_TOKEN:?env not set}
: ${GITLAB_REGISTRY_TOKEN:?env not set}
: ${LEASE_POOL_SIZE:=10}
: ${LEASES_PER_JOB:=2}
: ${REGISTER_DOCKER_RUNNER:="true"}
: ${INSTALL_DOCKER:="true"}
: ${SHELL_TAG_LIST:="type/shell,e2e,release,colossus"}
: ${DOCKER_TAG_LIST:="type/docker,colossus"}

check_prerequisites() {

    # Install required packages
    if ! command -v sshpass &> /dev/null || ! command -v ssh &> /dev/null || ! command -v python3 &> /dev/null || ! command -v pip3 &> /dev/null; then
        echo "Installing required packages..."
        apk add --no-cache sshpass openssh python3 py3-pip
    else
        echo "Required packages are already installed"
    fi
    # Download colossus CLI if not already installed
    if ! command -v colossus &> /dev/null
    then
        echo "colossus could not be found, installing..."
        pip3 install --upgrade -q pip --break-system-packages
        pip3 install --index-url=https://urm.nvidia.com/artifactory/api/pypi/sw-colossus-pypi/simple -q colossus-cli --break-system-packages
        echo "colossus cli version = $(colossus version)"
        colossus login --method ssa --client-id "$COLOSSUS_CLIENT_ID" --client-secret "$COLOSSUS_CLIENT_SECRET"
    else
        echo "colossus CLI is already installed"
    fi

    # make sure host has jq package installed.
    if ! command -v jq &> /dev/null
    then
        echo -e "\e[31mjq could not be found, please install jq\e[0m"
        exit 1
    fi
}

check_gitlab_runners_count() {
    local tag=$1
    local runners_count

    runners_count=$(curl --silent --header "PRIVATE-TOKEN: $GITLAB_REGISTRY_TOKEN" "https://gitlab-master.nvidia.com/api/v4/projects/112105/runners?tag_list[]=$tag" | jq '[.[] | select(.active == true)] | length')

    if [ $? -ne 0 ]; then
        echo -e "\e[31mFailed to retrieve the number of GitLab runners.\e[0m"
        echo "Output: $runners_count"
        exit 1
    fi

    echo "$runners_count"
}

release_unactive_runners_by_tag() {
    local tag=$1
    local runners

    runners=$(curl --silent --header "PRIVATE-TOKEN: $GITLAB_REGISTRY_TOKEN" "https://gitlab-master.nvidia.com/api/v4/projects/112105/runners?tag_list[]=$tag" | jq -r '.[] | select(.status == "offline") | .id')

    if [ -z "$runners" ]; then
        echo "No unactive runners found with tag '$tag'."
        return
    fi

    for runner_id in $runners; do
        echo "Releasing unactive runner with ID: $runner_id"
        curl --silent --request DELETE --header "PRIVATE-TOKEN: $GITLAB_REGISTRY_TOKEN" "https://gitlab-master.nvidia.com/api/v4/runners/$runner_id"
        if [ $? -ne 0 ]; then
            echo -e "\e[31mFailed to release runner with ID: $runner_id\e[0m"
        else
            echo -e "\e[32mSuccessfully released runner with ID: $runner_id\e[0m"
        fi
    done
}

check_prerequisites
release_unactive_runners_by_tag "$DOCKER_TAG_LIST"
release_unactive_runners_by_tag "$SHELL_TAG_LIST"
docker_runners_count=$(check_gitlab_runners_count "$DOCKER_TAG_LIST")
shell_runners_count=$(check_gitlab_runners_count "$SHELL_TAG_LIST")

if [ "$docker_runners_count" -eq "$LEASE_POOL_SIZE" ] && [ "$shell_runners_count" -eq "$LEASE_POOL_SIZE" ]; then
    echo "shell & docker runners on gitlab equals the required amount. Exiting..."
    exit 0
fi

num_existing_leases=$(colossus bm lease list -c --json | jq '. | length') || { echo -e "\e[31mFailed to retrieve the number of existing leases.\e[0m"; exit 1; }
num_existing_leases=$((num_existing_leases)) # Convert to integer
if [ "$num_existing_leases" -eq 0 ]; then
    echo "Failed to retrieve the number of existing leases or no leases found."
else
    echo -e "\e[32mNumber of existing leases: $num_existing_leases\e[0m"
fi

# Create the file `gitlab_runner_env.sh` locally
echo "Creating the environment file for the runner script..."
runner_env_file='gitlab_runner_env.sh'
cat <<EOF > "$runner_env_file"
export REGISTER_DOCKER_RUNNER=$REGISTER_DOCKER_RUNNER
export INSTALL_DOCKER=$INSTALL_DOCKER
export SHELL_TAG_LIST=$SHELL_TAG_LIST
export DOCKER_TAG_LIST=$DOCKER_TAG_LIST
export GITLAB_RUNNER_API_TOKEN=$GITLAB_RUNNER_TOKEN
export GITLAB_REGISTRY_TOKEN=$GITLAB_REGISTRY_TOKEN
EOF
chmod 777 "$runner_env_file"
echo "Environment file created: $runner_env_file"


check_lease_ready() {
    local lease_id=$1
    local status=""

    echo -e "Checking if lease \e[34m$lease_id\e[0m is ready... "
    while [ "$status" != "RESERVED" ]; do
        status=$(colossus bm lease list --lease-id "$lease_id" --json | jq -r '.[0].entity_details.status' | tr -d '\r\n')
        echo -n "."
        if [ "$status" == "DELETING" ]; then
            echo -e "\n\e[31mLease $lease_id is being deleted. Exiting...\e[0m"
            return 1
        fi
        [ "$status" != "RESERVED" ] && sleep 30
    done

    echo -e "\n\e[32mLease $lease_id is ready.\e[0m"
}

get_lease_credentials() {
    local lease_id=$1
    local json_output=""
    local failure_message="Secrets not yet configured for the Leased system"
    local spinner="/-\|"

    while true; do
        for i in $(seq 0 3); do
            printf "\rWaiting for lease credentials to be ready... ${spinner:$i:1}"
            json_output=$(colossus bm lease list --lease-id "$lease_id" --json --show-creds)
            failure_message=$(echo "$json_output" | jq -r '.[0].lease_secrets.failure_message // empty')

            if [ -z "$failure_message" ]; then
                lease_username=$(echo "$json_output" | jq -r '.[0].lease_secrets.local_accountname')
                lease_password=$(echo "$json_output" | jq -r '.[0].lease_secrets.local_accountpassword')
                lease_ip=$(echo "$json_output" | jq -r '.[0].entity_details.ipAddress')
                printf "\n\e[32mLease credentials are ready.\e[0m\n"
                return
            fi
        done
        sleep 5
    done
}

run_runner_script_on_lease() {
    local lease_username=$1
    local lease_password=$2
    local lease_ip=$3

    echo -e "\e[32m==================== Running runner script on lease with IP $lease_ip ====================\e[0m"

    # Copy the environment file to the leased machine
    sshpass -p "$lease_password" scp -q -o StrictHostKeyChecking=no "$runner_env_file" "$lease_username@$lease_ip:/tmp/"

    # Execute the commands on the leased machine
    sshpass -p "$lease_password" ssh -o StrictHostKeyChecking=no "$lease_username@$lease_ip" << EOF
        source /tmp/$runner_env_file
        curl --header "PRIVATE-TOKEN: $GITLAB_REGISTRY_TOKEN" "https://gitlab-master.nvidia.com/api/v4/projects/112105/repository/files/hack%2Fscripts%2Fgitlab_runner.sh/raw?ref=main" > gitlab-runner.sh
        sudo chmod 777 /etc/security/limits.conf
        sudo chmod 777 gitlab-runner.sh
        ./gitlab-runner.sh
EOF

    echo -e "\e[32m==================== Finished running runner script on lease with IP $lease_ip ====================\e[0m"
}

lease_bare_metal() {
    local attempt_counter=0
    local json_output=""
    local lease_id=""
    set +eou pipefail

    while true; do
        json_output=$(colossus bm lease create \
            --start-datetime=now \
            --duration=7d \
            --provision-mode=CLEAN \
            --os="ubuntu-24.04-x86_64-standard-uefi" \
            --lease-justification="Doca Platform Framework CI" \
            --query \
                status=available \
                status=deleting \
                status=recycled \
                status=pre_provisioned \
                gpuCount=0 \
                memory=64 \
            --json)

        if [ $? -ne 0 ]; then
            echo -e "\e[31mFailed to create lease. Output:\n$json_output\e[0m"
            attempt_counter=$((attempt_counter+1))
            if [ $attempt_counter -ge 3 ]; then
                echo -e "\e[31mFailed to create lease after 3 attempts. Exiting...\e[0m"
                exit 1
            fi
            sleep 30
        else
            lease_id=$(echo "$json_output" | jq -r '.lease_id')
            echo -e "Lease ID: \e[34m$lease_id\e[0m"
            break
        fi
    done
    set -eou pipefail

    lease_ids=("$lease_id" "${lease_ids[@]}")
}

lease_ids=()

for ((i=0; i<LEASES_PER_JOB; i++)); do
    lease_bare_metal
done

for lease_id in "${lease_ids[@]}"; do
    if ! check_lease_ready "$lease_id"; then
        echo -e "\e[31mLease $lease_id failed to provision. Skipping to the next lease.\e[0m"
        continue
    fi
    get_lease_credentials "$lease_id"
    run_runner_script_on_lease "$lease_username" "$lease_password" "$lease_ip"
done

docker_runners_count=$(check_gitlab_runners_count "$DOCKER_TAG_LIST")
shell_runners_count=$(check_gitlab_runners_count "$SHELL_TAG_LIST")
num_existing_leases=$(colossus bm lease list -c --json | jq '. | length') || { echo -e "\e[31mFailed to retrieve the number of existing leases.\e[0m"; exit 1; }


if [ "$docker_runners_count" -eq "$LEASE_POOL_SIZE" ] && [ "$shell_runners_count" -eq "$LEASE_POOL_SIZE" ] && [ "$num_existing_leases" -eq "$LEASE_POOL_SIZE" ]; then
    echo -e "\e[32mLease pool is full (${LEASE_POOL_SIZE} setups). Exiting...\e[0m"
else
    echo -e "\e[31mDesired colossus lease pool not full:\n  want: $LEASE_POOL_SIZE\n  actual: $num_existing_leases\e[0m"
    echo -e "\e[31mDesired runner pool not full:\n  Docker runners:\n   want: $LEASE_POOL_SIZE\n   actual: $docker_runners_count\n  Shell runners:\n   want: $LEASE_POOL_SIZE\n   actual: $shell_runners_count\e[0m"
fi
