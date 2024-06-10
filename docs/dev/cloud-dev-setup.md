# Cloud dev setup

A guide to bring up a simple DPF environment

# Setup

## Allocation

Allocate dev setup with DPU, for [example](http://linux-cloud.mellanox.com/session_info/c055e3ed-02ab-4d0e-b42f-03c1fbdc2c33)

## Installing DOCA

login into one of the hosts.

Switch to super user

```sh
sudo -i
```

Install doca

```sh
sudo dpkg -i /auto/sw/release/doca/doca-host-repo/doca-repo-2.7.0/doca-repo-2.7.0-0.0.2-240327-092655-daily/doca-host*ubuntu2204*amd*.deb
apt update && apt install -y rshim
```

create `bfb.cfg` file with this contents
```sh
printf "%s\n" \
"ubuntu_PASSWORD='\$1\$vgUlT5Ao\$g6wB3Quxen2j3UF8a9opv/'" \
"ENABLE_SFC_HBN=yes" \
"NUM_VFs_PHYS_PORT0=12" \
"NUM_VFs_PHYS_PORT1=2" \
"CLOUD_OPTION=true" > bfb.cfg
```

Install BFB, it may take ~10min

```sh
sudo bfb-install --bfb /auto/sw_mc_soc_release/doca_dpu/doca_2.7.0/last_stable_ubuntu_22.04_dk --config ./bfb.cfg --rshim rshim0
```

To monitor the progress you can open another ssh connection and run 

> sudo screen /dev/rshim0/console 

if session is already open you can reconnect with
> sudo screen -r

Wait for login to be ready before moving on, similar to 

```
Ubuntu 22.04.4 LTS c-234-185-20-p65-00-0-bf2 hvc0

c-234-185-20-p65-00-0-bf2 login:
```

To close screen use `ctrl+a d`

## Prepare DPU environment

SSH into DPU with `ubuntu` user

Setup k8s on the DPU

```sh
sudo apt update
sudo swapoff -a
sudo sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab
sudo tee /etc/modules-load.d/containerd.conf <<EOF
overlay
br_netfilter
EOF
sudo modprobe overlay
sudo modprobe br_netfilter

sudo tee /etc/sysctl.d/kubernetes.conf <<EOF
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
EOF

sudo sysctl --system
apt install -y curl gnupg2 software-properties-common apt-transport-https ca-certificates && apt autoremove

sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmour -o /etc/apt/trusted.gpg.d/docker.gpg
sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" -y
```

Go with the defaults

```sh
sudo apt-get update
sudo apt-get install ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

# Add the repository to Apt sources:
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null


sudo apt-get update
sudo apt-get install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin -y
sudo apt autoremove -y

containerd config default | sudo tee /etc/containerd/config.toml >/dev/null 2>&1
sudo sed -i 's/SystemdCgroup \= false/SystemdCgroup \= true/g' /etc/containerd/config.toml

sudo systemctl restart containerd
sudo systemctl enable containerd
```

```sh
sudo apt update
sudo apt install -y kubelet kubeadm kubectl --allow-change-held-packages 

sudo systemctl stop kubelet
sudo kubeadm init --pod-network-cidr=10.244.0.0/16
```

docker and kubelet are installed

copy kubecfg file

```sh
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
export KUBECONFIG=/etc/kubernetes/admin.conf
```

Untaint the node

```sh
kubectl taint nodes $(kubectl get node -o jsonpath="{.items[0].metadata.name}") node-role.kubernetes.io/control-plane-
```

# Setup DPU as dev environment 

## install golang 

```sh
wget https://go.dev/dl/go1.22.4.linux-arm64.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.4.linux-arm64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

## building the container

We will use harbor as a docker registry, can be replaced to a different repo but it will require to change the tags as well.

```sh
docker login harbor.mellanox.com -u ***** -p *****
```

## cloning the repo

Create access token in gitlab user

Copy [git-clone-repo.sh](https://gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/-/blob/main/hack/scripts/git-clone-repo.sh?ref_type=heads)

Export the token

```sh
./git-clone-repos.sh ssh://git@gitlab-master.nvidia.com:12051/doca-platform-foundation/dpf-operator.git dpf-operator
```

# deployment

All the instructions are relevant to both local development environment and DPU environment

Before starting make sure you are able to build

> make binaries

to push images define a registry, with your user, if you are using the DPU as a dev env make sure to change the user to your name

> export REGISTRY=harbor.mellanox.com/cloud-orchestration-dev/$USER; export TAG=v1.0.2

In order to deploy with your tags, make sure to run

> make generate-manifests

deploy flannel

> kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml

Now deploy sriov-dp, multus and ovs-cni (make sure you have helm chart installed)

```sh
helm install sriovdp ./deploy/helm/sriov-device-plugin/
helm install multus ./deploy/helm/multus/
helm install ovs-cni ./deploy/helm/ovs-cni/
```

Build your controller image and push it to a registry, in this doc i will use sfc-controller as an example

> make docker-build-sfc-controller &&  make docker-push-sfc-controller

deploy with helm

> helm install sfc-controller ./deploy/helm/sfc-controller