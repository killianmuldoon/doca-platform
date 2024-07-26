# Test Custom OVN Kubernetes on OpenShift Multi Node cluster

### NOTE: This guide is out of date and may not work with v0.1.X of DPF. 
## QA provided cluster

This guide serves as a blueprint on how to test the custom OVN Kubernetes setup on a cluster provided by QA.

* Cluster provided by QA via https://gitlab-master.nvidia.com/cloud-orchestration/qa/infra/cloud-ops/-/blob/main/openshift/deploy_openshift_cluster.sh?ref_type=heads
* Credentials are stored under `/.autodirect/QA/qa/qa/cloudx/openshift_cluster/dpf/`

### Setup Environment

1. Ask Noam Angel to create 4.14 cluster
2. Ensure secure boot is disabled on the worker nodes
3. Flash BFB on both DPUs (DK cards)
4. Setup DPU cluster
   1. Login to DPU node 1 (ubuntu@10.209.24.143)
   2. Run `systemctl disable kubelet && systemctl stop kubelet`
   3. Run `curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--tls-san 10.209.24.143" sh -`
   4. Login to DPU node 2 (ubuntu@10.209.24.155)
   5. Run `systemctl disable kubelet && systemctl stop kubelet`
   6. Run `curl -sfL https://get.k3s.io | K3S_TOKEN=xxx K3S_URL=https://10.209.24.143:6443 sh -`. Token can be found
      at DPU node 1 under `/var/lib/rancher/k3s/server/node-token`
5. Create secret for DPU cluster on the host cluster:
   ```
   apiVersion: v1
   kind: Secret
   metadata:
     name: dpu-admin-kubeconfig
     labels:
       kamaji.clastix.io/component: admin-kubeconfig
       kamaji.clastix.io/project: kamaji
       kamaji.clastix.io/name: dpu
   stringData:
     # Replace kubeconfig with what's under /etc/rancher/k3s/k3s.yaml
     admin.conf: |-
       apiVersion: v1
       clusters:
       - cluster:
           certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTVRJMk5USXdPVGt3SGhjTk1qUXdOREE1TURnME1UTTVXaGNOTXpRd05EQTNNRGcwTVRNNQpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTVRJMk5USXdPVGt3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUQ09FN0dUVWtJSXZ6S05HRm9jZGwxaXFoMEZSVkQ3K2FNKys4SHJMdmcKalVHWTJrb0VRY09sQjFiT3gyTHFJTlFRU1JSc3IrdWR6ZHFzZDkyUDBRaVlvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVWN1NnBUM0RWNjJNcVIxQW0ydk9UCnpueVgzSnN3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUloQU5uaXV0Rnl2OTJreFpMSUJ2SGNjY3lFbm43MG5PQUoKWkJ1dmQzaHVzdU9NQWlBT1lHTUdnaS9QWE95Vlk3SFI0dnRON0FKRWNzRTMzZHV4V0VlLy9OMFdEQT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
           # Replace 127.0.0.1 with IP of the control plane node
           server: https://10.209.24.143:6443
         name: default
       contexts:
       - context:
           cluster: default
           user: default
         name: default
       current-context: default
       kind: Config
       preferences: {}
       users:
       - name: default
         user:
           client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJrRENDQVRlZ0F3SUJBZ0lJZUlPbUFvMk1vSWt3Q2dZSUtvWkl6ajBFQXdJd0l6RWhNQjhHQTFVRUF3d1kKYXpOekxXTnNhV1Z1ZEMxallVQXhOekV5TmpVeU1EazVNQjRYRFRJME1EUXdPVEE0TkRFek9Wb1hEVEkxTURRdwpPVEE0TkRFek9Wb3dNREVYTUJVR0ExVUVDaE1PYzNsemRHVnRPbTFoYzNSbGNuTXhGVEFUQmdOVkJBTVRESE41CmMzUmxiVHBoWkcxcGJqQlpNQk1HQnlxR1NNNDlBZ0VHQ0NxR1NNNDlBd0VIQTBJQUJHUDBvaG9ZMHc3NWZqdlkKZ0RXcFZRZTI0RGliMEhDQmhqZ29leG95VkpEOWNVN0xDS2NFU2pQNDZROEI3dDRzZkZWRnlaQk9sTDcrUUNUSApRUms4THR1alNEQkdNQTRHQTFVZER3RUIvd1FFQXdJRm9EQVRCZ05WSFNVRUREQUtCZ2dyQmdFRkJRY0RBakFmCkJnTlZIU01FR0RBV2dCUzkxdEFad1ByRXZlMGlYYTUzTkJCN0I2dlhxREFLQmdncWhrak9QUVFEQWdOSEFEQkUKQWlCQlZEbE12UXB1eVJsZDZTd0FyYW5uT1ZzNmZCakI4eG5XdWlIa0ZLZjdXZ0lnWUlQL0xTdXNkK1c2R3JkNAphQ1BwVjdmdTYvMlJMNDlhSkFSdEJwYm43RXc9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0KLS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJlRENDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdFkyeHAKWlc1MExXTmhRREUzTVRJMk5USXdPVGt3SGhjTk1qUXdOREE1TURnME1UTTVXaGNOTXpRd05EQTNNRGcwTVRNNQpXakFqTVNFd0h3WURWUVFEREJock0zTXRZMnhwWlc1MExXTmhRREUzTVRJMk5USXdPVGt3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFSWnROekNFM1V3QlduSjkrenhiWSt6VDlZM2Jqd3RYQXJ1YUlQb2xqMTAKU0FvVE13bDBDRXAwbjJjZEcxa1k3ZDRXamZNRG84bHlQQzlSNUpjVk1UVmRvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVXZkYlFHY0Q2eEwzdElsMnVkelFRCmV3ZXIxNmd3Q2dZSUtvWkl6ajBFQXdJRFNRQXdSZ0loQUxnanFrNWlZaTRCZDBzZUZ1QnhXQ1lhbVMvbkVFUjcKRTdTdDYyYjlwbDZUQWlFQXJkeTBPRTZHb0hwOWJDU0Y2cVRlM0JyT01YTmJYVkFmZTRjeVFISkFlUk09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
           client-key-data: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSUpudGpEcWVjRVJDZE9oQlFJaWI1Y0pFT0thMDFsM3ZUWXZhYjJLc2xHRkdvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFWS9TaUdoalREdmwrTzlpQU5hbFZCN2JnT0p2UWNJR0dPQ2g3R2pKVWtQMXhUc3NJcHdSSwpNL2pwRHdIdTNpeDhWVVhKa0U2VXZ2NUFKTWRCR1R3dTJ3PT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=
   ```
6. Setup NFD (+CR) and SRIOV NetOp
7. Apply:
   * https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/blob/feature/multi_node/configure-sriov/02_sriov_network_node_policy.yaml?ref_type=heads
   * https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/blob/feature/multi_node/configure-sriov/04_netattachdef.yaml?ref_type=heads
8. After system is ready (sriovnetworknodestate in ready state), run `echo 6 > /sys/class/net/ens2f0np0/device/sriov_numvfs` on both OCP hosts
9. Login to DPU nodes and setup hugepages
10. Setup SF comm channel (run configure_sf.sh script on DPU and IP on the node)
11. Build and push images with the following commands:
    1. `make ARCH=amd64 REGISTRY=harbor.mellanox.com/cloud-orchestration-dev/dpf/vremmas TAG=parameterize-ips generate`
    2. `make ARCH=amd64 REGISTRY=harbor.mellanox.com/cloud-orchestration-dev/dpf/vremmas TAG=parameterize-ips docker-build-all`
    3. `make ARCH=amd64 REGISTRY=harbor.mellanox.com/cloud-orchestration-dev/dpf/vremmas TAG=parameterize-ips docker-push-all`
12. Deploy stack using `kubectl apply -k config/operator/default`
13. Apply this CR:
    ```
    apiVersion: operator.dpf.nvidia.com/v1alpha1
    kind: DPFOperatorConfig
    metadata:
      name: dpfoperatorconfig
      namespace: dpf-operator-system
    spec:
      hostNetworkConfiguration:
        hostPF0: ens2f0np0
        hostPF0VF0: enp23s0f0v0
        CIDR: 192.168.1.0/24
        hosts:
        - hostClusterNodeName: dpf-worker-0
          dpuClusterNodeName: co-node-34-dpu-oob
          hostIP: 192.168.1.1/24
          dpuIP: 192.168.1.10/24
          gateway: 192.168.1.1
        - hostClusterNodeName: dpf-worker-1
          dpuClusterNodeName: co-node-35-dpu-oob
          hostIP: 192.168.1.2/24
          dpuIP: 192.168.1.11/24
          gateway: 192.168.1.1
    ```

### Tests

1. Apply https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/blob/feature/multi_node/configure-sriov/05_testpod.yaml?ref_type=heads
2. Assuming the following table:

   | **IP / Port** | **Entity**                                     |
   |---------------|------------------------------------------------|
   | 10.209.86.153 | worker0                                        |
   | 10.209.86.154 | worker1                                        |
   | 10.131.0.46   | Pod running on worker0                         |
   | 10.130.0.15   | Pod running on worker1                         |
   | 172.30.133.42 | SVC ClusterIP backing the 2 pods               |
   | 31187         | NodePort of SVC backing the 2 pods             |
   | 5000          | Port that SVC ClusterIP and Pods are listening |

   Run the following tests:
   ```
   # From pod running on 10.209.86.153 (worker0)
   $ echo "hello_from_other_pod" | nc -vw1 10.130.0.15 5000
   $ echo "pod_to_cip" | nc -vw1 172.30.133.42 5000
   $ echo "pod_to_nodePort_same" | nc -vw1 10.209.86.153 31187
   # Doesn't work when we use the new annotation introduced with the patch, see Build custom OVN Kubernetes
   $ echo "pod_to_nodePort_diff" | nc -vw1 10.209.86.154 31187

   # From worker0
   $ echo "worker0_to_pod_same" | nc -vw1 10.131.0.46 5000
   $ echo "worker0_to_pod_diff" | nc -vw1 10.130.0.15 5000
   $ echo "worker0_to_cip" | nc -vw1 172.30.133.42 5000
   $ echo "worker0_to_nodePort_same" | nc -vw1 10.209.86.153 31187
   $ echo "worker0_to_nodePort_diff" | nc -vw1 10.209.86.154 31187
   ```

## Itai's cluster

This guide shows how to test the OVN Kubernetes installation flow on Itai's cluster

### Setup Environment

1. Ask Itai Levy to create 4.14 cluster
2. Flash BFB on both DPUs (PK cards)
   1. ocp-host-worker1-1 - rshim1
   2. ocp-host-worker1-2 - rshim0
3. Setup DPU cluster
   1. Use rshim to connect to the worker1-1 DPU (rshim1)
   2. Run the following commands:
      ```
      # Needed so that we can have access to the cluster and the DPU access to the internet. OOB is not connected in that
      # cluster
      $ ip addr add 10.0.120.151/24 dev enp3s0f0s0
      $ ip route add default via 10.0.120.254 dev enp3s0f0s0
      $ sed -i "s/nameserver.*/nameserver 10.0.110.254/" /etc/resolv.conf

      $ systemctl disable kubelet && systemctl stop kubelet
      $ curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--tls-san 10.0.120.151" K3S_NODE_NAME="dpu-worker1-1" sh -
      ```
   3. Run the following commands:
      ```
      # Needed so that we can have access to the cluster
      $ ip addr add 10.0.120.152/24 dev enp3s0f0s0
      $ ip route add default via 10.0.120.254 dev enp3s0f0s0
      $ sed -i "s/nameserver.*/nameserver 10.0.110.254/" /etc/resolv.conf

      $ systemctl disable kubelet && systemctl stop kubelet
      # Token can be found at DPU node 1 under `/var/lib/rancher/k3s/server/node-token`
      $ curl -sfL https://get.k3s.io | K3S_NODE_NAME=dpu-worker1-2 K3S_TOKEN=K10b146fe2ee9ef991e752855bc328490f7b0fdab026b9185c8f31faaac61b32b2b::server:15b63166152de9034ab1371351b2c663 K3S_URL=https://10.0.120.151:6443 sh -
      ```
      5. Create secret for DPU cluster on the host cluster:
   ```
   apiVersion: v1
   kind: Secret
   metadata:
     name: dpu-admin-kubeconfig
     labels:
       kamaji.clastix.io/component: admin-kubeconfig
       kamaji.clastix.io/project: kamaji
       kamaji.clastix.io/name: dpu
   stringData:
     # Replace kubeconfig with what's under /etc/rancher/k3s/k3s.yaml
     admin.conf: |-
       apiVersion: v1
       clusters:
       - cluster:
           certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTVRJMk5USXdPVGt3SGhjTk1qUXdOREE1TURnME1UTTVXaGNOTXpRd05EQTNNRGcwTVRNNQpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTVRJMk5USXdPVGt3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUQ09FN0dUVWtJSXZ6S05HRm9jZGwxaXFoMEZSVkQ3K2FNKys4SHJMdmcKalVHWTJrb0VRY09sQjFiT3gyTHFJTlFRU1JSc3IrdWR6ZHFzZDkyUDBRaVlvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVWN1NnBUM0RWNjJNcVIxQW0ydk9UCnpueVgzSnN3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUloQU5uaXV0Rnl2OTJreFpMSUJ2SGNjY3lFbm43MG5PQUoKWkJ1dmQzaHVzdU9NQWlBT1lHTUdnaS9QWE95Vlk3SFI0dnRON0FKRWNzRTMzZHV4V0VlLy9OMFdEQT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
           # Replace 127.0.0.1 with IP of the control plane node
           server: https://10.0.120.151:6443
         name: default
       contexts:
       - context:
           cluster: default
           user: default
         name: default
       current-context: default
       kind: Config
       preferences: {}
       users:
       - name: default
         user:
           client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJrRENDQVRlZ0F3SUJBZ0lJZUlPbUFvMk1vSWt3Q2dZSUtvWkl6ajBFQXdJd0l6RWhNQjhHQTFVRUF3d1kKYXpOekxXTnNhV1Z1ZEMxallVQXhOekV5TmpVeU1EazVNQjRYRFRJME1EUXdPVEE0TkRFek9Wb1hEVEkxTURRdwpPVEE0TkRFek9Wb3dNREVYTUJVR0ExVUVDaE1PYzNsemRHVnRPbTFoYzNSbGNuTXhGVEFUQmdOVkJBTVRESE41CmMzUmxiVHBoWkcxcGJqQlpNQk1HQnlxR1NNNDlBZ0VHQ0NxR1NNNDlBd0VIQTBJQUJHUDBvaG9ZMHc3NWZqdlkKZ0RXcFZRZTI0RGliMEhDQmhqZ29leG95VkpEOWNVN0xDS2NFU2pQNDZROEI3dDRzZkZWRnlaQk9sTDcrUUNUSApRUms4THR1alNEQkdNQTRHQTFVZER3RUIvd1FFQXdJRm9EQVRCZ05WSFNVRUREQUtCZ2dyQmdFRkJRY0RBakFmCkJnTlZIU01FR0RBV2dCUzkxdEFad1ByRXZlMGlYYTUzTkJCN0I2dlhxREFLQmdncWhrak9QUVFEQWdOSEFEQkUKQWlCQlZEbE12UXB1eVJsZDZTd0FyYW5uT1ZzNmZCakI4eG5XdWlIa0ZLZjdXZ0lnWUlQL0xTdXNkK1c2R3JkNAphQ1BwVjdmdTYvMlJMNDlhSkFSdEJwYm43RXc9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0KLS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJlRENDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdFkyeHAKWlc1MExXTmhRREUzTVRJMk5USXdPVGt3SGhjTk1qUXdOREE1TURnME1UTTVXaGNOTXpRd05EQTNNRGcwTVRNNQpXakFqTVNFd0h3WURWUVFEREJock0zTXRZMnhwWlc1MExXTmhRREUzTVRJMk5USXdPVGt3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFSWnROekNFM1V3QlduSjkrenhiWSt6VDlZM2Jqd3RYQXJ1YUlQb2xqMTAKU0FvVE13bDBDRXAwbjJjZEcxa1k3ZDRXamZNRG84bHlQQzlSNUpjVk1UVmRvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVXZkYlFHY0Q2eEwzdElsMnVkelFRCmV3ZXIxNmd3Q2dZSUtvWkl6ajBFQXdJRFNRQXdSZ0loQUxnanFrNWlZaTRCZDBzZUZ1QnhXQ1lhbVMvbkVFUjcKRTdTdDYyYjlwbDZUQWlFQXJkeTBPRTZHb0hwOWJDU0Y2cVRlM0JyT01YTmJYVkFmZTRjeVFISkFlUk09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
           client-key-data: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSUpudGpEcWVjRVJDZE9oQlFJaWI1Y0pFT0thMDFsM3ZUWXZhYjJLc2xHRkdvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFWS9TaUdoalREdmwrTzlpQU5hbFZCN2JnT0p2UWNJR0dPQ2g3R2pKVWtQMXhUc3NJcHdSSwpNL2pwRHdIdTNpeDhWVVhKa0U2VXZ2NUFKTWRCR1R3dTJ3PT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=
   ```
4. Setup NFD (+CR) via OLM and latest community SRIOV NetOp (w/ Yury's switchdev refactoring patches)
5. Apply:
   * https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/blob/feature/multi_node_itai/configure-sriov/02_sriov_network_node_policy.yaml?ref_type=heads
   * https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/blob/feature/multi_node_itai/configure-sriov/04_netattachdef.yaml?ref_type=heads
6. Ensure nodes have resources for VFs
7. Login to DPU nodes and setup hugepages
8. Setup SF comm channel (run configure_sf.sh script on DPU and IP on the node)
9. Build and push images with the following commands:
    1. `make ARCH=amd64 REGISTRY=harbor.mellanox.com/cloud-orchestration-dev/dpf/vremmas TAG=v0.0.0-$(git rev-parse HEAD) generate`
    2. `make ARCH=amd64 REGISTRY=harbor.mellanox.com/cloud-orchestration-dev/dpf/vremmas TAG=v0.0.0-$(git rev-parse HEAD) docker-build-all`
    3. `make ARCH=amd64 REGISTRY=harbor.mellanox.com/cloud-orchestration-dev/dpf/vremmas TAG=v0.0.0-$(git rev-parse HEAD) docker-push-all`
10. Deploy stack using:
    ```
    $ kubectl create namespace dpf-operator-system
    $ operator-sdk run bundle --namespace dpf-operator-system harbor.mellanox.com/cloud-orchestration-dev/dpf/vremmas/dpf-operator-bundle:0.0.0-$(git rev-parse HEAD)
    ```
11. Apply this CR:
    ```
    apiVersion: operator.dpf.nvidia.com/v1alpha1
    kind: DPFOperatorConfig
    metadata:
      name: dpfoperatorconfig
      namespace: dpf-operator-system
    spec:
      hostNetworkConfiguration:
        hostPF0: enp63s0f0np0
        hostPF0VF0: enp63s0f0v0
        CIDR: 10.0.120.0/24
        hosts:
        - hostClusterNodeName: ocp-host-worker1-1
          dpuClusterNodeName: dpu-worker-1-1
          hostIP: 10.0.120.10/24
          dpuIP: 10.0.120.20/24
          gateway: 10.0.120.254
        - hostClusterNodeName: ocp-host-worker1-2
          dpuClusterNodeName: dpu-worker-1-2
          hostIP: 10.0.120.11/24
          dpuIP: 10.0.120.21/24
          gateway: 10.0.120.254
    ```
12. At some point, you'll notice that the DPF Operator has lost access to the DPU cluster. To enable that you need to
    do the following in both DPUs by connecting using rshim:
    ```
    $ ovs-vsctl add-port br-ovn en3f0pf0sf0
    ```
13. After the OVN Kubernetes pods are running, you can revert the IPs added on the SFs above (step 3) and remove the
    sf from the br-ovn bridge. This will ensure a clean setup with no duplicate routes. However, you'll lose access to
    the DPU cluster.


### Tests

1. Apply https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/blob/feature/multi_node_itai/configure-sriov/05_testpod.yaml?ref_type=heads
2. Assuming the following table:

   | **IP / Port** | **Entity**                                     |
   |---------------|------------------------------------------------|
   | 10.0.110.11   | ocp-host-worker-1-1                            |
   | 10.0.110.12   | ocp-host-worker-1-2                            |
   | 10.131.0.46   | Pod running on ocp-host-worker-1-1             |
   | 10.130.0.15   | Pod running on ocp-host-worker-1-2             |
   | 10.128.0.15   | Pod running on control-plane node              |
   | 172.30.133.42 | SVC ClusterIP backing the 2 pods               |
   | 31187         | NodePort of SVC backing the 2 pods             |
   | 5000          | Port that SVC ClusterIP and Pods are listening |

   Run the following tests:
   ```
   # From pod running on 10.0.110.11 (ocp-host-worker-1-1)
   $ echo "hello_from_other_worker_pod" | nc -vw1 10.130.0.15 5000
   $ echo "hello_from_worker_pod" | nc -vw1 10.128.0.15 5000
   $ echo "pod_to_cip" | nc -vw1 172.30.133.42 5000
   $ echo "pod_to_nodePort_same" | nc -vw1 10.209.86.153 31187
   # Doesn't work when we use the new annotation introduced with the patch, see Build custom OVN Kubernetes
   $ echo "pod_to_nodePort_diff" | nc -vw1 10.209.86.154 31187

   # From ocp-host-worker-1-1
   $ echo "worker-1-1_to_worker_pod_same" | nc -vw1 10.131.0.46 5000
   $ echo "worker-1-1_to_worker_pod_diff" | nc -vw1 10.130.0.15 5000
   $ echo "worker-1-1_to_cp_pod" | nc -vw1 10.128.0.15 5000
   $ echo "worker-1-1_to_cip" | nc -vw1 172.30.133.42 5000
   $ echo "worker-1-1_to_nodePort_same" | nc -vw1 10.209.86.153 31187
   $ echo "worker-1-1_to_nodePort_diff" | nc -vw1 10.209.86.154 31187
   ```
