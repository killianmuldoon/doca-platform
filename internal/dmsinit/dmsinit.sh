#!/bin/bash

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

set -euo pipefail

# Define constants
readonly DEFAULT_EXTERNAL_CERTIFICATE='none'
readonly DEFAULT_K8S_ENV='true'
readonly DEFAULT_DMS_IP='0.0.0.0'
readonly DEFAULT_DMS_PORT=9339
readonly DEFAULT_NAMESPACE='dpf-operator-system'
readonly DEFAULT_ISSUER='dpf-provisioning-issuer'
readonly DEFAULT_KUBERNETES_VERSION='1.32.0'
readonly DEFAULT_NODE_REBOOT_METHOD='DMS'
readonly VALID_NODE_REBOOT_METHODS=('DMS' 'external' 'custom_script')

# Log function
log() {
  echo "[dmsinit] $1"
}

# Error function
error() {
  echo "[dmsinit] Error (${FUNCNAME[1]}:${BASH_LINENO[0]}): $1" >&2
  exit 1
}

# Define functions
check_and_install_kubectl() {
  if ! command -v kubectl &>/dev/null; then
    log "kubectl not found, downloading and installing kubectl $DEFAULT_KUBERNETES_VERSION in /tmp/doca-bin"
    mkdir -p /tmp/doca-bin
    if ! curl -sSfL -o /tmp/doca-bin/kubectl "https://dl.k8s.io/release/v$DEFAULT_KUBERNETES_VERSION/bin/linux/amd64/kubectl"; then
      error "Failed to download kubectl"
    fi
    chmod +x /tmp/doca-bin/kubectl || error "Failed to set execute permissions for kubectl"
    export PATH="/tmp/doca-bin:$PATH"
  else
    log "kubectl already installed"
  fi
  output=$(kubectl version 2>&1)
  if [ $? -ne 0 ]; then
    error "Failed to check kubectl version: $output"
  fi
}

cleanup() {
  log "Cleaning up"
  if [ -f /tmp/doca-bin/kubectl ]; then
    if ! rm -f /tmp/doca-bin/kubectl; then
      log "Failed to remove /tmp/doca-bin/kubectl"
    fi
  fi
}

check_resource_exists() {
  local api_group=$1
  local resource_type=$2
  local resource_name=$3
  output=$($kubectl_cmd  -n $namespace get -o json $resource_type $resource_name 2>&1)
  if [ $? -ne 0 ]; then
    return 1
  fi
  log "Resource $resource_name of type $resource_type in API group $api_group already exists, skipping creation"
  return 0
}

create_certificate() {
  if ! check_resource_exists "cert-manager.io" "Certificate" "$dpu_node_name-dms-server-cert"; then
    log "Creating certificate"
    yaml=$(cat <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: $dpu_node_name-dms-server-cert
  namespace: $namespace
spec:
  secretName: $dpu_node_name-server-secret
  commonName: $dpu_node_name-dms-server-cert
  duration: 8760h
  issuerRef:
    name: $issuer
    kind: Issuer
  usages:
  - server auth
  ipAddresses:
  - "$dms_ip"
EOF
)
    output=$($kubectl_cmd apply -f - <<<"$yaml" 2>&1)
    if [ $? -ne 0 ]; then
      error "Failed to create certificate $dpu_node_name-dms-server-cert: $output"
    fi
    log "Certificate $dpu_node_name-dms-server-cert created successfully"
  fi
}

create_dpu_device() {
  dpudevice_names=()
  for pci_addr in "${pci_address[@]}"; do
    if ! check_resource_exists "provisioning.dpu.nvidia.com" "DPUDevice" "dpu-device-$dpu_node_name-$pci_addr"; then
      log "Creating DPUDevice for PCI address $pci_addr"
      yaml=$(cat <<EOF
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUDevice
metadata:
  name: dpu-device-$dpu_node_name-$pci_addr
  namespace: $namespace
spec:
  pciAddress: $pci_addr
EOF
)
      output=$($kubectl_cmd apply -f - <<<"$yaml" 2>&1)
      if [ $? -ne 0 ]; then
        error "Failed to create DPUDevice dpu-device-$dpu_node_name-$pci_addr: $output"
      fi
      log "DPUDevice dpu-device-$dpu_node_name-$pci_addr created successfully"
      dpudevice_names+=("dpu-device-$dpu_node_name-$pci_addr")
    fi
  done
}

create_dpunode() {
  if ! check_resource_exists "provisioning.dpu.nvidia.com" "DPUNode" "$dpu_node_name"; then
    log "Creating DPUNode"
    yaml=$(cat <<EOF
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUNode
metadata:
  name: $dpu_node_name
  namespace: $namespace
spec:
  nodeRebootMethod: $node_reboot_method
EOF
)
    if [ -n "$dms_ip" ] && [ -n "$dms_port" ]; then
      yaml=$(cat <<EOF
$yaml
  nodeDMSAddress:
    ip: $dms_ip
    port: $dms_port
EOF
)
    fi
if [ ${#dpudevice_names[@]} -gt 0 ]; then
  yaml=$(cat <<EOF
$yaml
  dpus:
EOF
)
  for device in "${dpudevice_names[@]}"; do
    yaml=$(cat <<EOF
$yaml
    $device: true
EOF
)
  done
fi

    if [ -n "$kube_node_ref" ]; then
      yaml=$(cat <<EOF
$yaml
status:
  kubeNodeRef: $kube_node_ref
EOF
)
    fi
    output=$($kubectl_cmd apply -f - <<<"$yaml" 2>&1)
    if [ $? -ne 0 ]; then
      error "Failed to create DPUNode $dpu_node_name: $output"
    fi
    log "DPUNode $dpu_node_name created successfully"
  fi
}

parse_arguments() {
  allowed_arguments=(
    "--kubeconfig"
    "--external-certificate"
    "--kube-node-ref"
    "--dms-ip"
    "--dms-port"
    "--pci-address"
    "--k8s-env"
    "--issuer"
    "--node-reboot-method"
    "--namespace"
  )

  node_reboot_method=$DEFAULT_NODE_REBOOT_METHOD

  while [[ $# -gt 0 ]]; do
    case $1 in
      --kubeconfig)
        kubeconfig=$2
        shift 2
        ;;
      --external-certificate)
        external_certificate=$2
        shift 2
        ;;
      --kube-node-ref)
        kube_node_ref=$2
        shift 2
        ;;
      --dms-ip)
        dms_ip=$2
        shift 2
        ;;
      --dms-port)
        dms_port=$2
        shift 2
        ;;
      --pci-address)
        pci_address=($(echo $2 | tr ',' ' '))
        pci_address=(${pci_address[@]//:/-})
        shift 2
        ;;
      --k8s-env)
        k8s_env=$2
        shift 2
        ;;
      --namespace)
        namespace=$2
        shift 2
        ;;
      --issuer)
        issuer=$2
        shift 2
        ;;
      --node-reboot-method)
        node_reboot_method=$2
        if [[ ! " ${VALID_NODE_REBOOT_METHODS[@]} " =~ " ${node_reboot_method} " ]]; then
          error "Invalid node reboot method: $node_reboot_method. Valid options are: ${VALID_NODE_REBOOT_METHODS[*]}"
        fi
        shift 2
        ;;
      *)
        if [[ ! " ${allowed_arguments[@]} " =~ " $1 " ]]; then
          cat <<EOF >&2
Unrecognized argument: $1
Valid arguments:
  --kubeconfig                       <path>      Path to the kubeconfig file
  --external-certificate             <cert>      External certificate (default: none)
  --kube-node-ref                    <ref>       Kube node reference
  --dms-ip                           <ip>        DMS IP address (default: 0.0.0.0)
  --dms-port                         <port>      DMS port (default: 9339)
  --pci-address                      <addresses> Comma separated list of PCI addresses
  --k8s-env                          <bool>      Whether to use K8s environment (default: true)
  --issuer                           <issuer>    Issuer name (default: dpf-provisioning-issuer)
  --node-reboot-method               <method>    Node reboot method (default: DMS). Valid options: DMS, external, custom_script
EOF
          error "Unknown option: $1"
        fi
        ;;
    esac
  done

  external_certificate=${external_certificate:-$DEFAULT_EXTERNAL_CERTIFICATE}
  kube_node_ref=${kube_node_ref:-}
  dms_ip=${dms_ip:-$DEFAULT_DMS_IP}
  dms_port=${dms_port:-$DEFAULT_DMS_PORT}
  pci_address=(${pci_address[@]-})
  k8s_env=${k8s_env:-$DEFAULT_K8S_ENV}
  namespace=${namespace:-$DEFAULT_NAMESPACE}
  issuer=${issuer:-$DEFAULT_ISSUER}
  node_reboot_method=${node_reboot_method:-$DEFAULT_NODE_REBOOT_METHOD}
  kubeconfig=${kubeconfig:-}
  kubectl_cmd="kubectl"
  if [ -n "$kubeconfig" ]; then
    kubectl_cmd="kubectl --kubeconfig $kubeconfig"
  fi
  if [ "$k8s_env" = true ]; then
      if [ -z "$kube_node_ref" ]; then
      error "in k8s env, kube-node-ref is required"
    fi
  dpu_node_name=$kube_node_ref
else
  dpu_node_name=$(hostname | tr '[:upper:]' '[:lower:]')
fi
}

trap cleanup INT TERM ILL KILL FPE SEGV ALRM ERR EXIT

main() {
  log "Starting main function"
  check_and_install_kubectl

  create_dpu_device

  create_dpunode

  if [ "$external_certificate" = "none" ]; then
    create_certificate
  fi

  log "Main function completed successfully"
  cleanup
  trap - INT TERM ILL KILL FPE SEGV ALRM ERR EXIT
}

parse_arguments "$@"

if ! main; then
  error "Failed to complete main function: $?"
fi
