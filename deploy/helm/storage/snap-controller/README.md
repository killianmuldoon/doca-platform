# snap-controller-chart

![Version: 0.0.1](https://img.shields.io/badge/Version-0.0.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.1](https://img.shields.io/badge/AppVersion-0.0.1-informational?style=flat-square)

A Helm chart for SNAP controller

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| controller.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].key | string | `"node-role.kubernetes.io/master"` |  |
| controller.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].operator | string | `"Exists"` |  |
| controller.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[1].matchExpressions[0].key | string | `"node-role.kubernetes.io/control-plane"` |  |
| controller.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[1].matchExpressions[0].operator | string | `"Exists"` |  |
| controller.config.dpuClusterSecret | string | `""` |  |
| controller.image.repository | string | `"example.com/snap-controller"` |  |
| controller.image.tag | string | `""` |  |
| controller.nodeSelector | object | `{}` |  |
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
| imagePullSecrets | list | `[]` |  |

