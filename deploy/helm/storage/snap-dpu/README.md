# snap-dpu-chart

![Version: 0.0.1](https://img.shields.io/badge/Version-0.0.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.1](https://img.shields.io/badge/AppVersion-0.0.1-informational?style=flat-square)

Helm chart that deploys doca-snap, snap-node-driver, and storage-vendor-dpu-plugin together.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| configuration.storagePolicies | list | `[]` |  |
| configuration.storageVendors | list | `[]` |  |
| docaSnap.env | object | `{"APP_ARGS":"","SNAP_RPC_INIT_CONF":"","SPDK_RPC_INIT_CONF":"","SPDK_RPC_INIT_CONF_JSON":"","SPDK_XLIO_PATH":""}` | Environment variables |
| docaSnap.hostNetwork | bool | `false` |  |
| docaSnap.image | object | `{"repository":"nvcr.io/nvstaging/doca/doca_snap","tag":"4.5.0-6-doca2.9.0"}` | Container image |
| docaSnap.imagePullSecrets | list | `[]` | Image pull secrets (if pulling from a private registry) |
| docaSnap.name | string | `"doca-snap"` | DaemonSet name used in metadata.  |
| docaSnap.pullPolicy | string | `"IfNotPresent"` |  |
| docaSnap.resources | object | `{"limits":{"cpu":"16","hugepages-2Mi":"4Gi","memory":"4Gi"},"requests":{"cpu":"8","hugepages-2Mi":"4Gi","memory":"2Gi"}}` | Resource requests and limits |
| docaSnap.restartPolicy | string | `"Always"` | Restart policy for the DaemonSet pods |
| docaSnap.securityContext | object | `{"capabilities":{"add":["IPC_LOCK","SYS_RAWIO","SYS_NICE"]},"privileged":true}` | Security context for the container |
| imagePullSecrets | list | `[]` |  |
| rbacRoles.snapController.create | bool | `true` |  |
| rbacRoles.snapController.serviceAccount | string | `"snap-controller-sa"` |  |
| rbacRoles.snapCsiPlugin.create | bool | `true` |  |
| rbacRoles.snapCsiPlugin.serviceAccount | string | `"snap-csi-plugin-sa"` |  |
| serviceDaemonSet.annotations | object | `{}` |  |
| serviceDaemonSet.labels | object | `{}` |  |
| serviceDaemonSet.updateStrategy | object | `{}` |  |
| snapNodeDriver.annotations | object | `{}` |  |
| snapNodeDriver.env | list | `[]` |  |
| snapNodeDriver.image.repository | string | `"example.com/snap-node-driver"` |  |
| snapNodeDriver.image.tag | string | `"v0.1.0"` |  |
| snapNodeDriver.imagePullSecrets | list | `[]` |  |
| snapNodeDriver.labels | object | `{}` |  |
| snapNodeDriver.pullPolicy | string | `"IfNotPresent"` |  |
| snapNodeDriver.resources.limits.cpu | string | `"200m"` |  |
| snapNodeDriver.resources.limits.memory | string | `"256Mi"` |  |
| snapNodeDriver.resources.requests.cpu | string | `"50m"` |  |
| snapNodeDriver.resources.requests.memory | string | `"128Mi"` |  |
| snapNodeDriver.securityContext.runAsUser | int | `0` |  |
| storagePlugin.annotations | object | `{}` |  |
| storagePlugin.env | list | `[]` |  |
| storagePlugin.image.repository | string | `"example.com/storage-vendor-dpu-plugin"` |  |
| storagePlugin.image.tag | string | `"v0.1.0"` |  |
| storagePlugin.imagePullSecrets | list | `[]` |  |
| storagePlugin.labels | object | `{}` |  |
| storagePlugin.pullPolicy | string | `"IfNotPresent"` |  |
| storagePlugin.resources.limits.cpu | string | `"200m"` |  |
| storagePlugin.resources.limits.memory | string | `"256Mi"` |  |
| storagePlugin.resources.requests.cpu | string | `"50m"` |  |
| storagePlugin.resources.requests.memory | string | `"128Mi"` |  |
| storagePlugin.securityContext.runAsUser | int | `0` |  |
| tolerations | list | `[]` |  |

