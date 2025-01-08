#!/bin/bash

# Title: BFB Downloader
# Description: Downloads and sets up BFB files from a specified URL

# License
# 2024 NVIDIA CORPORATION & AFFILIATES

# Licensed under the Apache License, Version 2.0 (the License);
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an AS IS BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Set exit status on any error, and print the error message
set -euo pipefail
set -o errexit

# Define the function to handle cleanup
cleanup() {
    # Remove temporary files on exit or error
    [[ -n "$TEMP_FILE" ]] && rm -f "$TEMP_FILE"
    [[ -n "$opt_versions_output" ]] && rm -f "$opt_versions_output"
    [[ -n "$MD5_LOG_FILE" ]] && rm -f "$MD5_LOG_FILE"
}

# Define the function to handle downloading and setup of BFB files
download_and_setup_bfb_files() {
    # Define the temporary and final file paths
    local TEMP_FILE="$opt_base_dir/bfb-${opt_uid}"
    local FINAL_FILE="$opt_base_dir/$opt_file"
    local MD5_LOG_FILE="$opt_base_dir/${opt_file}.md5"

    # Check if the final file already exists
    if [[ ! -f "$FINAL_FILE" ]]; then
        echo "Starting download from $opt_url to $ TEMP_FILE..."
        curl -o "$TEMP_FILE" "$opt_url" || {
            echo "Failed to download $opt_url to $TEMP_FILE"
            exit 1
        }

        echo "Renaming $TEMP_FILE to $FINAL_FILE..."
        mv "$TEMP_FILE" "$FINAL_FILE" || {
            echo "Failed to rename $TEMP_FILE to $FINAL_FILE"
            exit 1
        }

        echo "Setting permissions for $FINAL_FILE..."
        chmod 0644 "$FINAL_FILE" || {
            echo "Failed to set permissions for $FINAL_FILE"
            exit 1
        }

        echo "Download and setup of $FINAL_FILE completed successfully."
    else
        echo "File $FINAL_FILE already exists. Skipping download."
    fi
}

# Define the function to handle MD5 checksum computation
compute_md5_checksum() {
    # Define the MD5 log file path
    local MD5_LOG_FILE="$opt_base_dir/${opt_file}.md5"

    # Check if the MD5 log file already exists
    if [[ ! -f "$MD5_LOG_FILE" ]]; then
        echo "Computing MD5 checksum for $FINAL_FILE..."
        md5sum "$FINAL_FILE" > "$MD5_LOG_FILE" || {
            echo "Failed to compute MD5 checksum for $FINAL_FILE"
            exit 1
        }

        echo "MD5 checksum saved to $MD5_LOG_FILE."
    else
        echo "MD5 checksum file $MD5_LOG_FILE already exists. Skipping checksum computation."
    fi
}

# Define the function to handle bfver execution with retry logic
run_bfver_with_retry() {
    # Define the maximum number of retries
    local MAX_RETRIES=5
    local RETRY_DELAY=1 # seconds

    # Define the output file path
    local OUTPUT_FILE="$opt_versions_output"

    echo "Running bfver on $FINAL_FILE and saving output to $OUTPUT_FILE..."

    # Set up a loop for retries
    for ((i=1; i<=MAX_RETRIES; i++)); do
        echo "Attempt $i of $MAX_RETRIES..."
        if timeout 30s bfver -f "$(realpath "$FINAL_FILE")" > "$OUTPUT_FILE" 2>&1; then
            echo "bfver completed successfully. Output saved to $OUTPUT_FILE."
            break # Exit loop on success
        fi

        # Check if the maximum number of retries is reached
        if [[ $i -ge MAX_RETRIES ]]; then
            echo "bfver failed after $MAX_RETRIES attempts. Exiting with error."
            exit 1
        fi

        # Handle retry logic
        echo "bfver failed on attempt $i."
        echo "Retrying in ${RETRY_DELAY} seconds..."
        sleep "$RETRY_DELAY"
    done
}

# Set up the trap to handle cleanup
trap cleanup INT TERM ILL KILL FPE SEGV ALRM ERR EXIT

# Define the command-line arguments
declare -A options
options[--url]=""
options[--file]=""
options[--uid]=""
options[--base-dir]=""
options[--versions-output]=""

# Parse the command-line options
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --url=*)
            options[--url]=${1#*=}
            ;;
        --url)
            options[--url]=$2
            shift
            ;;
        --file=*|-f=*)
            options[--file]=${1#*=}
            ;;
        --file|-f)
            options[--file]=$2
            shift
            ;;
        --uid=*)
            options[--uid]=${1#*=}
            ;;
        --uid)
            options[--uid]=$2
            shift
            ;;
        --base-dir=*|-d=*)
            options[--base-dir]=${1#*=}
            ;;
        --base-dir|-d)
            options[--base-dir]=$2
            shift
            ;;
        --versions-output=*|-o=*)
            options[--versions-output]=${1#*=}
            ;;
        --versions-output|-o)
            options[--versions-output]=$2
            shift
            ;;
        *)
            cat <<EOF >&2
Unrecognized argument: $1
Valid arguments:
  --url                     <url>     BFB Sources location (required)
  --file | -f               <name>    BFB File name (required)
  --uid                     <id>      BFB Unique ID (required)
  --base-dir | -d           <path>    BFB file location (required)
  --versions-output | -o    <path>    Path to save BFB versions output (required)
EOF
            exit 1
            ;;
    esac
    shift
done

# Validate the required options
declare -A required_options
required_options[--url]=1
required_options[--file]=1
required_options[--uid]=1
required_options[--base-dir]=1
required_options[--versions-output]=1

# Check if all required options are present
for option in "${!required_options[@]}"; do
    if [[ -z "${options[$option]}" ]]; then
        echo "Error: Missing required arguments." >&2
        cat <<EOF >&2
Usage:
  bfbdownloader.sh --url=<url> --file=<name> --uid=<id> --base-dir=<path> --versions-output=<path>
  bfbdownloader.sh --url <url> --file <name> --uid <id> --base-dir <path> --versions-output <path>

Example:
  bfbdownloader.sh --url="http://example.com/file.bfb" --file="file.bfb" --uid="12345" --base-dir="/bfb" --versions-output="/bfb/versions.txt"
EOF
        exit 1
    fi
done

# Set the options to variables
declare -g opt_url="${options[--url]}"
declare -g opt_file="${options[--file]}"
declare -g opt_uid="${options[--uid]}"
declare -g opt_base_dir="${options[--base-dir]}"
declare -g opt_versions_output="${options[--versions-output]}"

# Ensure the base directory exists
declare -g opt_base_dir
mkdir -p "$opt_base_dir"

# Define the file paths
opt_base_dir="$opt_base_dir"
FINAL_FILE="$opt_base_dir/$opt_file"
MD5_LOG_FILE="$opt_base_dir/${opt_file}.md5"

# Call the functions
download_and_setup_bfb_files
compute_md5_checksum
run_bfver_with_retry

echo "Script completed successfully."

# Remove the trap after successful execution
trap - INT TERM ILL KILL FPE SEGV ALRM ERR EXIT
exit 0