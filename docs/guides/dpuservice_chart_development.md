# DPUService Chart Development Guidelines

This doc describes how DPUService Charts should be built and the contract they have to implement so that the rest of DPF
controllers can control them. This does not nessecarily target system components, but rather services the customer
expects to run on the DPUs.

## Contract

The following requirements must be satisfied by the Helm Chart. The controllers dynamically modify and have
assumptions on those. If any of them is not satisfied, the system may not work as expected.

* There must be exactly one DaemonSet in the chart - the serviceDaemonSet - that is part of a service function chain.
* The DaemonSet should deploy the service on as many nodes as selected. The selector must be implemented with [NodeAffinity](https://pkg.go.dev/k8s.io/api@v0.32.1/core/v1#NodeAffinity)
  using `RequiredDuringSchedulingIgnoredDuringExecution` and must be exposed using `serviceDaemonSet.nodeSelector` as parameter.
* The chart must expose a parameter in the values to add labels to the pod that is part of the chain. The parameter is
  `serviceDaemonSet.labels` and takes a map of key and values.
* The chart must expose a parameter in the values to add annotations to the pod that is part of the chain. The parameter
  is `serviceDaemonSet.annotations` and takes a map of keys and values
* Daemonset upgrade strategy must be parameterized on the daemonset object contained in the chart. The parameter must
  be provided via the values as `serviceDaemonSet.upgradeStrategy`
* All resources included in a service helm chart must include `{{ .Release.Name }}` in their name to allow deployment of
  multiple instances of the service on the cluster (version specific), even in the same namespace.
* Version constraints must be added in the Helm chart annotations (e.g. `dpu.nvidia.com/doca-version` for DOCA version).

## Recommendations

The following list contains some recommendations for the Helm Chart. If the chart developer doesn't implement those, the
chart user may face issues deploying the chart in e.g. air-gapped or large scale environments.

* Resource requests and limits for the DaemonSet should be configurable
* ImagePullSecrets for the DaemonSet should be configurable
* CRDs should be placed in the crd folder of the Helm chart.
* If the application needs to access resources in a different cluster than the one the application is deployed in, the
  chart should allow for configurable `volumes`, `volumeMounts` and `env` so that a [DPUServiceCredentialRequest](dpuservice_credential_request.md)
  can be used.

## Example

An example of such a Helm Chart that aligns with the DPF contract and recommendations can be found in the [dummydpuservice](../../deploy/dpuservices/dummydpuservice)
folder.


## Additional information about supported features

### Version Constraints

The version constraints can be defined by setting specific annotations as part of the `Chart.yaml` file of the chart.
An example of that can be seen below:

[embedmd]:#(../../deploy/dpuservices/dummydpuservice/chart/Chart.yaml)
```yaml
apiVersion: v2
name: dummydpuservice-chart
description: Dummydpuservice chart for Kubernetes
type: application
version: 0.1.0
appVersion: "0.1.0"
annotations:
  dpu.nvidia.com/doca-version: ">= 2.9"
```

The constraints must comply with https://github.com/Masterminds/semver/blob/v3.3.1/README.md?plain=1#L117-L212

There is a specific set of annotations that are handled today, the rest are stripped:

- `dpu.nvidia.com/doca-version`
