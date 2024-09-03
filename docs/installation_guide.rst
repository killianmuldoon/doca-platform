#########################
DOCA Platform Foundations
#########################
################
Deployment Guide
################
Introduction
============

DOCA Platform Foundations (DPF) is a framework that enables the use of NVIDIA BlueField DPUs in modern, service-oriented data centers.

Through Kubernetes APIs DPF enables data center administrators to:

- Provision and manage the life cycle of DPUs: Automatically configure and update DPU components such as the BlueField OS (BFB), firmware and more.

- Define and configure desired services to run on the DPUs, including service life cycle management.

- Define the order of service connections using service function chains.

**Important:** This is an alpha release intended to showcase DPF. This release contains known limitations and incomplete functionality making it unsuitable for production environments.

Installation
============
This is a step-by-step installation guide for the **DOCA Platform Foundations** ("DPF") on vanilla Kubernetes.
It describes the complete steps required to deploy DPF with examples of basic operations and use cases.

Demo Release Requirements and Known Limitations
-----------------------------------------------
- Supported platforms: Kubernetes 1.31
- Supported OS: Ubuntu 24.04
- The cluster is a clean install with 3 x control plane nodes and no worker nodes
- There are no DPUs in the control plane nodes
- Each worker node contains a single DPU which is in DPU mode
- 1GbE fabric is used for cluster management and node internet access
- The high speed fabric and the cluster management fabric are routable
- The DPU Provisioning process may involve a host power cycle or reboot
- 3 x Single Root IO Virtualization (SR-IOV) Virtual Functions (VFs) on each worker node are reserved for system usage
- All pods created on DPF-enabled worker node are accelerated with an SR-IOV VF
- The Kubernetes cluster should provide storage for PersistentVolumes

Logical Design
==============
A DPF components overview and high level functional blocks diagrams are included in the repository.
## TODO: Add system diagrams.

Network Topology
================
A DPF reference deployment network diagram and underlay IPs scheme are included in the repository.
## TODO: Add network diagrams.

Deployment and Configuration
============================
Kubernetes Deployment
-----------------------------
Install Kubernetes 1.31 on cluster with 3 x control nodes. All nodes should run Ubuntu 24.04.

DPF Prerequisites Deployment
----------------------------
Install the components listed below before proceeding with DPF Operator deployment.

- CNI
    .. code-block:: bash

        # TODO: Include OVN Kubernetes installation flow. The cluster in that case should be deployed without either `kube-proxy` or a CNI. DPF uses OVN Kubernetes as the primary CNI.
        kubectl apply -f https://github.com/flannel-io/flannel/releases/download/v0.25.5/kube-flannel.yml


- Cert-Manager
    .. code-block:: bash

        kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.3/cert-manager.yaml

- CSI
    .. code-block:: bash
        # TODO: Include instructions on picking and customizing a storage class here.
        # TODO: The provisioning controller currently uses an NFS PVC in order to get ReadWriteMany. This should be configurable.
        kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.28/deploy/local-path-storage.yaml
        kubectl patch storageclass local-path -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'


- SR-IOV Network Operator
    .. code-block:: bash

        # TODO: Decide if SRIOV CNI is to be used for the October release.

- DPF Operator namespace and storage
    - A dedicated Namespace for the DPF Operator and shared NFS-backed storage for DPU images.
    - A PersistentVolumeClaim and PersistentVolume to use for BFB creation.
        # TODO: Update this section with instructions on how to bring your own CSI to DPF.

    .. code-block:: bash

        kubectl create ns dpf-operator-system

    .. code-block:: bash

        # TODO: Users must supply their own NFS Server configuration.
        cat <<EOF | kubectl apply -f -
        apiVersion: v1
        kind: PersistentVolume
        metadata:
          name: bfb-pv
        spec:
          capacity:
            storage: 10Gi
          volumeMode: Filesystem
          accessModes:
            - ReadWriteMany
          nfs:
            path: /mnt/dpf_share
            server: $IP_ADDRESS_FOR_NFS_SERVER
          persistentVolumeReclaimPolicy: Retain
        ---
        apiVersion: v1
        kind: PersistentVolumeClaim
        metadata:
          name: bfb-pvc
          namespace: dpf-operator-system
        spec:
          accessModes:
          - ReadWriteMany
          resources:
            requests:
              storage: 10Gi
          volumeMode: Filesystem
        EOF


DPF Deployment
--------------
DPF Operator Deployment
~~~~~~~~~~~~~~~~~~~~~~~

- Export your NGC API key

    .. code-block:: bash

        export NGC_API_KEY=XXXXXXXXXXXXXXXXXXXXXXXXXXXXX


- Registry log in and imagePullSecrets

    .. code-block:: bash

        echo "$NGC_API_KEY" | helm registry login nvcr.io --username \$oauthtoken --password-stdin
        kubectl -n dpf-operator-system create secret docker-registry dpf-pull-secret --docker-server=nvcr.io --docker-username="\$oauthtoken" --docker-password=$NGC_API_KEY
        cat <<EOF | kubectl apply -f -
        apiVersion: v1
        kind: Secret
        metadata:
          name: dpf-helm-secret
          namespace: dpf-operator-system
          labels:
            argocd.argoproj.io/secret-type: repository
        stringData:
          name: dpf-helm
          url: nvcr.io/nvstaging/mellanox
          type: helm
          username: \$oauthtoken
          password: $NGC_API_KEY
        EOF


- Deploy the DPF Operator bundle

    .. code-block:: bash

        echo "$NGC_API_KEY" | helm registry login nvcr.io --username \$oauthtoken --password-stdin
        helm upgrade --install -n dpf-operator-system --set imagePullSecrets\[0\]\.name=dpf-pull-secret dpf-operator oci://nvcr.io/nvstaging/mellanox/dpf-operator --version=v0.1.0-latest

    - Verify dpf-operator-controller-manager pod is Running:

    .. code-block:: bash

        kubectl get pod -n dpf-operator-system

- DPF DPUCluster control plane

    .. code-block:: bash

        kubectl create ns dpu-cplane-tenant1

        cat <<EOF | kubectl apply -f -
        apiVersion: kamaji.clastix.io/v1alpha1
        kind: TenantControlPlane
        metadata:
          name: dpu-cplane-tenant1
          namespace: dpu-cplane-tenant1
          labels:
            tenant.clastix.io: dpu-cplane-tenant1
        spec:
          dataStore: default
          controlPlane:
            deployment:
              replicas: 3
              additionalMetadata:
                labels:
                  tenant.clastix.io: dpu-cplane-tenant1
              extraArgs:
                apiServer: []
                controllerManager: []
                scheduler: []
            service:
              additionalMetadata:
                labels:
                  tenant.clastix.io: dpu-cplane-tenant1
              serviceType: ClusterIP
          kubernetes:
            version: v1.29.3
            kubelet:
              cgroupfs: systemd
            admissionControllers:
              - ResourceQuota
              - LimitRanger
          networkProfile:
            port: 6443
            certSANs:
              - dpu-cplane-tenant1.clastix.labs
            serviceCidr: 10.96.0.0/16
            podCidr: 10.36.0.0/16
            dnsServiceIPs:
              - 10.96.0.10
          addons:
            coreDNS: {}
            kubeProxy: {}
        EOF


DPF Operator Configuration
~~~~~~~~~~~~~~~~~~~~~~~~~~
- Apply the DPF Operator Configuration using the DPFOperatorConfig CR. Configuration includes a reference to the previously create image pull Secret and the BFB PVC.

    .. code-block:: bash

        cat <<EOF | kubectl apply -f -
        apiVersion: operator.dpf.nvidia.com/v1alpha1
        kind: DPFOperatorConfig
        metadata:
          name: dpfoperatorconfig
          namespace: dpf-operator-system
        spec:
          imagePullSecrets:
          - dpf-pull-secret
          provisioningConfiguration:
            bfbPVCName: "bfb-pvc"
        EOF

    - Verify DPF controllers and services are running:

    .. code-block:: console

        kubectl get -n dpf-operator-system pod,dpuservices
