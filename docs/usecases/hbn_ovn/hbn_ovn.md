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
## URL to the BFB used in the `bfb.yaml` and linked by the DPUSet.
export BLUEFIELD_BITSTREAM="http://nbu-nfs.mellanox.com/auto/sw_mc_soc_release/doca_dpu/doca_2.9.0/20241103.1/bfbs/pk/bf-bundle-2.9.0-80_24.10_ubuntu-22.04_prod.bfb"

## Virtual IP used by the load balancer for the DPU Cluster. Must be a reserved IP from the management subnet and not allocated by DHCP.
export DPUCLUSTER_VIP=

## Interface on which the DPUCluster load balancer will listen. Should be the management interface of the control plane node.
export DPUCLUSTER_INTERFACE=

# IP address to the NFS server used as storage for the BFB.
export NFS_SERVER_IP=

# API key for accessing containers and helm charts from the NGC private repository.
export NGC_API_KEY=

# Name of the P0 physical function of the DPU.
export BF3_PF_NAME=
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
helm repo add nvidia https://charts.jetstack.io --force-update

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
kubectl -n dpf-operator-system create secret docker-registry dpf-pull-secret --docker-server=nvcr.io --docker-username="\$oauthtoken" --docker-password=$NGC_API_KEY
helm registry login nvcr.io --username \$oauthtoken --password $NGC_API_KEY
```

4. Create prerequisite objects for the installation.
```shell
cat manifests/<install_variant>/*.yaml | envsubst - | kubectl apply -f - 
```

#### OVN CNI installation
1. Create ovnk8s NS, secrets, and install its chart (host part). If the first run fails, rerun the command after a couple of seconds.
```shell
# The first VF (vf0) will be used by provisioning
# ovn-kube will use the second vf (vf1) and sriovdp CM should use from the third VF until the last
kubectl create -f manifests/dpuservice/namespace_ovn.yaml
kubectl -n ovn-kubernetes create secret docker-registry dpf-pull-secret --docker-server=nvcr.io --docker-username="\$oauthtoken" --docker-password=$NGC_API_KEY
kubectl -n ovn-kubernetes label secret dpf-pull-secret dpu.nvidia.com/image-pull-secret=""


helm upgrade --install -n ovn-kubernetes ovn-kubernetes oci://nvcr.io/nvstaging/doca/ovn-kubernetes-chart --version v24.10.0-rc.0 -f - <<EOF
global:
  imagePullSecretName: "dpf-pull-secret" 
k8sAPIServer: https://kube-vip.dpf.clx.labs.mlnx:6443
ovnkube-node-dpu-host:
    ## TODO: What are the IP addresses and interfaces here?
  nodeMgmtPortNetdev: ns1f0v1 
  gatewayOpts:
    - --gateway-interface=ens1f0np0 
podNetwork: 10.233.64.0/18/24
serviceNetwork: 10.233.0.0/18 
ovn-kubernetes-resource-injector:
  resourceName: nvidia.com/bf3-p0-vfs 
dpuServiceAccountNamespace: dpf-operator-system
EOF
```

2. Wait until ovnkube-node pods under ovn-kubernetes NS are up, and install DPF Operator #Internal - first remove all content from NFS folders bfb and pvX
```shell
helm upgrade --install -n dpf-operator-system dpf-operator oci://nvcr.io/nvstaging/doca/dpf-operator --version=v24.10.0-rc.0 -f - <<EOF
imagePullSecrets:
  - name=dpf-pull-secret
kamaji-etcd: 
  persistentVolumeClaim:
    storageClassName: 
      kamaji-storage
node-feature-discovery:
  worker:
    extraEnvs: 
      - name:"KUBERNETES_SERVICE_HOST"
        ## TODO: What is this IP address?
        value:"10.0.110.10"
      - name:"KUBERNETES_SERVICE_PORT"
        "value":"6443"
EOF
```


### Worker node configuration
```shell
kubectl create ns dpu-cplane-tenant1
cat manifests/dpf-system/*.yaml | envsubst - | kubectl apply -f - 
// TODO: Find a cleaner way of differentiating between the install variants.
cat manifests/<install_variant>/*.yaml | envsubst - | kubectl apply -f - 
```

### Testing traffic with 
1. Run traffic
```shell
kubectl apply -f manifests/traffic
```

# Bugs:
1. The route management script needs rewrite as it's not stable. If you see 3 default routes, delete the one with metric
