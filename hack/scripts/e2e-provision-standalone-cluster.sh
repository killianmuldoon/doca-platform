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

set -o nounset
set -o pipefail
set -o errexit

# Note the node that this script runs on must be set up as a gitlab runner with the root user.
# The node must have an available Bluefield DPU.
# SR-IOV VFs should be precreated on the DPU.

# Provision a NIC Cloud setup as a standalone DPF Cluster. Used for e2e tests.
if [[ -z "$NGC_API_KEY" ]]; then
    echo "NGC_API_KEY not set"
    exit 1
fi

## This is a secret and should be preset in the environment.
if [[ -z "$VM_PASSWORD" ]]; then
    echo "VM_PASSWORD not set"
    exit 1
fi

# Ensure the BFBs have been copied to this node so they're accessible.
mkdir -p /bfb-images
cp /auto/sw_mc_soc_release/doca_dpu/doca_2.9.0/20241020/bfbs/qp/bf-bundle-2.9.0-64_24.10_ubuntu-22.04_unsigned.bfb /bfb-images

# Run DPF Standalone from https://gitlab-master.nvidia.com/doca-platform-foundation/dpf-standalone
docker run --pull=always --rm --net=host nvcr.io/nvstaging/mellanox/dpf-standalone:latest \
    -u root \
    -e ansible_password=$VM_PASSWORD \
    -e ngc_key=$NGC_API_KEY \
    -e '{"deploy_dpf_bfb_pvc": false}' \
    -e '{"deploy_dpf_create_node_feature_rule": false}' \
    -e '{"deploy_dpf_operator_chart": false}' \
    -e '{"deploy_dpf_create_operator_config": false}' \
    install.yml
