#!/bin/bash

# Title: BFB Downloader
# Description: Downloads and sets up BFB files from a specified URL

# License
# 2025 NVIDIA CORPORATION & AFFILIATES

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

# Log function
log() {
    # Add a timestamp to the log message
    echo "[bfbdownloader] $1"
}

# Error function
error() {
    # Send error messages to stderr instead of stdout
    echo "[bfbdownloader] Error: $1" >&2
    cleanup
    exit 1
}

# Define the function to handle cleanup
cleanup() {
    # Remove temporary files on exit or error
    local temp_file="$opt_base_dir/bfb-${opt_uid}"
    local md5_log_file="$opt_base_dir/${opt_file}.md5"
    local output_file="$opt_versions_output"
    if [ -n "$temp_file" ]; then
        rm -f "$temp_file"
    fi
    if [ -n "$output_file" ]; then
        rm -f "$output_file"
    fi
    if [ -n "$md5_log_file" ]; then
        rm -f "$md5_log_file"
    fi
}

# Define the function to handle downloading and setup of BFB files
download_and_setup_bfb_files() {
    # Define the temporary and final file paths
    local temp_file="$opt_base_dir/bfb-${opt_uid}"
    local final_file="$opt_base_dir/$opt_file"
    local md5_log_file="$opt_base_dir/${opt_file}.md5"

    # Check if the final file already exists
    if [ ! -f "$final_file" ]; then
        log "Starting download from $opt_url to $temp_file..."
        # Validate the URL
        if ! curl -s -f -I "$opt_url" > /dev/null; then
            error "Invalid URL: $opt_url"
        fi
        # Download the file
        curl -o "$temp_file" "$opt_url" || {
            error "Failed to download $opt_url to $temp_file"
        }
        log "Renaming $temp_file to $final_file..."
        mv "$temp_file" "$final_file" || {
            error "Failed to rename $temp_file to $final_file"
        }
        log "Setting permissions for $final_file..."
        chmod 0644 "$final_file" || {
            error "Failed to set permissions for $final_file"
        }
        log "Download and setup of $final_file completed successfully."
    else
        log "File $final_file already exists. Skipping download."
    fi
}

# Define the function to handle MD5 checksum computation
compute_md5_checksum() {
    # Define the MD5 log file path
    local md5_log_file="$opt_base_dir/${opt_file}.md5"
    # Check if the MD5 log file already exists
    if [ ! -f "$md5_log_file" ]; then
        log "Computing MD5 checksum for $final_file..."
        md5sum "$final_file" > "$md5_log_file" || {
            error "Failed to compute MD5 checksum for $final_file"
        }
        log "MD5 checksum saved to $md5_log_file."
    else
        log "MD5 checksum file $md5_log_file already exists. Skipping checksum computation."
    fi
}

# Define the function to handle bfver execution with retry logic
run_bfver_with_retry() {
    # Define the maximum number of retries
    local max_retries=5
    local retry_delay=1 # seconds
    # Define the output file path
    local output_file="$opt_versions_output"
    log "Running bfver on $final_file and saving output to $output_file..."
    # Set up a loop for retries
    for ((i=1; i<=max_retries; i++)); do
        log "Attempt $i of $max_retries..."
        if timeout 30s bfver -f "$(realpath "$final_file")" > "$output_file" 2>&1; then
            # Check if the error message is file not found
            if grep -q "bfver: warn: $(realpath "$final_file") does not exist, skipping" "$output_file"; then
                error "bfver failed with error: file not found, skipping"
            fi
            log "bfver completed successfully. Output saved to $output_file."
            break # Exit loop on success
        else
            # Check if the error message is file not found
            if grep -q "bfver: warn: $(realpath "$final_file") does not exist, skipping" "$output_file"; then
                error "bfver failed with error: file not found, skipping"
            fi
            error "bfver failed with an unknown error"
        fi
        # Check if the maximum number of retries is reached
        if ((i >= max_retries)); then
            error "bfver failed after $max_retries attempts. Exiting with error."
        fi
        # Handle retry logic
        log "bfver failed on attempt $i."
        log "Retrying in ${retry_delay} seconds..."
        sleep "$retry_delay"
    done
}

# Define the command-line arguments
declare -A options
options[--url]=""
options[--file]=""
options[--uid]=""
options[--base-dir]=""
options[--versions-output]=""

# Parse the command-line options
while [[ $# -gt 0 ]]; do
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
    if [ -z "${options[$option]}" ]; then
        error "Missing required arguments."
    fi
done

# Set the options to variables
declare -g opt_url="${options[--url]}"
declare -g opt_file="${options[--file]}"
declare -g opt_uid="${options[--uid]}"
declare -g opt_base_dir="${options[--base-dir]}"
declare -g opt_versions_output="${options[--versions-output]}"

# Ensure the base directory exists
mkdir -p "$opt_base_dir"

# Define the file paths
final_file="$opt_base_dir/$opt_file"
md5_log_file="$opt_base_dir/${opt_file}.md5"

# Setup trap for cleanup
trap cleanup INT TERM ILL KILL FPE SEGV ALRM ERR EXIT

# Call the functions
download_and_setup_bfb_files
compute_md5_checksum
run_bfver_with_retry
log "Script completed successfully."

# Remove the trap after successful execution
trap - INT TERM ILL KILL FPE SEGV ALRM ERR EXIT
exit 0