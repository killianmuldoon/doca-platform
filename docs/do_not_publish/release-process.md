## DPF release process

### 1 Cutting a new branch

#### 1 Creating a new branch and protecting it
This documentation describes example steps for cutting a branch for a new DPF release. For this example the branch will be called `release-v24.10` and created for a release version `v24.10.0`.

A new branch should always be cut from main. Once the new branch is prepared `main` can be used as the bleeding edge to continue developing DPF.

A new branch can be created from the Gitlab UI at under [Code > Branches](https://gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/-/branches). The branch should be called `release-v24.10`

The new branch must be protected by going to [Settings > Repository > Protected branches ](https://gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/-/settings/repository). Press the "Add protected branch" button. Select `release-24.10` from the `Branch:` drop down menu. The setting should be copied from main. 

For this example:
- `Allowed to merge` should be set to "Maintainers"
- `Allowed to push and merge` should be set to "No one"
- `Allowed to force push` should be off
- `Require approval from code owners` should be off

Press the `Protect` button to add the branch as a protected branch.

#### 2 Adding protection rules

DPF has a conservative approach to making changes to release branches once they are created. Only a limited set of people should be allowed to merge new changes to ensure the stability of the branch.

To enforce this go to [Settings > Merge Requests > Merge request approvals](https://gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/-/settings/merge_requests) and press the `Add approval rule` button. 

In the window add the following:

- `Rule name` "v24.10.0 cherry-pick"
- `Target branch` "release-v24.10"
- `Users` - add the limited set approvers who have the ability to merge changes into the release branch
- Press `Save changes`

#### 3 Setting up CI jobs for the new branch

The presubmit jobs that run on each and every Merge Request (MR) in the DPF repository should start to work on new MRs to the release branch without issue. Periodic jobs, however, need to be set up so that they run on the new branch.

To do so navigate to [Build > Pipeline schedules](https://gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/-/pipeline_schedules).

For each job prefixed with `main:`:
- Press `New schedule` to define a new job
- Copy the title of the source job. Update the prefix from `main:` to `v24.10.0`.
- Copy all variables from the source job to the target job
- Update variables which contain a version number to `v24.10.0`


#### 4 Cleaning up old jobs

TODO: Define cleanup process for cleaning up old jobs once a GA release has been done and the support window is closed.


### 2 Publishing code and release artefacts to Github

Publishing a release must be done by someone with write access to the public Github repo. @Nadav-Rub @adrianchiris and @killianmuldoon are admins who can give write access on the repo.
This documentation has additional steps which cover the [First release] which are marked.

Make sure you are part of NVIDIA GitHub organization here: https://github.com/settings/organizations.

If not, you can join using https://github-onboarding.nvidia.com.
Select NVIDIA, and input your NVIDIA email. You will get an invitation by mail.

#### 1 Create a GitHub personal access token

Log in to your GitHub account. Go to https://github.com/settings/tokens.

Click `Generate new token > Generate new token (classic)`. Add "DPF release" as a Note. Set Expiration to 7 days.

Enable the fololowing permissions:
- repo (all)
- write:packages
- read:packages

Click `Generate token` at the bottom of the page. Make a note of the value of the generated token - YOUR_GITHUB_TOKEN - which appears on the next page. It will have the form `ghp_XXXXXX`

On the Personal access tokens (classic) page click the `Configure SSO` button, select 'NVIDIA' organization and go through the SSO process.

#### 2 Add the GitHub token to Gitlab

Go to the [Gitlab CI page for DPF](https://gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/-/settings/ci_cd) and navigate to "Variables". Click the `Add variable` button. 

Add a new variable with:

- `Visibility`: Masked
- `Key`: "GITHUB_RELEASE_TOKEN"
- `Value`: $YOUR_GITHUB_TOKEN

Press `Add variable`.

Navigate to the [GitLab pipeline schedule page](https://gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/-/pipeline_schedules). Open the "Github release" job. By clicking the "pencil image" with the tooltip "Edit pipeline schedule." Take ownership of the job if needed. Ensure that the `TAG` variable has the correct value e.g. for the `v24.10.0` release this value should be `v24.10.0`. Click `Save changes`.

On the Schedules page press the `Run pipeline schedule` button for Github release. This will kick off the full release process including both images and source code.

Review the externally published code to ensure it's the expected version. Review the published containers to ensure they have the correct Git commit as a label with the key `org.opencontainers.image.revision`.

The release should contain the following packages on GitHub:
- https://github.com/NVIDIA/doca-platform/pkgs/container/dpf-system
- https://github.com/NVIDIA/doca-platform/pkgs/container/ovn-kubernetes
- https://github.com/NVIDIA/doca-platform/pkgs/container/dpf-tools
- https://github.com/NVIDIA/doca-platform/pkgs/container/ovs-cni-plugin
- https://github.com/NVIDIA/doca-platform/pkgs/container/hostdriver
- https://github.com/NVIDIA/doca-platform/pkgs/container/dpf-operator
- https://github.com/NVIDIA/doca-platform/pkgs/container/ovn-kubernetes-chart
- https://github.com/NVIDIA/doca-platform/pkgs/container/dpu-networking

#### 3 Add release notes

On github.com/nvidia/doca-platform [create a new release](https://github.com/NVIDIA/doca-platform/releases/new).

For "Release title" use the version e.g. v24.10.0.

For the "Description" copy and paste the contents from [the release notes folder](./../release-notes) for the relevant version e.g. for Release v24.10.0 the [release notes are here](./../release-notes/v24.10.0.md).

Click `Publish release`

#### 4 Revoke your personal token and remove it from GitLab variables

- Go https://github.com/settings/tokens on your GitHub profile. Click "Delete" beside the "DPF release" token.
- Go to the [Gitlab CI page for DPF](https://gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/-/settings/ci_cd) and navigate to "Variables". Click the "Delete" bin image for the variable `GITHUB_RELEASE_TOKEN`. 

#### [First release] Change repository and package visibility

TODO: Fill out this process.
