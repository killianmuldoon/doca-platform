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

# create-ngc-secrets.sh will create secrets required to pull images and helm charts from NGC if an NGC API KEY is set in the environment

NGC_API_KEY="${NGC_API_KEY:-""}"

if [[ "$NGC_API_KEY" == "" ]]; then
  echo "NGC_API_KEY not set. Skipping NGC secret creation"
  exit 1
fi


## Create a pull secret to be used for images in Kubernetes
kubectl -n dpf-operator-system create secret docker-registry ngc-secret --docker-server=nvcr.io --docker-username="\$oauthtoken" --docker-password=$NGC_API_KEY

## Create a pull secret to be used by ArgoCD for pulling helm charts
echo "
apiVersion: v1
kind: Secret
metadata:
  name: nvstaging-helm
  namespace: dpf-operator-system
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  name: nvstaging
  url: https://helm.ngc.nvidia.com/nvstaging/mellanox
  type: helm
  username: \$oauthtoken
  password: $NGC_API_KEY
" | kubectl apply -f -
