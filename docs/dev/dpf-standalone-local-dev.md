# dpf-standalone Local Dev

This guide helps the developer in setting and running a local dpf-standalone env that can be used to deploy the various DPF components and run tests. Allow user to run full provisioning and service chaining flows.

## Prerequisites
1. [GO](https://go.dev/doc/install) >= 1.22
2. [Docker Engine](https://docs.docker.com/engine/install/)
3. [dpf-standalone prerequisites](https://gitlab-master.nvidia.com/doca-platform-foundation/dpf-standalone#prerequisites)

## Dev Flow
1. Go to your local dev project git repo
```
cd <path_to_local_repo/doca-platform-foundation
```
2. Export relevant env vars for image building and pushing
```
export REGISTRY=gitlab-master.nvidia.com:5005/doca-platform-foundation/doca-platform-foundation/e2e
export IMAGE_PULL_KEY=<GITLAB_API_TOKEN>
export TAG=v0.1.0-$(git rev-parse --short HEAD)-$USER-test
```
3. Generate all 
```
make generate
```
4. Build images required for the quick DPF e2e test.
```
make test-release-e2e-quick
```
5. SSH to local machine
6. Export NGC API Key
```
export NGC_API_KEY=<NGC_API_KEY>
```
7. docker login to NGC Catalog # can use podman as well
```
echo "$NGC_API_KEY" | docker login --username \$oauthtoken --password-stdin nvcr.io
```
8. Pull latest dpf-standalone image
```
docker pull nvcr.io/nvstaging/mellanox/dpf-standalone
```
9. Run dpf-standalone # you will be required to provide the host root password to begin installation
```
docker run -ti --rm --net=host nvcr.io/nvstaging/mellanox/dpf-standalone \
  -k -K -u root \
  -e ngc_key=$NGC_API_KEY \
  -e '{"deploy_dpf_operator_chart": false}' \
  -e '{"deploy_dpf_bfb_pvc": false}' \
  -e '{"deploy_dpf_create_node_feature_rule": false}' \
  -e '{"deploy_dpf_create_operator_config": false}' \
  install.yml
```
10. Create a directory for BFB images
```
mkdir -p /bfb-images
```
11. Copy a BFB file from DOCA Release dir to local /bfb-images dir. For example:
```
cp /auto/sw_mc_soc_release/doca_dpu/doca_2.7.0/GA/bfbs/qp/bf-bundle*ubuntu-22.04_unsigned.bfb /bfb-images
```
12. On the target machine - Go to your local dev project git repo
```
cd <path_to_local_repo/doca-platform-foundation
```
13. Generate image pull secrets
```
hack/scripts/create-artefact-secrets.sh
```
14. Deploy DPF Operator
```
make test-deploy-operator-helm
```

### Run e2e Tests
To run DPF e2e tests:
15. Export dpf-standalone-e2e testing vars
```
export DPF_E2E_NUM_DPU_NODES=1
```
16. Run e2e tests
```
make test-e2e
```
_Note:_
If you want to keep the created resources (e.g., `DPFOperatorConfig`, `DPU` and such) you can use the environment variable `E2E_SKIP_CLEANUP=true`
```
E2E_SKIP_CLEANUP=true make test-e2e
```

### Provisioning Flow
To test full provisioning flow, follow steps 1-8. Step 9 command should be:

9. Run dpf-standalone # you will be required to provide the host root password to begin installation
```
docker run -ti --rm --net=host nvcr.io/nvstaging/mellanox/dpf-standalone \
  -k -K -u root \
  -e ngc_key=$NGC_API_KEY \
  -e '{"deploy_dpf_operator_chart": false, "deploy_dpf_bfb_pvc": false, "deploy_dpf_create_node_feature_rule": false, "deploy_dpf_create_operator_config": false, "deploy_test_service": true, "target_host": "<target-host-ip-here>"}' \
  install.yml
```
_Note:_ a host reboot will happen during the provisioning, so it is not possible to provision the local host
(the host from which the dpf-standalone command is executed). You should use dpf-standalone from another host with
-e target_host argument.

 Continue with steps 10-14 and then follow [DPU Provisioning](https://gitlab-master.nvidia.com/doca-platform-foundation/dpf-standalone#dpu-provisioning)

### Test DPU Services
To test full provisioning flow, follow steps 1-14 and then follow [DPU Services](https://gitlab-master.nvidia.com/doca-platform-foundation/dpf-standalone#deploy-test-service-on-dpu)

### Clean dpf-standalone env
To clean the env run:
```
docker run --rm --net=host nvcr.io/nvstaging/mellanox/dpf-standalone:latest -u root reset.yml
```
## Troubleshooting
