# OVN Kubernetes with Host Based Networking

In this configuration OVN Kubernetes is offloaded to the DPU and combined with [NVIDIA Host Based Networking (HBN)](https://docs.nvidia.com/doca/sdk/nvidia+doca+hbn+service+guide/index.html).

## Prerequisites
The system is set up as described in the [system prerequisites](../../prerequisites.md).  The OVN Kubernetes with HBN use case has the additional requirements:

### Software prerequisites
This guide uses the following tools which must be installed where it is running.
- kubectl
- helm
- envsubst

### Network prerequisites
TODO: Clarify networking requirements.
For each worker node: DHCP allocation must be used for workers
- https://gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/-/blob/main/docs/provisioning/host-network-configuration-prerequisite.md#integrating-to-cloud-init
- https://gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/-/blob/157d962f65b29e703270a5a5ad448767bce3719d/docs/provisioning/default-route-through-high-speed-port-prerequisite.md#route-management-service

### Kubernetes prerequisites
- CNI not installed
- kube-proxy and coredns not installed
- control plane setup - including OVN Kubernetes CNI installation - is completed before adding worker nodes to the cluster
#### Control plane Nodes
- Have the labels:
  - `"k8s.ovn.org/zone-name": $KUBERNETES_NODE_NAME`

#### Worker Nodes
- Have the labels:
  - `"k8s.ovn.org/dpu-host": ""`
  - `"k8s.ovn.org/zone-name": $KUBERNETES_NODE_NAME`
- Have the annotations:
  - `"k8s.ovn.org/remote-zone-migrated": $KUBERNETES_NODE_NAME`
 
#### DPF installation
0. Variables
The following variables are required by this guide. A sensible default is provided where it makes sense, but many will be specific to the target infrastructure.

```bash
## Address for the Kubernetes API server of the target cluster on which DPF is installed.
## This should never include a scheme or a port.
## e.g. 10.10.10.10 or kube-vip.dpf.labs.internal
export TARGETCLUSTER_API_SERVER_HOST=

## Port for the Kubernetes API server of the target cluster on which DPF is installed.
export TARGETCLUSTER_API_SERVER_PORT=6443

## IP address range for hosts in the target cluster on which DPF is installed.
## This is a CIDR in the form e.g. 10.10.10.0/24
export TARGETCLUSTER_NODE_CIDR=

## Virtual IP used by the load balancer for the DPU Cluster. Must be a reserved IP from the management subnet and not allocated by DHCP.
export DPUCLUSTER_VIP=

## DPU_P0 is the name of the first port of the DPU. This name must be the same on all worker nodes.
## TODO: Add information in the hardware guide about how extract this name from a worker node.
export DPU_P0=

## DPU_P0_VF1 is the name of the second Virtual Function (VF) of the first port of the DPU. This name must be the same on all worker nodes.
export DPU_P0_VF1=

## Interface on which the DPUCluster load balancer will listen. Should be the management interface of the control plane node.
export DPUCLUSTER_INTERFACE=

# IP address to the NFS server used as storage for the BFB.
export NFS_SERVER_IP=

# API key for accessing containers and helm charts from the NGC private repository.
export NGC_API_KEY=

## POD_CIDR is the CIDR used for pods in the target Kubernetes cluster.
export POD_CIDR=10.233.64.0/18

## SERVICE_CIDR is the CIDR used for services in the target Kubernetes cluster.
## This is a CIDR in the form e.g. 10.10.10.0/24
export SERVICE_CIDR=10.233.0.0/18 

## DPF_VERSION is the version of the DPF components which will be deployed in this use case guide.
## This is a CIDR in the form e.g. 10.10.10.0/24
export DPF_VERSION=v24.10.0-rc.2

## URL to the BFB used in the `bfb.yaml` and linked by the DPUSet.
export BLUEFIELD_BITSTREAM="http://nbu-nfs.mellanox.com/auto/sw_mc_soc_release/doca_dpu/doca_2.9.0/20241103.1/bfbs/pk/bf-bundle-2.9.0-80_24.10_ubuntu-22.04_prod.bfb"

```

1. Install cert-manager
```shell
helm repo add jetstack https://charts.jetstack.io --force-update
helm upgrade --install --create-namespace --namespace cert-manager cert-manager jetstack/cert-manager --version v1.16.1 -f - <<EOF
startupapicheck:
  enabled: false
crds:
  enabled: true
tolerations:
   - operator: Exists
     effect: NoSchedule
     key: node-role.kubernetes.io/control-plane
   - operator: Exists
     effect: NoSchedule
     key: node-role.kubernetes.io/master
cainjector:
  tolerations:
  - operator: Exists
    effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
  - operator: Exists
    effect: NoSchedule
    key: node-role.kubernetes.io/master
webhook:
 tolerations:
 - operator: Exists
   effect: NoSchedule
   key: node-role.kubernetes.io/control-plane
 - operator: Exists
   effect: NoSchedule
   key: node-role.kubernetes.io/master
EOF
```

2. Install Multus and SRIOV Network Operator using NVIDIA Network Operator

```shell
helm repo add nvidia https://helm.ngc.nvidia.com/nvidia --force-update

helm upgrade --no-hooks --install --create-namespace --namespace nvidia-network-operator network-operator nvidia/network-operator --version 24.7.0 -f - <<EOF
nfd:
  enabled: false
nfd:
  deployNodeFeatureRules: false 
sriovNetworkOperator:
  enabled: true
sriov-network-operator:
  crds:
    enabled: true
  sriovOperatorConfig:
    deploy: true
    configDaemonNodeSelector: null
EOF
```

3. Log in to helm and docker registries.
```shell
kubectl apply -f manifests/prerequisites/namespace_dpf.yaml
kubectl -n dpf-operator-system create secret docker-registry dpf-pull-secret --docker-server=nvcr.io --docker-username="\$oauthtoken" --docker-password=$NGC_API_KEY
helm registry login nvcr.io --username \$oauthtoken --password $NGC_API_KEY
```

4. Create prerequisite objects for the installation.
```shell
cat manifests/prerequisites/*.yaml | envsubst | kubectl apply -f - 
```

#### OVN CNI installation
1. Create ovnk8s NS, secrets, and install its chart (host part). If the first run fails, rerun the command after a couple of seconds.
```shell
# The first VF (vf0) will be used by provisioning
# ovn-kube will use the second vf (vf1) and sriovdp CM should use from the third VF until the last
kubectl create -f manifests/dpuservice/namespace_ovn.yaml
kubectl -n ovn-kubernetes create secret docker-registry dpf-pull-secret --docker-server=nvcr.io --docker-username="\$oauthtoken" --docker-password=$NGC_API_KEY
kubectl -n ovn-kubernetes label secret dpf-pull-secret dpu.nvidia.com/image-pull-secret=""


helm upgrade --install -n ovn-kubernetes ovn-kubernetes oci://nvcr.io/nvstaging/doca/ovn-kubernetes-chart --version $DPF_VERSION -f - <<EOF
global:
  imagePullSecretName: "dpf-pull-secret" 
k8sAPIServer: https://$TARGETCLUSTER_API_SERVER_HOST:$TARGETCLUSTER_API_SERVER_PORT
ovnkube-node-dpu-host:
  nodeMgmtPortNetdev: $DPU_P0_VF1 
  gatewayOpts:
    - --gateway-interface=$DPU_P0
## Note this CIDR is followed by a trailing /24 which informs OVN Kubernetes on how to split the CIDR per node.
podNetwork: $POD_CIDR/24
serviceNetwork: $SERVICE_CIDR
ovn-kubernetes-resource-injector:
  resourceName: nvidia.com/bf3-p0-vfs 
dpuServiceAccountNamespace: dpf-operator-system
EOF
```

2. Wait until ovnkube-node pods under ovn-kubernetes NS are up, and install DPF Operator #Internal - first remove all content from NFS folders bfb and pvX
```shell
helm upgrade --install -n dpf-operator-system dpf-operator oci://nvcr.io/nvstaging/doca/dpf-operator --version=$DPF_VERSION -f - <<EOF
imagePullSecrets:
  - name: dpf-pull-secret
kamaji-etcd: 
  persistentVolumeClaim:
    storageClassName: kamaji-storage
node-feature-discovery:
  worker:
    extraEnvs: 
      - name: "KUBERNETES_SERVICE_HOST"
        value: $TARGETCLUSTER_API_SERVER_HOST
      - name: "KUBERNETES_SERVICE_PORT"
        value: $TARGETCLUSTER_API_SERVER_PORT
EOF
```


### Deploy the DPF System
```shell
kubectl create ns dpu-cplane-tenant1
cat manifests/dpf-system/*.yaml | envsubst | kubectl apply -f - 
```

### Deploy the OVN and HBN DPUServices
```shell
cat manifests/dpuservice/*.yaml | envsubst | kubectl apply -f - 
```

### Testing traffic with 
1. Run traffic
```shell
kubectl apply -f manifests/traffic
```

# Bugs:
1. The route management script needs rewrite as it's not stable. If you see 3 default routes, delete the one with metric
