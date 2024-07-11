#!/usr/bin/env bash

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

set -Eeu
trap cleanup SIGHUP SIGINT SIGTERM ERR

cleanup() {
  die "HBN initial config for DPF/SFC failed."
}

TIMESTAMP=$(date "+%Y-%m-%d %H:%M:%S")
msg() {
  echo 2>&1 -e "[$TIMESTAMP]   ${1-}"
}

die() {
  local msg=$1
  local code=${2-1} # default exit status 1
  msg "Error: $msg"
  exit "$code"
}

HBN_DEFAULT_FILES_PATH=${HBN_DEFAULT_FILES_PATH:-/hbn_files}
HBN_CONFIG_DIR=${HBN_CONFIG_DIR:-/host/var/lib/hbn}
HBN_LOG_DIR=${HBN_LOG_DIR:-/host/var/log/doca/hbn}


#Set system configuration on host level
write_sysctl_host() {
    msg "seting $1 in host namespace"
    msg "   chroot /host nsenter -t 1 -n sysctl -w $1"
    chroot /host nsenter -t 1 -n sysctl -w "$1"
}

configure_host_sysctl() {
    msg "configure sysctl on host"

    write_sysctl_host net.ipv4.neigh.default.base_reachable_time_ms=60000
    write_sysctl_host net.ipv6.neigh.default.base_reachable_time_ms=60000
    msg "configure sysctl on host complete"
}

#Set system configuration on container level

write_sysctl_con() {
    msg "seting $1 in container namespace"
    msg "   sysctl -w $1"
    sysctl -w "$1"
}


configure_container_sysctl() {
    msg "configure sysctl on container"

    write_sysctl_con net.ipv4.udp_l3mdev_accept=1 # <-- used for mgmt VRF which we dont use in DPF ?
    write_sysctl_con net.ipv4.tcp_l3mdev_accept=1 # <-- used for mgmt VRF which we dont use in DPF ?
    write_sysctl_con net.ipv6.conf.all.forwarding=1
    write_sysctl_con net.ipv4.ip_forward=1 # <- already set as part of k8s prereq
    msg "configure sysctl on container complete"
}


#Verify iptables modules on host level
verify_iptabels() {
    msg "verifying iptables modules on host"
    chroot /host nsenter -t 1 -n /usr/sbin/ebtables-legacy -t filter -L > /dev/null
    chroot /host nsenter -t 1 -n /usr/sbin/iptables-legacy -t filter -L > /dev/null
    chroot /host nsenter -t 1 -n /usr/sbin/ip6tables-legacy -t filter -L > /dev/null
}


#Create directories and files required for HBN startup
copy_required_startup_configs() {
  msg "copying required startup configs"

# frr config files
  mkdir -p "${HBN_CONFIG_DIR}"/etc/frr

  if [[ ! -e "${HBN_CONFIG_DIR}"/frr/daemons ]]; then
    cp "${HBN_DEFAULT_FILES_PATH}"/etc/frr/frr.daemons "${HBN_CONFIG_DIR}"/etc/frr/daemons
  fi

  if [[ ! -e "${HBN_CONFIG_DIR}"/frr/frr.conf ]]; then
    cp "${HBN_DEFAULT_FILES_PATH}"/etc/frr/frr.conf "${HBN_CONFIG_DIR}"/etc/frr/frr.conf
  fi

  if [[ ! -e "${HBN_CONFIG_DIR}"/frr/vtysh.conf ]]; then
    cp "${HBN_DEFAULT_FILES_PATH}"/etc/frr/vtysh.conf "${HBN_CONFIG_DIR}"/etc/frr/vtysh.conf
  fi

  if [[ ! -e "${HBN_CONFIG_DIR}"/frr/support_bundle_commands.conf ]]; then
    cp "${HBN_DEFAULT_FILES_PATH}"/etc/frr/support_bundle_commands.conf "${HBN_CONFIG_DIR}"/etc/frr/support_bundle_commands.conf
  fi

# ifupdown2 config files
  mkdir -p "${HBN_CONFIG_DIR}"/etc/network/ifupdown2
  if [[ ! -e "${HBN_CONFIG_DIR}"/etc/network/ifupdown2/addons.conf ]]; then
	cp "${HBN_DEFAULT_FILES_PATH}"/etc/network/ifupdown2/addons.conf "${HBN_CONFIG_DIR}"/etc/network/ifupdown2/addons.conf
  fi

  if [[ ! -e "${HBN_CONFIG_DIR}"/etc/network/ifupdown2/ifupdown2.conf ]]; then
    cp "${HBN_DEFAULT_FILES_PATH}"/etc/network/ifupdown2/ifupdown2.conf "${HBN_CONFIG_DIR}"/etc/network/ifupdown2/ifupdown2.conf
  fi

# mgmt.intf config file
  mkdir -p "${HBN_CONFIG_DIR}"/etc/network/interfaces.d
  if [[ ! -e "${HBN_CONFIG_DIR}"/etc/network/interfaces.d/mgmt.intf ]]; then
        cp "${HBN_DEFAULT_FILES_PATH}"/etc/network/interfaces.d/mgmt.intf "${HBN_CONFIG_DIR}"/etc/network/interfaces.d/mgmt.intf
  fi

# supervisor config files 
  mkdir -p "$HBN_CONFIG_DIR"/etc/supervisor/conf.d

  if [[ ! -e "$HBN_CONFIG_DIR"/etc/supervisor/conf.d/supervisor-isc-dhcp-relay.conf ]]; then
        cp "${HBN_DEFAULT_FILES_PATH}"/etc/supervisor/conf.d/supervisor-isc-dhcp-relay.conf "${HBN_CONFIG_DIR}"/etc/supervisor/conf.d/supervisor-isc-dhcp-relay.conf
  fi

  if [[ ! -e "${HBN_CONFIG_DIR}"/etc/supervisor/conf.d/supervisor-isc-dhcp-relay6.conf ]]; then
          cp "${HBN_DEFAULT_FILES_PATH}"/etc/supervisor/conf.d/supervisor-isc-dhcp-relay6.conf "${HBN_CONFIG_DIR}"/etc/supervisor/conf.d/supervisor-isc-dhcp-relay6.conf
  fi

  if [[ ! -e "$HBN_CONFIG_DIR"/etc/supervisor/conf.d/supervisor-isc-dhcp-server.conf ]]; then
          cp "${HBN_DEFAULT_FILES_PATH}"/etc/supervisor/conf.d/supervisor-isc-dhcp-server.conf "${HBN_CONFIG_DIR}"/etc/supervisor/conf.d/supervisor-isc-dhcp-server.conf
  fi

  if [[ ! -e "${HBN_CONFIG_DIR}"/etc/supervisor/conf.d/supervisor-isc-dhcp-server6.conf ]]; then
          cp "${HBN_DEFAULT_FILES_PATH}"/etc/supervisor/conf.d/supervisor-isc-dhcp-server6.conf "${HBN_CONFIG_DIR}"/etc/supervisor/conf.d/supervisor-isc-dhcp-server6.conf
  fi

  if [[ ! -e "${HBN_CONFIG_DIR}"/etc/supervisor/conf.d/nginx-authenticator.conf ]]; then
          cp "${HBN_DEFAULT_FILES_PATH}"/etc/supervisor/conf.d/nginx-authenticator.conf "${HBN_CONFIG_DIR}"/etc/supervisor/conf.d/nginx-authenticator.conf
  fi

# cumulus config files
  if [[ ! -e "${HBN_CONFIG_DIR}"/etc/cumulus/nl2docad.conf ]]; then
         cp -r "${HBN_DEFAULT_FILES_PATH}"/etc/cumulus "${HBN_CONFIG_DIR}"/etc/
  fi
}

create_additional_dirs() {

  msg "create /etc/nvue.d dir for startup.yaml"
  mkdir -p "${HBN_CONFIG_DIR}"/etc/nvue.d

  msg "create supervisor log dir"
  mkdir -p "${HBN_LOG_DIR}"/supervisor

  msg "create support/core dir"
  mkdir -p "${HBN_CONFIG_DIR}"/var/support/core

}

#Generate HBN configuration file for local DPU node
#The operation is based on files named: hbn_nodes_config.json,startup.yaml.j2 provided using ConfigMaps
generate_configuration_files() {
    local config_dir="/tmp/config-data"
    local template_dir="/tmp/j2-startup-template"
    local target_dir="${HBN_CONFIG_DIR}/etc/nvue.d"
    local hostname="${HOSTNAME}"

    echo "Extract the local node configuration parameters from the multi-node configuration file"
    local host_config=$(jq -r --arg hostname "${hostname}" '.[$hostname]' "${config_dir}/hbn_nodes_config.json")

    if [ "$host_config" != "null" ]; then
        echo "Render startup configuration with local node parameters from Jinja2 template"
        python3 -c "from jinja2 import Environment, FileSystemLoader; \
            env = Environment(loader=FileSystemLoader('${template_dir}'), autoescape=True); \
            template = env.get_template('startup.yaml.j2'); \
            print(template.render(config=${host_config}))" > "${target_dir}/startup.yaml"
        echo "Save generated nvue startup configuration file for the local node"
    else
        echo "No configuration parameters found for local node: ${hostname}"
    fi
}

# aserdean: This is a HACK in order to make HBN 2.2 happy and see the ports inside the container
#           via the command devlink port show.
#           TODO remove this workaround in the future since HBN 2.3 will not need it
msg "Moving host SFs to container using devlink port reload"
POD=$(chroot /host crictl pods | grep ${POD_NAME} | cut -d " " -f 1)
NS=$(chroot /host crictl inspectp ${POD} | jq -r '.info.runtimeSpec.linux.namespaces[]  | select(.type=="network") | .path' | cut -d "/" -f 5)
PORTS=$(ip -br a | grep _sf | cut -f1 -d ' ')

for port in $PORTS
do
   msg "processing ${port}"
   aux_num=$(ls -l /sys/class/net/ | grep ${port} | awk  -F '/' '{print $9}')
   pci_id=$(chroot /host nsenter -t 1 -n devlink port show | grep ${aux_num} |cut -f1 -d ' ' | tr ':' ' ' | tr '\n' ' ')
   port_r=$(echo ${pci_id} | cut --complement -d'/' -f3)
   msg "pci_id=${pci_id}"
   msg "devlink dev reload ${port_r} netns ${NS}"
   chroot /host nsenter -t 1 -n devlink dev reload ${port_r} netns ${NS}
   intf_post=$(devlink port show ${pci_id} | cut -f5 -d ' ' | head -1)
   msg "renaming ${intf_post} with ${port}"
   ip link set name ${port} dev ${intf_post}
done

msg "SFs moved to container"

msg "HBN initial config for DPF/SFC started."

configure_host_sysctl

configure_container_sysctl

verify_iptabels

copy_required_startup_configs

create_additional_dirs

generate_configuration_files

msg "HBN initial config for DPF/SFC completed."


