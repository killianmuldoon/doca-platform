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

# This script is a workaround for the issue where the mlx5_core driver enters an error state and the device stops working.
# The script listens to the kernel logs for the error message and reinitializes the device by unbinding and binding the driver.
#
# It is only a workaround for our CI environments where the system is not rebooted during the e2e tests.
# Without a reboot, the device will not be reinitialized and the error state persists.
# The script is only a workaround for our CI environment and should not be used in production.

target_message="mlx5_fw_fatal_reporter_err_work:.*Driver is in error state. Unloading"

# Call $@, send it to the background and get pid to be able to kill it later.
# Gitlab is not able to kill the process group, so we need to kill the process by pid.
# There have been issues in the past and it should be fixed already, but it is not working for us.
# For reference:
# - https://gitlab.com/gitlab-org/gitlab-runner/-/issues/3228
# - https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26499
# Tested with running the background process in the same subshell and in a separate subshell of the .gitlab-ci.yaml.
"$@" &
pid=$!

while true; do
  # Exit if the process is not running anymore.
  if ! kill -0 $pid; then
    echo "Process $pid is not running anymore. Exiting."
    exit 0
  fi

  devices=$(journalctl --since "-10s" -k -g "$target_message" -o cat | sed -r 's/.* (0000:..:..\..): .*/\1/')
  for device in $devices; do
    echo "Reloading device $device"
    echo "$device" > /sys/bus/pci/drivers/mlx5_core/unbind
    echo "$device" > /sys/bus/pci/drivers/mlx5_core/bind
  done
  sleep 1
done
