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

# This script is for Bluefield System-level Reset(BF-SLR). When the running DOCA version is 2.7 and above, we will use BF-SLR
# instead of power-cycle.
# In future release, this script will be replaced by DMS api.

# When running Arm shutdown from the host OS it is expected to get the message -E- Failed to send Register MRSI. This message should be ignored.
# This is a known issue refer https://docs.nvidia.com/doca/sdk/known+issues/index.html #3837255
shutdown_arm() {
  pci_address=$1
  output=$(mlxfwreset -d ${pci_address}.0 -l 1 -t 4 reset -y --sync 0)
  echo "shutdown arm:" >> /tmp/slr.log
  echo ${output} >> /tmp/slr.log
}

get_rshim_by_PCI() {
  pci_address=$1

  rshim=$(ls /dev | egrep 'rshim.*[0-9]+' | while read line ; do echo $(echo 'DISPLAY_LEVEL 1' > /dev/$line/misc && cat /dev/$line/misc | grep ${pci_address} | xargs -r echo $line | awk 'END {print $1}') ; done | tr -d '[:space:]')
  if [ $? -eq 0 ]; then
    echo ${rshim}
  else
    echo "Get rshim name failed"
    exit 1
  fi
}

# wait until ARM shutdown, expected get "INFO[BL31]: System Off" output from rshim misc
query_bf_state() {
  rshim=$1
  echo DISPLAY_LEVEL 2 > /dev/${rshim}/misc
  counter=1
  while [ $counter -le 30 ];
  do
    output=$(cat /dev/${rshim}/misc)
    echo ${output} >> /tmp/slr.log
    if [[ ${output} =~ "INFO[BL31]: System Off" ]]; then
      echo "System off"
      return
    fi
    sleep 5
    ((counter++))
  done
  exit 2
}

reboot_host() {
  pci_address=$1
  output=$(mlxfwreset -d ${pci_address}.0 -l 4 r -y)
  echo ${output}
}

pci=$1
cmd=$2

# This is a workaround for https://redmine.mellanox.com/issues/4035418
echo "ipmitool chassis power "$cmd > /usr/sbin/reboot
chmod +x /usr/sbin/reboot

shutdown_arm ${pci}
rshim=$(get_rshim_by_PCI ${pci})
echo "rshim: ${rshim}"
query_bf_state ${rshim}
reboot_host ${pci}
