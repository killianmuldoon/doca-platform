# spdk-csi-controller-chart

![Version: 0.0.1](https://img.shields.io/badge/Version-0.0.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.1](https://img.shields.io/badge/AppVersion-0.0.1-informational?style=flat-square)

A Helm chart for SPDK CSI controller

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].key | string | `"node-role.kubernetes.io/master"` |  |
| affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].operator | string | `"Exists"` |  |
| affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[1].matchExpressions[0].key | string | `"node-role.kubernetes.io/control-plane"` |  |
| affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[1].matchExpressions[0].operator | string | `"Exists"` |  |
| controller.replicas | int | `1` |  |
| csiConfig.nodes[0].name | string | `"localhost"` |  |
| csiConfig.nodes[0].rpcURL | string | `"http://127.0.0.1:9009"` |  |
| csiConfig.nodes[0].targetAddr | string | `"127.0.0.1"` |  |
| csiConfig.nodes[0].targetType | string | `"nvme-tcp"` |  |
| dpuClusterSecret | string | `""` |  |
| externallyManagedConfigmap.create | bool | `true` |  |
| image.csiProvisioner.pullPolicy | string | `"IfNotPresent"` |  |
| image.csiProvisioner.repository | string | `"registry.k8s.io/sig-storage/csi-provisioner"` |  |
| image.csiProvisioner.tag | string | `"v3.5.0"` |  |
| image.csiSnapshotter.pullPolicy | string | `"IfNotPresent"` |  |
| image.csiSnapshotter.repository | string | `"registry.k8s.io/sig-storage/csi-snapshotter"` |  |
| image.csiSnapshotter.tag | string | `"v6.2.2"` |  |
| image.externalSnapshotter.pullPolicy | string | `"IfNotPresent"` |  |
| image.externalSnapshotter.repository | string | `"registry.k8s.io/sig-storage/snapshot-controller"` |  |
| image.externalSnapshotter.tag | string | `"v6.2.2"` |  |
| image.nodeDriverRegistrar.pullPolicy | string | `"IfNotPresent"` |  |
| image.nodeDriverRegistrar.repository | string | `"registry.k8s.io/sig-storage/csi-node-driver-registrar"` |  |
| image.nodeDriverRegistrar.tag | string | `"v2.8.0"` |  |
| image.spdkcsi.pullPolicy | string | `"IfNotPresent"` |  |
| image.spdkcsi.repository | string | `"example.com/spdkcsi"` |  |
| image.spdkcsi.tag | string | `"v0.1.0"` |  |
| tolerations[0].effect | string | `"NoSchedule"` |  |
| tolerations[0].key | string | `"node-role.kubernetes.io/control-plane"` |  |
| tolerations[0].operator | string | `"Exists"` |  |
| tolerations[1].effect | string | `"NoSchedule"` |  |
| tolerations[1].key | string | `"node-role.kubernetes.io/master"` |  |
| tolerations[1].operator | string | `"Exists"` |  |

