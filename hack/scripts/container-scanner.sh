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

set -euo pipefail

CONTAINER_ARCHIVE=/dist/archive.tar
PLATFORM=linux/amd64
AUTHORIZATION=$(echo -n $SSA_CLIENT_ID:$SSA_CLIENT_SECRET | base64 -w0)
SCRIPT_PATH="$(dirname $(realpath $0))"

# Ensure were in the right directory of the project
pushd "${SCRIPT_PATH}/../.."

# Get SSA Bearer Token (TTL 60 minutes)
SSA_TOKEN=$(curl --silent --location --request POST \
  "https://${PSS_SSA_ID}.ssa.nvidia.com/token" \
  --header "Authorization: Basic ${AUTHORIZATION}" \
  --header "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=client_credentials" \
  --data-urlencode "scope=nspect.updatereport nspect.verify scan.anchore" | jq -r '.access_token')

# Directory for all scan-related files
scans_dir="/tmp/scans"

# Directories for archives, results, and policy inside the scans directory
archive_dir="${scans_dir}/archives"
results_dir="${scans_dir}/results"
policy_dir="${scans_dir}/policy"

# Create the scans directory and subdirectories if they don't exist
mkdir -p $archive_dir $results_dir $policy_dir

# Download the scan policy file
policy_url="https://gitlab-master.nvidia.com/pstooling/container-scan-policy-documents/-/raw/main/prodsec_approved_release_policy.json"
policy_file="${policy_dir}/prodsec_approved_release_policy.json"

curl --silent -o $policy_file $policy_url

if [[ ! -f "$policy_file" ]]; then
  echo "[ERROR] Failed to download scan policy file."
  exit 1
fi

# Store the policy file path in a variable
CONTAINER_SCAN_POLICY=$policy_file

# Read image names from defaults.yaml
if [[ ! -f "internal/release/manifests/defaults.yaml" ]]; then
  echo "[ERROR] defaults.yaml file not found."
  exit 1
fi

images=$(awk '/Image:/{print $2}' internal/release/manifests/defaults.yaml)

if [[ -z "$images" ]]; then
  echo "[ERROR] No images found in defaults.yaml."
  exit 1
fi

# Array to collect return codes from pulse-cli
return_codes=()

# Iterate over each image
for image in $images; do

  # Check if $REGISTRY is a substring of the image
  if [[ "$image" != *"$REGISTRY"* ]]; then
    echo "Skipping $image (registry doesn't match)"
    continue
  fi

  # Pull the image from repo
  docker pull "${image}"

  # Capture the return code of the docker pull command
  pull_return_code=$?

  # Check if the docker pull was successful
  if [[ $pull_return_code -ne 0 ]]; then
    echo "[ERROR] Failed to pull image ${image} with return code $pull_return_code"
    exit 1
  fi

  # Extract just the name and tag of the image for the archive and log file
  image_name=$(basename $image)
  archive_name="${image_name//:/_}.tar"

  # Create a directory for each image
  mkdir -p ${results_dir}/${image_name}

  # Save the Docker image as a tar archive in the archive directory
  docker save $image > "${archive_dir}/${archive_name}"

  # Run PulseScanner docker with the policy file and create a scan log file in the results directory
  echo "--------- Running PulseScanner for $image_name ---------"

  # Set manual error handling
  set +e

  docker run \
    --rm -i \
    -e NSPECT_ID=$NSPECT_ID \
    -e SSA_TOKEN=$SSA_TOKEN \
    -e CONTAINER_ARCHIVE="${archive_dir}/${archive_name}" \
    -v ${scans_dir}:${scans_dir} \
    -w ${scans_dir} \
    --user $(id -u):$(id -g) \
    gitlab-master.nvidia.com:5005/pstooling/pulse-group/pulse-container-scanner/pulse-cli:stable \
    /bin/sh -c "pulse-cli -n $NSPECT_ID --ssa $SSA_TOKEN \
      scan \
      --output-dir=${results_dir}/${image_name} \
      --platform=$PLATFORM \
      -i ${archive_dir}/${archive_name} \
      -p $CONTAINER_SCAN_POLICY \
      -o" | tee -a "${results_dir}/${image_name}/scan.log"


  # Capture the return code of the pulse-cli command
  return_code=$?

  # Set back manual error handling
  set -e

  # Add the return code to the array
  return_codes+=($return_code)

  # Check if the command was successful
  if [[ $return_code -ne 0 ]]; then
    echo "PulseScanner for $image_name failed with return code $return_code"
  else
    echo "PulseScanner for $image_name completed successfully"
  fi

done

# Check if any return code in the array is non-zero
for code in "${return_codes[@]}"; do
  if [[ $code -ne 0 ]]; then
    echo "One or more PulseScanner commands failed."
    exit 1
  fi
done

echo "All PulseScanner commands completed successfully."
exit 0