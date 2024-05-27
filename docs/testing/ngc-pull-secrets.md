## Setting up pull secrets for NGC nvstaging

Before GA, DPF container images are published in NGC in a private registry.
In order to be able to pull from this registry, a Kubernetes Secret needs to be created
and referenced in the `DPFOperatorConfig` CR under `Spec.ImagePullSecrets`.

1. Make sure you have a user in NGC that is part of the nvstaging/mellanox organization.
2. If needed, generate a NGC API key from [here](https://org.ngc.nvidia.com/setup/api-key). Note that generating a new key will invalidate previous one.
3. Export your NGC API key to an environment variable (Replace `YOUR_KEY` with your actual key):
```bash
export NGC_API_KEY=YOUR_KEY
```
4. Create the Kubernetes secret in the DPF Operator namespace:
```bash
kubectl -n dpf-operator-system create secret docker-registry ngc-secret --docker-server=nvcr.io --docker-username="\$oauthtoken" --docker-password=$NGC_API_KEY
```
5. Specify the secret in the `DPFOperatorConfig`:

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