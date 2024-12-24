This document describes how to build SPDK CSI image, helm chart and how to deploy SPDK CSI into the DPF environment through DPUservice.

[SPDK CSI plugin](https://github.com/spdk/spdk-csi) is a open source project which brings SPDK to Kubernetes. It provisions SPDK logical volumes on storage node dynamically and enables Pods to access SPDK storage backend through NVMe-oF or iSCSI.

## Prerequisites



### Build image
[SPDK CSI plugin](https://github.com/spdk/spdk-csi) does not provide the released image. We need build the image from source code.

*Beofre building/pushing images and helm chart, it is recommended to set `CSI_IMAGE_REGISTRY` and `CSI_IMAGE_TAG` for the image.*

- `$ git clone https://github.com/spdk/spdk-csi.git`
- `$ make image`

### Push image
Use `docker push` command to push the SPDK CSI controller image to your image registry. 

### Build helm chart
- `$ make helm-package-spdk-csi-controller`

### Push helm chart
- `$ make helm-push-spdk-csi-controller`

### Deploy SPDK CSI controller through DPUService
#### Create RBAC and storage class in DPU cluster
Create the `spdk-csi-controller-dpu-cluster-role.yaml` and `storageclass.yaml` in DPU cluster.

#### Create DPUServiceCredentialRequest in host cluster

`dpu-service/DPUServiceCredentialRequest.yaml` is an example of a `DPUServiceCredentialRequest` which requests credentials
to access a cluster. Need update the `namespace` base on the namespace where the `DPUCluster` is in your DPF environment. 

- `$ kubectl apply -f dpu-service/DPUServiceCredentialRequest.yaml`

#### Create DPUService in host cluster

`dpu-service/DPUService.yaml` is an example of a `DPUService` which requests credentials
to access a cluster. Need update the `namespace` base on the namespace where the `DPUCluster` is in your DPF environment. Also need update the `spec.helmChart.values.csiConfig` according to your spdk storage.

- `$ kubectl apply -f dpu-service/DPUService.yaml`