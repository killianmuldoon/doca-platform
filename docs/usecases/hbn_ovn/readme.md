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
The following variables are required by this guide. A sensible default is provided where it makes sense, but many will be specific to the target infrastructure.

Commands in this guide are run in the same directory that contains this readme.

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

This deploys the following objects:
<details><summary>Namespace for the DPF system</summary>

[embedmd]:#(manifests/prerequisites/namespace_dpf.yaml)
```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: dpf-operator-system
```
</details>

<details><summary>Secret for pulling images and helm charts</summary>

[embedmd]:#(manifests/prerequisites/helm-secret-dpf.yaml)
```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: ngc-doca-oci-helm
  namespace: dpf-operator-system
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  name: nvstaging-doca-oci
  url: nvcr.io/nvstaging/doca
  type: helm
  ## Note `no_variable` here is used to ensure envsubst renders the correct username which is `$oauthtoken`
  username: $${no_variable}oauthtoken
  password: $NGC_API_KEY
---
apiVersion: v1
kind: Secret
metadata:
  name: ngc-doca-https-helm
  namespace: dpf-operator-system
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  name: nvstaging-doca-https
  url: https://helm.ngc.nvidia.com/nvstaging/doca
  type: helm
  username: $${no_variable}oauthtoken
  password: $NGC_API_KEY
```
</details>

<details><summary>StorageClass and PersistentVolumes for the DPUCluster etcd</summary>

[embedmd]:#(manifests/prerequisites/nfs-storage-for-kamaji.yaml)
```yaml
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: kamaji-storage
provisioner: kubernetes.io/no-provisioner
reclaimPolicy: Retain
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: kamaji-pv-1
spec:
  capacity:
    storage: 10Gi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: kamaji-storage
  nfs:
    path: /mnt/dpf_share/pv1
    server: $NFS_SERVER_IP
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: kamaji-pv-2
spec:
  capacity:
    storage: 10Gi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: kamaji-storage
  nfs:
    path: /mnt/dpf_share/pv2
    server: $NFS_SERVER_IP
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: kamaji-pv-3
spec:
  capacity:
    storage: 10Gi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: kamaji-storage
  nfs:
    path: /mnt/dpf_share/pv3
    server: $NFS_SERVER_IP
```
</details>

<details><summary>PersistentVolume and PersistentVolumeClaim for the provisioning controller</summary>

[embedmd]:#(manifests/prerequisites/nfs-storage-for-bfb-dpf-ga.yaml)
```yaml
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: bfb-pv
spec:
  capacity:
    storage: 10Gi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteMany
  nfs: 
    path: /mnt/dpf_share/bfb
    server: $NFS_SERVER_IP
  persistentVolumeReclaimPolicy: Retain
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: bfb-pvc
  namespace: dpf-operator-system
spec:
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
  volumeMode: Filesystem
```
</details>

<details><summary>NICClusterPolicy for the NVIDIA Network Operator etcd</summary>

[embedmd]:#(manifests/prerequisites/nic_cluster_policy.yaml)
```yaml
---
apiVersion: mellanox.com/v1alpha1
kind: NicClusterPolicy
metadata:
  name: nic-cluster-policy
spec:
  secondaryNetwork:
    multus:
      image: multus-cni
      imagePullSecrets: []
      repository: ghcr.io/k8snetworkplumbingwg
      version: v3.9.3
```
</details>

<details><summary>SriovNetworkNodePolicy for the SR-IOV Network Operator</summary>

[embedmd]:#(manifests/prerequisites/sriov_network_operator_policy.yaml)
```yaml
---
apiVersion: sriovnetwork.openshift.io/v1
kind: SriovNetworkNodePolicy
metadata:
  name: bf3-p0-vfs
  namespace: nvidia-network-operator
spec:
  mtu: 1500
  nicSelector:
    deviceID: "a2dc"
    vendor: "15b3"
    pfNames:
    # TODO: Will this work based on naming conventions?
    - $DPU_P0#2-45
  nodeSelector:
    node-role.kubernetes.io/worker: ""
  numVfs: 46
  resourceName: bf3-p0-vfs
  isRdma: true
  externallyManaged: true
  deviceType: netdevice
  linkType: eth

```
</details>

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
        value: "$TARGETCLUSTER_API_SERVER_HOST"
      - name: "KUBERNETES_SERVICE_PORT"
        value: "$TARGETCLUSTER_API_SERVER_PORT"
EOF
```

### DPF System installation
```shell
kubectl create ns dpu-cplane-tenant1
cat manifests/dpf-system/*.yaml | envsubst | kubectl apply -f - 
```

This will create the following objects:
<details><summary>DPF Operator to install the DPF System components</summary>

[embedmd]:#(manifests/dpf-system/operatorconfig.yaml)
```yaml
---
apiVersion: operator.dpu.nvidia.com/v1alpha1
kind: DPFOperatorConfig
metadata:
  name: dpfoperatorconfig
  namespace: dpf-operator-system
spec:
  imagePullSecrets:
  - dpf-pull-secret
  provisioningController:
    bfbPVCName: "bfb-pvc"
    dmsTimeout: 900
  kamajiClusterManager:
    disable: false
```

</details>

<details><summary>BFB to download Bluefield Bitstream to a shared volume</summary>

[embedmd]:#(manifests/dpf-system/bfb.yaml)
```yaml
---
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: BFB
metadata:
  name: bf-bundle
  namespace: dpf-operator-system
spec:
  fileName: "bf-bundle-2.9.0-80.bfb"
  url: $BLUEFIELD_BITSTREAM
```
</details>
<details><summary>DPUCluster to serve as Kubernetes control plane for DPU nodes</summary>

[embedmd]:#(manifests/dpf-system/dpucluster.yaml)
```yaml
---
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUCluster
metadata:
  name: dpu-cplane-tenant1
  namespace: dpu-cplane-tenant1
spec:
  type: kamaji
  maxNodes: 10
  version: v1.30.2
  clusterEndpoint:
    # deploy keepalived instances on the nodes that match the given nodeSelector.
    keepalived:
      # interface on which keepalived will listen. Should be the oob interface of the control plane node.
      interface: $DPUCLUSTER_INTERFACE
      # Virtual IP reserved for the DPU Cluster load balancer. Must not be allocatable by DHCP.
      vip: $DPUCLUSTER_VIP
      # virtualRouterID must be in range [1,255], make sure the given virtualRouterID does not duplicate with any existing keepalived process running on the host
      virtualRouterID: 126
      nodeSelector:
        node-role.kubernetes.io/control-plane: ""
```
</details>


### OVN Kubernetes and HBN DPUService installation
```shell
cat manifests/dpuservice/*.yaml | envsubst | kubectl apply -f - 
```

This will deploy the following objects:

<details><summary>DPUSet to provision DPUs on worker nodes</summary>

[embedmd]:#(manifests/dpuservice/dpuset.yaml)
```yaml
---
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUSet
metadata:
  name: dpuset
  namespace: dpf-operator-system
spec:
  nodeSelector:
    matchLabels:
      feature.node.kubernetes.io/dpu-enabled: "true"
  strategy:
    rollingUpdate:
      maxUnavailable: "10%"
    type: RollingUpdate
  dpuTemplate:
    spec:
      dpuFlavor: dpf-provisioning-hbn-ovn
      bfb:
        name: bf-bundle
      nodeEffect:
        taint:
          key: "dpu"
          value: "provisioning"
          effect: NoSchedule
      automaticNodeReboot: true
```
</details>

<details><summary>OVN DPUService to deploy OVN workloads to the DPUs</summary>

[embedmd]:#(manifests/dpuservice/ovn-dpuservice.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUService
metadata:
  name: ovn-dpu
  namespace: dpf-operator-system 
spec:
  helmChart:
    source:
      repoURL: oci://nvcr.io/nvstaging/doca
      chart: ovn-kubernetes-chart
      version: $DPF_VERSION
    values:
      tags:
        ovn-kubernetes-resource-injector: false
        ovnkube-node-dpu: true
        ovnkube-node-dpu-host: false
        ovnkube-single-node-zone: false
        ovnkube-control-plane: false
      k8sAPIServer: https://$TARGETCLUSTER_API_SERVER_HOST:$TARGETCLUSTER_API_SERVER_PORT
      podNetwork: $POD_CIDR/24
      serviceNetwork: $SERVICE_CIDR
      global:
        gatewayOpts: "--gateway-interface=br-ovn --gateway-uplink-port=puplinkbrovn"
        imagePullSecretName: dpf-pull-secret
      ovnkube-node-dpu:
        kubernetesSecretName: "ovn-dpu" # user needs to populate based on DPUServiceCredentialRequest
        vtepCIDR: "10.0.120.0/22" # user needs to populate based on DPUServiceIPAM
        hostCIDR: $TARGETCLUSTER_NODE_CIDR
        ipamPool: "pool1" # user needs to populate based on DPUServiceIPAM
        ipamPoolType: "cidrpool" # user needs to populate based on DPUServiceIPAM
        ipamVTEPIPIndex: 0
        ipamPFIPIndex: 1
```

</details>

<details><summary>HBN DPUService to deploy HBN workloads to the DPUs</summary>

[embedmd]:#(manifests/dpuservice/hbn-dpuservice.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUService
metadata:
  name: doca-hbn
  namespace: dpf-operator-system
spec:
  serviceID: doca-hbn
  interfaces:
  - p0-sf
  - p1-sf
  - app-sf
  serviceDaemonSet:
    annotations:
      k8s.v1.cni.cncf.io/networks: |-
        [
        {"name": "iprequest", "interface": "ip_lo", "cni-args": {"poolNames": ["loopback"], "poolType": "cidrpool"}},
        {"name": "iprequest", "interface": "ip_pf2dpu2", "cni-args": {"poolNames": ["pool1"], "poolType": "cidrpool", "allocateDefaultGateway": true}}
        ]
  helmChart:
    source:
      repoURL: https://helm.ngc.nvidia.com/nvstaging/doca
      version: 1.0.0-dev.1
      chart: doca-hbn
    values:
      imagePullSecrets:
      - name: dpf-pull-secret
      image:
        repository: nvcr.io/nvstaging/doca/doca_hbn
        tag: 2.dev.200-doca2.9.0
      resources:
        memory: 6Gi
        nvidia.com/bf_sf: 3
      configuration:
        perDPUValuesYAML: |
          - hostnamePattern: "*"
            values:
              bgp_autonomous_system: 65111
              bgp_peer_group: hbn
        startupYAMLJ2: |
          - header:
              model: BLUEFIELD
              nvue-api-version: nvue_v1
              rev-id: 1.0
              version: HBN 2.4.0
          - set:
              interface:
                lo:
                  ip:
                    address:
                      {{ ipaddresses.ip_lo.ip }}/32: {}
                  type: loopback
                p0_if,p1_if:
                  type: swp
                  link:
                    mtu: 9000
                pf2dpu2_if:
                  ip:
                    address:
                      {{ ipaddresses.ip_pf2dpu2.cidr }}: {}
                  type: swp
                  link:
                    mtu: 9000
              router:
                bgp:
                  autonomous-system: {{ config.bgp_autonomous_system }}
                  enable: on
                  graceful-restart:
                    mode: full
                  router-id: {{ ipaddresses.ip_lo.ip }}
              vrf:
                default:
                  router:
                    bgp:
                      address-family:
                        ipv4-unicast:
                          enable: on
                          redistribute:
                            connected:
                              enable: on
                        ipv6-unicast:
                          enable: on
                          redistribute:
                            connected:
                              enable: on
                      enable: on
                      neighbor:
                        p0_if:
                          peer-group: {{ config.bgp_peer_group }}
                          type: unnumbered
                        p1_if:
                          peer-group: {{ config.bgp_peer_group }}
                          type: unnumbered
                      path-selection:
                        multipath:
                          aspath-ignore: on
                      peer-group:
                        {{ config.bgp_peer_group }}:
                          remote-as: external
```
</details>

<details><summary>DOCA Telemetry Service DPUService to deploy DTS to the DPUs</summary>

[embedmd]:#(manifests/dpuservice/dts-dpuservice.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUService
metadata:
  name: doca-telemetry-service
  namespace: dpf-operator-system
spec:
  helmChart:
    source:
      repoURL: https://helm.ngc.nvidia.com/nvstaging/doca
      version: 0.2.2
      chart: doca-telemetry
```
</details>

<details><summary>Blueman DPUService to deploy Blueman to the DPUs</summary>

[embedmd]:#(manifests/dpuservice/blueman-dpuservice.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUService
metadata:
  name: doca-blueman-service
  namespace: dpf-operator-system
spec:
  helmChart:
    source:
      repoURL: https://helm.ngc.nvidia.com/nvstaging/doca
      version: 1.0.3
      chart: doca-blueman
    values:
      imagePullSecrets:
      - name: dpf-pull-secret
```
</details>

<details><summary>OVN DPUServiceCredentialRequest to allow cross cluster communication</summary>

[embedmd]:#(manifests/dpuservice/ovn-credentials.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceCredentialRequest
metadata:
  name: ovn-dpu
  namespace: dpf-operator-system 
spec:
  serviceAccount:
    name: ovn-dpu
    namespace: dpf-operator-system 
  duration: 24h
  type: tokenFile
  secret:
    name: ovn-dpu
    namespace: dpf-operator-system 
  metadata:
    labels:
      dpu.nvidia.com/image-pull-secret: ""
```
</details>

<details><summary>DPUServiceInterfaces for physical ports on the DPU</summary>

[embedmd]:#(manifests/dpuservice/physical-ifaces.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceInterface
metadata:
  name: p0
  namespace: dpf-operator-system
spec:
  template:
    spec:
      template:
        metadata:
          labels:
            uplink: "p0"
        spec:
          interfaceType: physical
          physical:
            interfaceName: p0
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceInterface
metadata:
  name: p1
  namespace: dpf-operator-system
spec:
  template:
    spec:
      template:
        metadata:
          labels:
            uplink: "p1"
        spec:
          interfaceType: physical
          physical:
            interfaceName: p1
```
</details>

<details><summary>OVN DPUServiceInterface to define the ports attached to OVN workloads on the DPU</summary>

[embedmd]:#(manifests/dpuservice/ovn-iface.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceInterface
metadata:
  name: ovn
  namespace: dpf-operator-system
spec:
  template:
    spec:
      template:
        metadata:
          labels:
            port: ovn
        spec:
          interfaceType: ovn
```
</details>


<details><summary>HBN DPUServiceInterfaces to define the ports attached to HBN workloads on the DPU</summary>

[embedmd]:#(manifests/dpuservice/hbn-ifaces.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceInterface
metadata:
  name: app-sf 
  namespace: dpf-operator-system
spec:
  template:
    spec:
      template:
        metadata:
          labels:
            svc.dpu.nvidia.com/interface: "app_sf"
            svc.dpu.nvidia.com/service: doca-hbn
        spec:
          interfaceType: service
          service:
            serviceID: doca-hbn
            network: mybrhbn
            interfaceName: pf2dpu2_if
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceInterface
metadata:
  name: p0-sf
  namespace: dpf-operator-system
spec:
  template:
    spec:
      template:
        metadata:
          labels:
            svc.dpu.nvidia.com/interface: "p0_sf"
            svc.dpu.nvidia.com/service: doca-hbn
        spec:
          interfaceType: service
          service:
            serviceID: doca-hbn
            network: mybrhbn
            interfaceName: p0_if
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceInterface
metadata:
  name: p1-sf
  namespace: dpf-operator-system
spec:
  template:
    spec:
      template:
        metadata:
          labels:
            svc.dpu.nvidia.com/interface: "p1_sf"
            svc.dpu.nvidia.com/service: doca-hbn
        spec:
          interfaceType: service
          service:
            serviceID: doca-hbn
            network: mybrhbn
            interfaceName: p1_if
```
</details>

<details><summary>DPUServiceFunctionChain to define the HBN-OVN ServiceFunctionChain</summary>

[embedmd]:#(manifests/dpuservice/hbn-ovn-chain.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceChain
metadata:
  name: hbn-to-fabric
  namespace: dpf-operator-system
spec:
  template:
    spec:
      template:
        spec:
          switches:
            - ports:
              - serviceInterface:
                  matchLabels:
                    uplink: p0
              - serviceInterface:
                  matchLabels:
                    svc.dpu.nvidia.com/service: doca-hbn
                    svc.dpu.nvidia.com/interface: "p0_sf"
            - ports:
              - serviceInterface:
                  matchLabels:
                    uplink: p1
              - serviceInterface:
                  matchLabels:
                    svc.dpu.nvidia.com/service: doca-hbn
                    svc.dpu.nvidia.com/interface: "p1_sf"
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceChain
metadata:
  name: ovn-to-hbn
  namespace: dpf-operator-system
spec:
  template:
    spec:
      template:
        spec:
          switches:
            - ports:
              - serviceInterface:
                  matchLabels:
                    svc.dpu.nvidia.com/service: doca-hbn
                    svc.dpu.nvidia.com/interface: "app_sf"
              - serviceInterface:
                  matchLabels:
                    port: ovn
```
</details>

<details><summary>DPUServiceIPAM to set up IP Address Management on the DPUCluster</summary>

[embedmd]:#(manifests/dpuservice/hbn-ovn-ipam.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceIPAM
metadata:
  name: pool1
  namespace: dpf-operator-system
spec:
  ipv4Network:
    network: "10.0.120.0/22"
    gatewayIndex: 3
    prefixSize: 29
```
</details>

<details><summary>DPUServiceIPAM for the loopback interface in HBN</summary>

[embedmd]:#(manifests/dpuservice/hbn-loopback-ipam.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceIPAM
metadata:
  name: loopback
  namespace: dpf-operator-system
spec:
  ipv4Network:
    network: "11.0.0.0/24"
    prefixSize: 32
```
</details>


### Testing traffic with 
1. Run traffic
```shell
kubectl apply -f manifests/traffic
```

# Bugs:
1. The route management script needs rewrite as it's not stable. If you see 3 default routes, delete the one with metric
