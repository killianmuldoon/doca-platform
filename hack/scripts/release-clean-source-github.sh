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

: ${GITHUB_RELEASE_TOKEN:?env not set}
: ${PREVIOUS_RELEASE_TAG}:?env not set}
: ${TAG:?env not set}

files=(.gitlab
  .gitlab-ci.yml
  hack/scripts/ci-rebind-dev-wrapper.sh
  hack/scripts/ci_helm_docker_prereqs.sh
  hack/scripts/container-scanner.sh
  hack/scripts/create-artefact-secrets.sh
  hack/scripts/e2e-provision-standalone-cluster.sh
  hack/scripts/gitlab_runner.sh
  hack/scripts/nic-cloud-gitlab-runner.sh
  hack/scripts/periodic_test_triage.sh
  hack/scripts/release-clean-source-github.sh
  hack/scripts/slack-notification.sh
)

#git fetch --unshallow
COMMIT_MESSAGE=$(git log --pretty=tformat:'Co-authored-by: %an <%ae>' | sort -u)

#for file in ${files[@]}; do
#  git rm -r --cached $file ;
#done

git config user.email "svc-dpf-release@nvidia.com"
git config user.name "DPF Release bot"

# TODO: After the initial release this script should produce a commit on top of the previous commit on GitHub.
# This line can be removed at this point as it handles a special case for the first release.
if [ "$PREVIOUS_RELEASE_TAG" == "0" ]; then
   git reset --soft $(git rev-list --max-parents=0 HEAD)
   git commit --author="DPF Release bot <svc-dpf-release@nvidia.com>" --amend --date=now -s -m "DPF Release $TAG" -m "$COMMIT_MESSAGE"
 else
   git reset --soft $(git rev-list -n 1 $PREVIOUS_RELEASE_TAG)
   git commit --author="DPF Release bot <svc-dpf-release@nvidia.com>" --date=now -s -m "DPF Release $TAG" -m "$COMMIT_MESSAGE"
fi

git tag $TAG

git remote rm public || true
git remote add public https://$GITHUB_RELEASE_TOKEN@github.com/killianmuldoon/doca-platform
git push --force public HEAD:main
git push --force origin tag $TAG








