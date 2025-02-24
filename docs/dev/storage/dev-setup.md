# Dev setup for DPF storage subsystem

This guide provides instructions for configuring the developer setup for the DPF storage subsystem using DPF-standalone. 
This guide will help you set up an all-in-one single node setup with no external dependencies. The SPDK storage target will run as part of the host OS.

## Prerequisites
-	Host with Ubuntu 24.04 and BF3 card.
-	Host should be provisioned by dpf-standalone (DPU should not be provisioned at this stage)

## Run SDPK on the host

### Build SPDK from sources

```
git clone https://github.com/spdk/spdk
cd spdk

# v24.01 is the last version that is compatible with the spdk-csi
git checkout v24.01
git submodule update --init
apt update && apt install meson python3-pyelftools -y
./scripts/pkgdep.sh --rdma
./configure --with-rdma
make
```

### Run SPDK target

Notes: 
* you need run these steps after each reboot
* the IP address `10.33.33.33` is a static IP address that is available in each dpf-standalone env
* `exampleuser` and `examplepassword` must be aligned with the password defined in the `manifests/spdk-csi/secret.yaml`


```
# dir with the spdk repo from the previous step
cd spdk
env PCI_ALLOWED="none" ./scripts/setup.sh
./build/bin/nvmf_tgt --no-pci

# in a separate window
cd spdk
fallocate -l 10G test1_store
./scripts/rpc.py bdev_aio_create test1_store test1 512
./scripts/rpc.py bdev_lvol_create_lvstore test1 test1

# in a separate window
cd spdk
./scripts/rpc_http_proxy.py 10.33.33.33 8000 exampleuser examplepassword

```

### Configure PF1 on the host

The PF1 on the host should be configured with IP address 10.44.44.100 (aligned with the config of DPUServiceIPAM from this doc)

Note: you need to do this after each reboot

```
PF1_NAME=enp177s0f1np1
ip add add 10.44.44.100/24 dev $PF1_NAME
ip link set dev $PF1_NAME up

```

## Deploy

The instruction from the guide should be run on the host where kubectl is configured to access the cluster API (of the host clustser) of the DPF-standalone setup.

### Required variables
The following variables are required by this guide. A sensible default is provided where it makes sense, but many will be specific to the target infrastructure.

Commands in this guide are run in the same directory that contains this readme.

```bash

## DPF_VERSION is the version of the DPF components which will be deployed in this guide.
# e.g. v25.4.0-c780744e
export DPF_VERSION=<set latest tag here> 

## DPF_REGISTRY is the registry address that holds DPF images
export DPF_REGISTRY=nvcr.io/nvstaging/doca

## DPF_IMAGE_PULL_SECRET, default values is to use images from nvstaging
export DPF_IMAGE_PULL_SECRET='[{"name": "dpf-pull-secret"}]'
# use '[]' to pull images from the registry that doesn't require auth
# export DPF_IMAGE_PULL_SECRET='[]'

## URL to the BFB used in the `bfb.yaml` and linked by the DPUSet.
export BLUEFIELD_BITSTREAM="http://nbu-nfs.mellanox.com/auto/sw_mc_soc_release/doca_dpu/doca_2.9.1/FUR_20241229/bfbs/pk/bf-bundle-2.9.1-50_dec-fur_24.11-ubuntu-22.04_prod.bfb"
```

### Provision DPU with the storage flavor

A number of [environment variables](#required-variables) must be set before running this command.

```shell
cat manifests/provisioning/*.yaml | envsubst | kubectl apply -f - 
```

This will deploy the following objects:

<details><summary>BFB to download Bluefield Bitstream to a shared volume</summary>

[embedmd]:#(manifests/provisioning/bfb.yaml)
```yaml
---
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: BFB
metadata:
  name: bf-bundle
  namespace: dpf-operator-system
spec:
  url: $BLUEFIELD_BITSTREAM
```
</details>

<details><summary>DPUFlavor with storage config</summary>

[embedmd]:#(manifests/provisioning/dpuflavor.yaml)
```yaml
---
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUFlavor
metadata:
  name: dpf-provisioning-hbn-ovn-storage
  namespace: dpf-operator-system
spec:
  bfcfgParameters:
  - UPDATE_ATF_UEFI=yes
  - UPDATE_DPU_OS=yes
  - WITH_NIC_FW_UPDATE=yes
  configFiles:
  - operation: override
    path: /etc/mellanox/mlnx-bf.conf
    permissions: "0644"
    raw: |
      ALLOW_SHARED_RQ="no"
      IPSEC_FULL_OFFLOAD="no"
      ENABLE_ESWITCH_MULTIPORT="yes"
  - operation: override
    path: /etc/mellanox/mlnx-ovs.conf
    permissions: "0644"
    raw: |
      CREATE_OVS_BRIDGES="no"
  - operation: override
    path: /etc/mellanox/mlnx-sf.conf
    permissions: "0644"
    raw: ""
  - operation: override
    path: /etc/nvda_snap/snap_rpc_init.conf
    permissions: "0644"
    raw: |
      nvme_subsystem_create --nqn nqn.2022-10.io.nvda.nvme:0
      nvme_controller_create --nqn nqn.2022-10.io.nvda.nvme:0 --ctrl NVMeCtrl1 --pf_id 0 --admin_only
  grub:
    kernelParameters:
    - console=hvc0
    - console=ttyAMA0
    - earlycon=pl011,0x13010000
    - fixrttc
    - net.ifnames=0
    - biosdevname=0
    - iommu.passthrough=1
    - cgroup_no_v1=net_prio,net_cls
    - hugepagesz=2048kB
    - hugepages=5120
  nvconfig:
  - device: '*'
    parameters:
    - PF_BAR2_ENABLE=0
    - PER_PF_NUM_SF=1
    - PF_TOTAL_SF=20
    - PF_SF_BAR_SIZE=10
    - NUM_PF_MSIX_VALID=0
    - PF_NUM_PF_MSIX_VALID=1
    - PF_NUM_PF_MSIX=228
    - INTERNAL_CPU_MODEL=1
    - INTERNAL_CPU_OFFLOAD_ENGINE=0
    - SRIOV_EN=1
    - NUM_OF_VFS=46
    - LAG_RESOURCE_ALLOCATION=1
    - NVME_EMULATION_ENABLE=1
    - NVME_EMULATION_NUM_PF=1
    - NVME_EMULATION_NUM_VF=125
    - NVME_EMULATION_NUM_MSIX=2
  ovs:
    rawConfigScript: |
      # temporary hack to switch RDMA netns mode to shared
      sed -i \
        -e 's/rdma system show netns 2>\&1 | grep -q exclusive/rdma system show netns 2>\&1 | grep -q shared/g' \
        -e 's/rdma system set netns exclusive/rdma system set netns shared/g' \
        /usr/sbin/mlnx_bf_configure
      
      _ovs-vsctl() {
        ovs-vsctl --no-wait --timeout 15 "$@"
      }

      _ovs-vsctl set Open_vSwitch . other_config:doca-init=true
      _ovs-vsctl set Open_vSwitch . other_config:dpdk-max-memzones=50000
      _ovs-vsctl set Open_vSwitch . other_config:hw-offload=true
      _ovs-vsctl set Open_vSwitch . other_config:pmd-quiet-idle=true
      _ovs-vsctl set Open_vSwitch . other_config:max-idle=20000
      _ovs-vsctl set Open_vSwitch . other_config:max-revalidator=5000
      _ovs-vsctl set Open_vSwitch . other_config:ctl-pipe-size=1024
      _ovs-vsctl --if-exists del-br ovsbr1
      _ovs-vsctl --if-exists del-br ovsbr2
      _ovs-vsctl --may-exist add-br br-sfc
      _ovs-vsctl set bridge br-sfc datapath_type=netdev
      _ovs-vsctl set bridge br-sfc fail_mode=secure
      _ovs-vsctl --may-exist add-port br-sfc p0
      _ovs-vsctl set Interface p0 type=dpdk
      _ovs-vsctl set Port p0 external_ids:dpf-type=physical

      _ovs-vsctl set Open_vSwitch . external-ids:ovn-bridge-datapath-type=netdev
      _ovs-vsctl --may-exist add-br br-ovn
      _ovs-vsctl set bridge br-ovn datapath_type=netdev
      _ovs-vsctl --may-exist add-port br-ovn pf0hpf
      _ovs-vsctl set Interface pf0hpf type=dpdk
```
</details>


<details><summary>DPUSet</summary>

[embedmd]:#(manifests/provisioning/dpuset.yaml)
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
      dpuFlavor: dpf-provisioning-hbn-ovn-storage
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

### Deploy SPDK CSI

```shell
cat manifests/spdk-csi/*.yaml | envsubst | kubectl apply -f - 
```

This will deploy the following objects:

<details><summary>DPUService for host cluster</summary>

[embedmd]:#(manifests/spdk-csi/DPUService.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUService
metadata:
  name: spdk-csi-controller
  namespace: dpf-operator-system
spec:
  serviceID: spdk-csi-controller
  deployInCluster: true
  helmChart:
    source:
      repoURL: oci://nvcr.io/nvstaging/doca
      version: v0.1.0
      chart: spdk-csi-controller-chart
    values:
      host:
        enabled: true
        imagePullSecrets:
          - name: dpf-pull-secret
        plugin:
          image:
            repository: nvcr.io/nvstaging/doca/spdk-csi
            tag: v0.1.0
        config:
          targets:
            nodes:
              # name of the target
              - name: spdk-target
                # management address
                rpcURL: http://10.33.33.33:8000
                # type of the target, e.g. nvme-tcp, nvme-rdma
                targetType: nvme-tcp
                # target service IP
                targetAddr: 10.44.44.100
          # required parameter, name of the secret that contains connection
          # details to access the DPU cluster.
          # this secret should be created by the DPUServiceCredentialRequest API.
          dpuClusterSecret: spdk-csi-controller-dpu-cluster-credentials
```
</details>


<details><summary>DPUService for dpu cluster</summary>

[embedmd]:#(manifests/spdk-csi/DPUService-dpu.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUService
metadata:
  name: spdk-csi-controller-dpu
  namespace: dpf-operator-system
spec:
  serviceID: spdk-csi-controller-dpu
  helmChart:
    source:
      repoURL: oci://nvcr.io/nvstaging/doca
      version: v0.1.0
      chart: spdk-csi-controller-chart
    values:
      dpu:
        enabled: true
        storageClass:
          # the name of the storage class that will be created for spdk-csi,
          # this StorageClass name should be used in the StorageVendor settings
          name: spdkcsi-sc
          # name of the secret that contains credentials for the remote SPDK target,
          # content of the secret is injected during CreateVolume request
          secretName: spdkcsi-secret
          # namespace of the secret with credentials for the remote SPDK target
          secretNamespace: dpf-operator-system
        rbacRoles:
          spdkCsiController:
            # the name of the service account for spdk-csi-controller
            # this value must be aligned with the value from the DPUServiceCredentialRequest
            serviceAccount: spdk-csi-controller-sa
```
</details>

<details><summary>DPUServiceCredentialRequest</summary>

[embedmd]:#(manifests/spdk-csi/DPUServiceCredentialRequest.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceCredentialRequest
metadata:
  name: spdk-csi-controller-credentials
  namespace: dpf-operator-system
spec:
  duration: 10m
  serviceAccount:
    name: spdk-csi-controller-sa
    namespace: dpf-operator-system
  targetCluster:
    name: dpu-cplane-tenant1
    namespace: dpu-cplane-tenant1
  type: tokenFile
  secret:
    name: spdk-csi-controller-dpu-cluster-credentials
    namespace: dpf-operator-system
```
</details>

<details><summary>Secret</summary>

[embedmd]:#(manifests/spdk-csi/secret.yaml)
```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: spdkcsi-secret
  namespace: dpf-operator-system
  labels:
    # this label enables replication of the secret from the host to the dpu cluster
    dpu.nvidia.com/image-pull-secret: ""
stringData:
  # name field in the "rpcTokens" list should match name of the
  # spdk target from DPUService.helmChart.values.host.config.targets.nodes
  secret.json: |-
    {
      "rpcTokens": [
        {
          "name": "spdk-target",
          "username": "exampleuser",
          "password": "examplepassword"
        }
      ]
    }
```
</details>


### Configure service chain

Host's PF1 will be configured to be part of the same chain as the SNAP service. 
This configuration enables the deployment of the SPDK target on the host and the consumption of volumes from it on the DPU. 
This setup facilitates testing the storage solution end-to-end on a single physical machine. 
The SNAP service is also connected to PF1 on the DPU, allowing it to also reach targets that are running on a different machine and reachable from PF1.

```shell
cat manifests/network/*.yaml | envsubst | kubectl apply -f -
```

This will deploy the following objects:

<details><summary>DPUServiceChain</summary>

[embedmd]:#(manifests/network/DPUServiceChain.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceChain
metadata:
  name: spdk-chain
  namespace: dpf-operator-system
spec:
  template:
    spec:
      nodeSelector:
        matchExpressions:
          - key: kubernetes.io/os
            operator: In
            values:
              - "linux"
      template:
        spec:
          switches:
            - ports:
                - serviceInterface:
                    matchLabels:
                      uplink: p1
                - serviceInterface:
                    matchLabels:
                      uplink: pf1hpf
                - serviceInterface:
                    matchLabels:
                      svc.dpu.nvidia.com/service: snap-dpu
                      svc.dpu.nvidia.com/interface: app_sf
                    ipam:
                      matchLabels:
                        svc.dpu.nvidia.com/pool: spdk-pool
```

</details>

<details><summary>DPUServiceInterface-p1</summary>

[embedmd]:#(manifests/network/DPUServiceInterface-p1.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceInterface
metadata:
  name: p1
  namespace: dpf-operator-system
spec:
  template:
    spec:
      nodeSelector:
        matchExpressions:
          - key: kubernetes.io/os
            operator: In
            values:
              - "linux"
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

<details><summary>DPUServiceInterface-pf1hpf</summary>

[embedmd]:#(manifests/network/DPUServiceInterface-pf1hpf.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceInterface
metadata:
  name: pf1hpf
  namespace: dpf-operator-system
spec:
  template:
    spec:
      nodeSelector:
        matchExpressions:
          - key: kubernetes.io/os
            operator: In
            values:
              - "linux"
      template:
        metadata:
          labels:
            uplink: "pf1hpf"
        spec:
          interfaceType: pf
          pf:
            pfID: 1
```

</details>

<details><summary>DPUServiceInterface-app_sf</summary>

[embedmd]:#(manifests/network/DPUServiceInterface-app_sf.yaml)
```yaml
---
apiVersion: "svc.dpu.nvidia.com/v1alpha1"
kind: DPUServiceInterface
metadata:
  name: snap-sf
  namespace: dpf-operator-system
spec:
  template:
    spec:
      nodeSelector:
        matchExpressions:
          - key: kubernetes.io/os
            operator: In
            values:
              - "linux"
      template:
        metadata:
          labels:
            svc.dpu.nvidia.com/service: snap-dpu
            svc.dpu.nvidia.com/interface: app_sf
        spec:
          interfaceType: service
          service:
            interfaceName: app_sf
            serviceID: snap-dpu
            network: dpf-operator-system/mybrsfc
```

</details>

<details><summary>DPUServiceIPAM</summary>

[embedmd]:#(manifests/network/DPUServiceIPAM.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceIPAM
metadata:
  name: spdk-pool
  namespace: dpf-operator-system
spec:
  metadata:
    labels:
      svc.dpu.nvidia.com/pool: spdk-pool
  ipv4Subnet:
    subnet: "10.44.44.0/24"
    gateway: "10.44.44.1"
    perNodeIPCount: 20
```

</details>


### Deploy snap-dpu

```shell
cat manifests/snap-dpu/*.yaml | envsubst | kubectl apply -f -
```

This will deploy the following objects:

<details><summary>DPUService</summary>

[embedmd]:#(manifests/snap-dpu/DPUService.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUService
metadata:
  name: snap-dpu
  namespace: dpf-operator-system
spec:
  serviceID: snap-dpu
  interfaces:
    - snap-sf
  helmChart:
    source:
      repoURL: oci://$DPF_REGISTRY
      version: $DPF_VERSION
      chart: snap-dpu-chart
    values:
      imagePullSecrets: $DPF_IMAGE_PULL_SECRET
      docaSnap:
        hostNetwork: false
        resources:
          requests:
            memory: "2Gi"
            hugepages-2Mi: "4Gi"
            cpu: "8"
            nvidia.com/bf_sf: 1
          limits:
            memory: "4Gi"
            hugepages-2Mi: "4Gi"
            cpu: "16"
            nvidia.com/bf_sf: 1
        image:
          repository: nvcr.io/nvstaging/doca/doca_snap
          tag: 4.6.0-doca2.10.0
        imagePullSecrets:
          - name: dpf-pull-secret
        env:
          SNAP_RPC_INIT_CONF: "/etc/nvda_snap/snap_rpc_init.conf"
      snapNodeDriver:
        image:
          repository: $DPF_REGISTRY/snap-node-driver
          tag: $DPF_VERSION
      storagePlugin:
        image:
          repository: $DPF_REGISTRY/storage-vendor-dpu-plugin
          tag: $DPF_VERSION
      # RBAC roles for clients from host cluster
      # serviceAccount names must be aligned with the serviceAccount names from DPUServiceCredentialRequest
      rbacRoles:
        snapCsiPlugin:
          serviceAccount: snap-csi-plugin-sa
        snapController:
          serviceAccount: snap-controller-sa
      configuration:
        storageVendors:
          - name: nvidia
            storageClassName: spdkcsi-sc
            pluginName: nvidia
        storagePolicies:
          - name: policy1
            storageVendors: ["nvidia"]
            storageSelectionAlg: "Random"
```

</details>


### Deploy snap-cotroller

```shell
cat manifests/snap-controller/*.yaml | envsubst | kubectl apply -f -
```

This will deploy the following objects:

<details><summary>DPUService</summary>

[embedmd]:#(manifests/snap-controller/DPUService.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUService
metadata:
  name: snap-controller
  namespace: dpf-operator-system
spec:
  serviceID: snap-controller
  deployInCluster: true
  helmChart:
    source:
      repoURL: oci://$DPF_REGISTRY
      version: $DPF_VERSION
      chart: snap-controller-chart
    values:
      controller:
        image:
          repository: $DPF_REGISTRY/snap-controller
          tag: $DPF_VERSION
        imagePullSecrets: $DPF_IMAGE_PULL_SECRET
        config:
          # required parameter, name of the secret that contains connection
          # details to access the DPU cluster.
          # this secret should be created by the DPUServiceCredentialRequest API.
          dpuClusterSecret: snap-controller-dpu-cluster-credentials
          # controls where to search for StorageVendor and StoragePolicy CRs
          configNamespace: dpf-operator-system
```

</details>

<details><summary>DPUServiceCredentialRequest</summary>

[embedmd]:#(manifests/snap-controller/DPUServiceCredentialRequest.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceCredentialRequest
metadata:
  name: snap-controller-credentials
  namespace: dpf-operator-system
spec:
  duration: 10m
  serviceAccount:
    name: snap-controller-sa
    namespace: dpf-operator-system
  targetCluster:
    name: dpu-cplane-tenant1
    namespace: dpu-cplane-tenant1
  type: tokenFile
  secret:
    name: snap-controller-dpu-cluster-credentials
    namespace: dpf-operator-system
```

</details>


### Deploy snap-csi-plugin

```shell
cat manifests/snap-csi-plugin/*.yaml | envsubst | kubectl apply -f - 
```

This will deploy the following objects:

<details><summary>DPUService</summary>

[embedmd]:#(manifests/snap-csi-plugin/DPUService.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUService
metadata:
  name: snap-csi-plugin
  namespace: dpf-operator-system
spec:
  serviceID: snap-csi-plugin
  deployInCluster: true
  helmChart:
    source:
      repoURL: oci://$DPF_REGISTRY
      version: $DPF_VERSION
      chart: snap-csi-plugin-chart
    values:
      image:
        repository: $DPF_REGISTRY/snap-csi-plugin
        tag: $DPF_VERSION
      imagePullSecrets: $DPF_IMAGE_PULL_SECRET
      controller:
        config:
          # required parameter, name of the secret that contains connection
          # details to access the DPU cluster.
          # this secret should be created by the DPUServiceCredentialRequest API.
          dpuClusterSecret: snap-csi-plugin-dpu-cluster-credentials
          # required parameter, target namespace in the DPU cluster to create storage-related CRs
          targetNamespace: dpf-operator-system
```

</details>

<details><summary>DPUServiceCredentialRequest</summary>

[embedmd]:#(manifests/snap-csi-plugin/DPUServiceCredentialRequest.yaml)
```yaml
---
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUServiceCredentialRequest
metadata:
  name: snap-csi-plugin-credentials
  namespace: dpf-operator-system
spec:
  duration: 10m
  serviceAccount:
    name: snap-csi-plugin-sa
    namespace: dpf-operator-system
  targetCluster:
    name: dpu-cplane-tenant1
    namespace: dpu-cplane-tenant1
  type: tokenFile
  secret:
    name: snap-csi-plugin-dpu-cluster-credentials
    namespace: dpf-operator-system
```

</details>


### Examples workload for host cluster

```shell
cat manifests/host-cluster-examples/*.yaml | envsubst | kubectl apply -f - 
```

This will deploy the following objects:

<details><summary>StorageClass</summary>

[embedmd]:#(manifests/host-cluster-examples/storageclass.yaml)
```yaml
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: snap
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: csi.snap.nvidia.com
parameters:
  policy: "policy1"
```


</details>

<details><summary>Pod</summary>

[embedmd]:#(manifests/host-cluster-examples/pod.yaml)
```yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  containers:
    - name: myfrontend
      image: nginx
      volumeDevices:
        - name: data
          devicePath: /dev/xvda
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: myclaim
```


</details>

<details><summary>PersistentVolumeClaim</summary>

[embedmd]:#(manifests/host-cluster-examples/pvc.yaml)
```yaml
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: myclaim
spec:
  storageClassName: snap
  accessModes:
    - ReadWriteOnce
  volumeMode: Block
  resources:
    requests:
      storage: 8Gi
```


</details>


## Use localy build components

This example demonstrates how to use locally build image/chart for snap-dpu components

```
# increase this tag on each build to make sure that the right artifacts are pulled by the DPF cluster
export DPF_VERSION=v0.1.0-my-dev1
export DPF_REGISTRY=<your_registry>


# set env variables required for image/chart build
export TAG=$DPF_VERSION
export HELM_REGISTRY=oci://$DPF_REGISTRY
export REGISTRY=$DPF_REGISTRY

# build and push images and charts
pushd ../../..
make docker-build-storage-snap-node-driver \
 docker-build-storage-vendor-dpu-plugin \
 docker-push-storage-snap-node-driver \
 docker-push-storage-vendor-dpu-plugin \
 helm-package-snap-dpu \
 helm-push-snap-dpu

popd

# deploy/re-deploy manifests
# note: edit manifests in manifests/snap-dpu to modify values as needed

cat manifests/snap-dpu/*.yaml | envsubst | kubectl apply -f -

```
