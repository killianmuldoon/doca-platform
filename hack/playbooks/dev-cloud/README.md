# Pepare cloud dev setup

The goal of this playbook is a base setup of DPU in a cloud setup, [allocation example](../../../docs/dev/cloud-dev-setup.md#allocation). Each DPU will be a separate K8S cluster

The playbook will burn BFB on the DPU and setup a vanilla k8s cluster so you can start work with and deploy workloads on

The palybook will not setup flannel and other other helm charts, for those parts you can follow this setup [guide](../../../docs/dev/cloud-dev-setup.md)

## Prepare and run

Edit inventory file and set your host and dpu, for example

```
[cloud]
c-234-181-200-201

[dpu]
10.234.0.241
```

If you already had an ssh connection to the DPU you may need to clear it from known hosts otherwise it will not be able to connect to if after it have a new bfb version

```sh
ssh-keygen -R "<dpi-ip-addr>"
```

To run the playbook

```sh
ansible-playbook setup.yaml --ask-pass
```

If the setup is completed without errors you will have a kubeconfig to your DPU under `/tmp/cloud_kubeconfig.conf`


Export the kubeconfig and it's ready to be used

```sh
export KUBECONFIG=/tmp/cloud_kubeconfig.conf
```

Output example

```sh
kubectl get pod -A
NAMESPACE      NAME                                                 READY   STATUS    RESTARTS   AGE
kube-system    coredns-76f75df574-4rq55                             1/1     Running   0          59m
kube-system    coredns-76f75df574-9jg6n                             1/1     Running   0          59m
kube-system    etcd-c-234-181-200-p17-00-0-bf2                      1/1     Running   0          59m
kube-system    kube-apiserver-c-234-181-200-p17-00-0-bf2            1/1     Running   0          59m
kube-system    kube-controller-manager-c-234-181-200-p17-00-0-bf2   1/1     Running   0          59m
kube-system    kube-proxy-46w2c                                     1/1     Running   0          59m
kube-system    kube-scheduler-c-234-181-200-p17-00-0-bf2            1/1     Running   0          59m
```