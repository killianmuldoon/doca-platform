# Updating the DPUSet

An update to the DPU Set can be done for upgrading the BFB or modifying provisioning parameters. 

> Notice:<br>
> This operation will result in a network
> disruption and also a host reboot.<br> 
> A rolling update can be configured to control the number of nodes that will be out-of-service in parallel (Please see the DPUSet YAML example below).<br>
> The cluster can also be divided into several DPU-Sets, please refer to the section
> "Using several DPU Sets"

These are the required steps for upgrading the BFB on a set of DPUs
(The BFB is specified as part of the DPU Set CRD):
1) Create a BFB YAML that includes the required BFB file and also assigns a distinct name for the object  (Different than the currently used BFB objects). 
After applying the YAML, the BFB will be pulled from the specified URL to the shared storage:
  
  ```yaml
---
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: BFB
metadata:
  name: bf-bundle-new
  namespace: dpf-operator-system
spec:
  url: https://content.mellanox.com/BlueField/BFBs/Ubuntu22.04/bf-bundle-2.9.0-90_24.10_ubuntu-22.04_prod.bfb
```
   
2) Update the DPUSet YAML to point to the new BFB object:


    
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
      name: bf-bundle-new
      nodeEffect:
      taint:
        key: "dpu"
        value: "provisioning"
        effect: NoSchedule
      automaticNodeReboot: true


3) Then delete the DPU objects of the relevant DPUs. 

This will initiate a provisioning cycle for the DPUs using the new BFB image:
   

```shell
kubectl delete dpu -n dpf-operator-system worker1-0000-2b-00 worker2-0000-2b-00
```
4) You can later delete the previous BFB object:

```shell
kubectl delete bfb -n dpf-operator-system bf-bundle
```

## Using several DPU Sets

There's an option to create several DPU-Set objects, and assign them to different groups of worker nodes.
This is done by adding relevant labels to the node selector in the DPUSet object YAML.
Each DPU Set can use a different BFB object, can have a different DPU flavor, a differnet rolling update strategy, etc.

For example:

```yaml
---
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUSet
metadata:
  name: dpuset-dk
  namespace: dpf-operator-system
spec:
  nodeSelector:
  matchLabels:
    e2e.servers/dk: "true"
  strategy:
  rollingUpdate:
    maxUnavailable: "10%"
  type: RollingUpdate
  dpuTemplate:
  spec:
    dpuFlavor: dpf-provisioning-hbn-ovn
    bfb:
    name: bf-bundle-dk-ga
    nodeEffect:
    taint:
      key: "dpu"
      value: "provisioning"
      effect: NoSchedule
    automaticNodeReboot: true
```
