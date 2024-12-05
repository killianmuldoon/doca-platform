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

#### 2 Setting up CI jobs for the new branch

The presubmit jobs that run on each and every Merge Request (MR) in the DPF repository should start to work on new MRs to the release branch without issue. Periodic jobs, however, need to be set up so that they run on the new branch.

To do so navigate to [Build > Pipeline schedules](https://gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/-/pipeline_schedules).

For each job prefixed with `main:`:
- Press `New schedule` to define a new job
- Copy the title of the source job. Update the prefix from `main:` to `v24.10.0`.
- Copy all variables from the source job to the target job
- Update variables which contain a version number to `v24.10.0`


#### 3 Cleaning up old jobs

TODO: Define cleanup process for cleaning up old jobs once a GA release has been done and the support window is closed.



