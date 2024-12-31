# snap-csi-plugin-chart

![Version: 0.0.1](https://img.shields.io/badge/Version-0.0.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.1](https://img.shields.io/badge/AppVersion-0.0.1-informational?style=flat-square)

A Helm chart for SNAP CSI plugin

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| controller.affinity | object | `{}` |  |
| controller.config.dpuClusterSecret | string | `""` |  |
| controller.config.targetNamespace | string | `""` |  |
| controller.externalAttacher.image.repository | string | `"registry.k8s.io/sig-storage/csi-attacher"` |  |
| controller.externalAttacher.image.tag | string | `"v4.7.0"` |  |
| controller.externalAttacher.pullPolicy | string | `"IfNotPresent"` |  |
| controller.externalAttacher.resources | object | `{}` |  |
| controller.externalAttacher.securityContext.allowPrivilegeEscalation | bool | `false` |  |
| controller.externalProvisioner.image.repository | string | `"registry.k8s.io/sig-storage/csi-provisioner"` |  |
| controller.externalProvisioner.image.tag | string | `"v5.1.0"` |  |
| controller.externalProvisioner.pullPolicy | string | `"IfNotPresent"` |  |
| controller.externalProvisioner.resources | object | `{}` |  |
| controller.externalProvisioner.securityContext.allowPrivilegeEscalation | bool | `false` |  |
| controller.imagePullSecrets | list | `[]` |  |
| controller.livenessProbe.image.repository | string | `"registry.k8s.io/sig-storage/livenessprobe"` |  |
| controller.livenessProbe.image.tag | string | `"v2.14.0"` |  |
| controller.livenessProbe.pullPolicy | string | `"IfNotPresent"` |  |
| controller.livenessProbe.resources | object | `{}` |  |
| controller.livenessProbe.securityContext.allowPrivilegeEscalation | bool | `false` |  |
| controller.nodeSelector | object | `{}` |  |
| controller.plugin.args.logLevel | int | `2` |  |
| controller.plugin.image.repository | string | `""` |  |
| controller.plugin.image.tag | string | `""` |  |
| controller.plugin.pullPolicy | string | `"IfNotPresent"` |  |
| controller.plugin.resources | object | `{}` |  |
| controller.plugin.securityContext.allowPrivilegeEscalation | bool | `false` |  |
| controller.podAnnotations | object | `{}` |  |
| controller.podLabels | object | `{}` |  |
| controller.podSecurityContext | object | `{}` |  |
| controller.replicas | int | `1` |  |
| controller.tolerations[0].effect | string | `"NoSchedule"` |  |
| controller.tolerations[0].key | string | `"node-role.kubernetes.io/control-plane"` |  |
| controller.tolerations[0].operator | string | `"Exists"` |  |
| controller.tolerations[1].effect | string | `"NoSchedule"` |  |
| controller.tolerations[1].key | string | `"node-role.kubernetes.io/master"` |  |
| controller.tolerations[1].operator | string | `"Exists"` |  |
| image.repository | string | `"example.com/snap-csi-plugin"` |  |
| image.tag | string | `""` |  |
| imagePullSecrets | list | `[]` |  |
| node.config.snapControllerDeviceId | string | `"6001"` |  |
| node.imagePullSecrets | list | `[]` |  |
| node.kubeletDir | string | `"/var/lib/kubelet"` |  |
| node.livenessProbe.image.repository | string | `"registry.k8s.io/sig-storage/livenessprobe"` |  |
| node.livenessProbe.image.tag | string | `"v2.14.0"` |  |
| node.livenessProbe.pullPolicy | string | `"IfNotPresent"` |  |
| node.livenessProbe.resources | object | `{}` |  |
| node.livenessProbe.securityContext.allowPrivilegeEscalation | bool | `false` |  |
| node.nodeDriverRegistrar.image.repository | string | `"registry.k8s.io/sig-storage/csi-node-driver-registrar"` |  |
| node.nodeDriverRegistrar.image.tag | string | `"v2.12.0"` |  |
| node.nodeDriverRegistrar.pullPolicy | string | `"IfNotPresent"` |  |
| node.nodeDriverRegistrar.resources | object | `{}` |  |
| node.nodeDriverRegistrar.securityContext.allowPrivilegeEscalation | bool | `false` |  |
| node.plugin.args.logLevel | int | `2` |  |
| node.plugin.image.repository | string | `""` |  |
| node.plugin.image.tag | string | `""` |  |
| node.plugin.pullPolicy | string | `"IfNotPresent"` |  |
| node.plugin.resources | object | `{}` |  |
| node.plugin.securityContext.allowPrivilegeEscalation | bool | `true` |  |
| node.plugin.securityContext.capabilities.add[0] | string | `"SYS_ADMIN"` |  |
| node.plugin.securityContext.privileged | bool | `true` |  |
| node.plugin.securityContext.runAsUser | int | `0` |  |
| node.podSecurityContext | object | `{}` |  |
| node.tolerations[0].effect | string | `"NoSchedule"` |  |
| node.tolerations[0].key | string | `"node-role.kubernetes.io/control-plane"` |  |
| node.tolerations[0].operator | string | `"Exists"` |  |
| node.tolerations[1].effect | string | `"NoSchedule"` |  |
| node.tolerations[1].key | string | `"node-role.kubernetes.io/master"` |  |
| node.tolerations[1].operator | string | `"Exists"` |  |
| serviceDaemonSet.annotations | object | `{}` |  |
| serviceDaemonSet.labels | object | `{}` |  |
| serviceDaemonSet.updateStrategy | object | `{}` |  |

