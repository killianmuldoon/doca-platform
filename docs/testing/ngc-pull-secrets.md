## Setting up pull secrets for NGC nvstaging

Before GA, DPF container images are published in NGC in a private registry.
In order to be able to pull from this registry, a Kubernetes Secret needs to be created
and referenced in the `DPFOperatorConfig` CR under `Spec.ImagePullSecrets`.

### Create Secret
1. Make sure you have a user in NGC that is part of the nvstaging/mellanox organization.
2. If needed, generate a NGC API key from [here](https://org.ngc.nvidia.com/setup/api-key). Note that generating a new key will invalidate previous one.
3. Create DPF Operator namespace:
```bash
kubectl create ns dpf-operator-system
```
4. Export your NGC API key to an environment variable (Replace `<YOUR_KEY>` with your actual key):
```bash
export NGC_API_KEY=<YOUR_KEY>
```
5. Create the Kubernetes secret in the DPF Operator namespace:
```bash
kubectl -n dpf-operator-system create secret docker-registry ngc-secret --docker-server=nvcr.io --docker-username="\$oauthtoken" --docker-password=$NGC_API_KEY
```

### Create Argo CD secret for NGC nvstaging Helm charts

Some DPF components are deployed by the DPF Operator as DPU Services, that are based on Helm charts that are hosted on NGC nvstaging.

The following secret is required for Argo CD to be able to deploy the charts.


```bash
echo "
apiVersion: v1
kind: Secret
metadata:
  name: nvstaging-helm
  namespace: dpf-operator-system
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  name: nvstaging
  url: https://helm.ngc.nvidia.com/nvstaging/mellanox
  type: helm
  username: \$oauthtoken
  password: $NGC_API_KEY
" | kubectl apply -f -
```

### Install DPF Operator with Operator-SDK from NGC nvstaging

1. Create Secret as described above.
2. Login to NGC in docker:
```bash
docker login nvcr.io --username \$oauthtoken --password $NGC_API_KEY
```
3. Download Operator SDK from [here](https://github.com/operator-framework/operator-sdk/releases/download).
4. If needed, install OLM. (Not required on OpenShift cluster)
```bash
operator-sdk olm install --version v0.27.0
```
5. Install DPF Operator bundle:
```bash
operator-sdk run bundle --namespace dpf-operator-system  --pull-secret-name ngc-secret nvcr.io/nvstaging/mellanox/dpf-operator-bundle:0.0.1
```

### Configure secret in DPFOperatorConfig
Specify the secret in the `DPFOperatorConfig`:

```yaml
apiVersion: operator.dpf.nvidia.com/v1alpha1
kind: DPFOperatorConfig
metadata:
  name: dpfoperatorconfig
  namespace: dpf-operator-system
spec:
  imagePullSecrets:
  - ngc-secret
...
```