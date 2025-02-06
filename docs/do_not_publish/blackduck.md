# Black Duck Integration

This doc describes how the Black Duck integration works with GitLab and how one can trigger it. Additional info about
Black Duck can be found in https://confluence.nvidia.com/pages/viewpage.action?spaceKey=SW&title=Blackduck.

## Credentials

All of the credentials are created by using the `svc-dpf-release@nvidia.com` Service Account. Ping @kmuldoon to get the
root credentials for that Service Account.

There are 2 types of tokens used:
* Black Duck: One needs to login to https://blackduck.mellanox.com/ and generate an access token which can be found
  in the top right corner dropdown.
* Gerrit: One needs to login to https://git-nbu.nvidia.com/r/settings/#HTTPCredentials and create a new password. This
  password in reality is a token.

  **Warning**: If you click on the "Generate New Password", the old token will immediately become invalid and you have
  to update the places where this token is used.

## Permissions

### Service Accounts

* The Service Account should have `Project Code Scanner` permission in the Black Duck project.

### Humans

In case one wants to access the project in Black Duck, ask @nrubinstein or @kmuldoon to add you with `Project Viewer`
role. Unfortunately, adding a group that is automatically populated by the distribution list is not possible due to
Black Duck limitation.

## Pipelines

There is a single pipeline used for Black Duck which can be found under the scheduled pipelines in GitLab. It requires
the following environment variables:
* `BLACKDUCK`: set to "true" so that the job defined in [.gitlab-ci.yml](../../..gitlab-ci.yml) can be triggered
* `TAG`: set to the tag that should be displayed in the Black Duck UI after the pipeline is triggered
* `BLACK_DUCK_API_TOKEN`: token generated via the Black Duck UI, see [Credentials](#credentials)
* `GERRIT_USERNAME`: set to the username of the service account without the @nvidia.com (i.e. `svc-dpf-release`)
* `GERRIT_PASSWORD`: token generated via the Gerrit UI, see [Credentials](#credentials)

## How to Run

The pipeline in theory runs on a schedule once per year, but the schedule exists just so that people can trigger it
manually whenever needed.

Whenever one needs to run a Black Duck scan, they have to find the pipeline under the scheduled pipelines in GitLab
and:
1. Adjust the branch/tag to the branch/tag the scan should run against
2. Adjust the `TAG` environment variable, see [Pipelines](#pipelines)
3. Run the pipeline (this may take a while to run)

The result will be visible in https://blackduck.mellanox.com/api/projects/c73826c4-2dd7-4655-a17b-bc3a72607375
