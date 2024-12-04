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

: ${NGC_API_KEY:?env not set}
: ${VM_PASSWORD:?env not set}

# Even if we don't use this directory we have to ensure that it is created.
mkdir -p /bfb-images

# Run DPF Standalone from https://gitlab-master.nvidia.com/doca-platform-foundation/dpf-standalone
docker run --pull=always --rm --net=host nvcr.io/nvstaging/doca/dpf-standalone:latest \
    -u root \
    -e "ansible_password=$VM_PASSWORD" \
    -e "ngc_key=$NGC_API_KEY" \
    -e '{"deploy_dpf_bfb_pvc": false}' \
    -e '{"deploy_dpf_create_node_feature_rule": false}' \
    -e '{"deploy_dpf_operator_chart": false}' \
    -e '{"deploy_dpf_create_operator_config": false}' \
    install.yml
