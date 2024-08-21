# DOCA Platform Foundation

DOCA Platform Foundation (DPF) is a system that orchestrates NVIDIA Data Processing Units (DPU) using a Kubernetes API. It provisions and manages DPU devices and orchestrates specialized DPUServices which run on those devices.

DPF manages DPUs installed in nodes that are part of a Kuberntes cluster.

## Glossary
- **Target cluster**: A Kubernetes cluster on which DPF is installed.
- **DPU Cluster**: A Kubernetes cluster in which each node is a DPU.

## Quick start
The quickest way to get up and running with DPF is to follow the [cloud deployment guide](./docs/dev/cloud-dev-setup.md)

## Hardware
DPF enables NVIDIA [Bluefield DPUs](https://www.nvidia.com/en-gb/networking/products/data-processing-unit/). These PCI devices are installed in servers and handle network traffic through network ports. Bluefield DPUs have arm64 CPUs and runs a standard Linux OS.

## Provisioning
DPF provisions a Kubernetes cluster - the DPU Cluster - which each DPU joins as a node. The control plane is based on [Kamaji](https://github.com/clastix/kamaji), and is hosted as a set of Kuberetes pods in the Target cluster.

DPF's provisioning controllers manage the bootstrap sequence for DPUs. This involves:
1) Installing and configuring firmware on the DPU.
2) Accessing the Bluefield boot stream (BFB). The BFB contains an OS and other tools.
3) Flashing the BFB to the DPU with additional config.
4) Bootstrapping the DPU as a Kubernetes node and joining the DPU cluster.

## Network services
DPF enables offloading network operations to DPUs. This frees up resources on machines in the Target cluster.

Many of these network operations enable the use of discrete accelerated data plane networks which are attached to pods in the Target cluster but accelerated by services running on the DPUs.

Network services targeted by DPF:
- [OpenShift Cloud Platform primary CNI ovn-kubernetes offloads](https://docs.openshift.com/container-platform/4.10/networking/hardware_networks/configuring-hardware-offloading.html)
- [Host Based Networking -  L3 networking](https://docs.nvidia.com/doca/sdk/nvidia+doca+hbn+service+guide/index.html#:~:text=Host%2Dbased%20Networking%20(HBN),BlueField%20as%20a%20BGP%20router)
- [Firefly - precision time protocol service](https://docs.nvidia.com/doca/sdk/nvidia+doca+firefly+service+guide/index.html)

## Service orchestration
DPF orchestrates workloads which run on DPUs using [NVIDIA DOCA](https://developer.nvidia.com/networking/doca).

DPF's service orchestration controllers manage the installation and lifecycle of these services.


## Contributing

This repository follows a [contribution guide](docs/CONTRIBUTING.md).