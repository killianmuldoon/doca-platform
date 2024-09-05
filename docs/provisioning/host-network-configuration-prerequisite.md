# Host Network Configuration Prerequisite

## Overview
To facilitate connectivity between the Data Processing Unit (DPU) and the Kubernetes control plane (API server) and the DHCP server on the management network, a fundamental network configuration is necessary on the host. The appropriate solution may vary depending on the specific environment. This guide provides a basic example for a vanilla Kubernetes setup.

## Assumption
This configuration is intended to be implemented during the host provisioning phase, prior to its integration into the Kubernetes cluster.

### Host-DPU Network Diagram
The solution involves setting up a network bridge with an IP address and connecting the Out-of-Band (OOB) uplink (a 1 Gbps Ethernet port) to it. During the DPU provisioning process, a Virtual Function (VF) will be created for the DPU and connected to the bridge to enable connectivity.
```
             +--------------------------------+  +-------------+
             | Host                           |  | DPU         |
             |        +---------+             |  |             |
Management---|--OOB---|  br-dpu |--ensXf0vf0--|--|--pf0vf0     |
  Network    | uplink +---------+             |  | representor |
             | (No IP)    IP                  |  |             |
             |                                |  |             |
             +--------------------------------+  +-------------+
```
## Static IP Configuration
This approach assigns a static IP address to the created bridge.

### Steps to Create and Configure the Bridge:
1. Create br-dpu bridge
```
ip link add name br-dpu type bridge
```
2. Bring up br-dpu bridge
```
ip link set br-dpu up
```
3. Set an IP address for br-dpu bridge
```
ip addr add  <IP_Address/Netmask> dev br-dpu
```
4. Plug the OOB interface to br-dpu bridge. Assume eno1 is our OOB.
```
ip link set eno1 master br-dpu
```

## Dynamic IP Configuration (DHCP)
This approach relies on a DHCP server to reassign the IP address of the OOB interface to the br-dpu bridge. Network administrators can incorporate this configuration during host provisioning or after the host is provisioned.
### Assumption
The DHCP server is configured to assign IP addresses based on the MAC address of each device. This configuration relies on the MAC address as the DHCP identifier.
### Initial Host Provisioning
To integrate the required configuration into the host provisioning process, modify the cloud-init networking section in the ```/etc/cloud/cloud.cfg.d/*``` file as follows:
```
network:
    version: 2
    ethernets:
        $OOB_INTERFACE:
            dhcp4: no
            macaddress: $RANDOM_MAC
    bridges:
        $BRIDGE_NAME:
            interfaces: [$OOB_INTERFACE]
            dhcp4: yes
            critical: true
            dhcp-identifier: mac
            macaddress: $PERMANENT_MAC
```
_Note_: 
Network administrators should know the OOB interface and its MAC address in advance.

### Post-Provisioning Configuration
#### Example Solution for Vanilla Kubernetes
Utilize  [netplan](https://netplan.readthedocs.io/en/stable/) to configure the necessary network interfaces. Netplan runs at boot time before the kubelet process.

#### Netplan Configuration Script Workflow
1. Identify the host's OOB uplink interface via the default route.
2. Retrieve the permanent MAC address of the OOB interface.
3. Generate a random local MAC address for the OOB interface.
4. Create a Netplan configuration file for the bridge, designating the OOB uplink as a bridge interface. Disable DHCP on the OOB and enable it on the bridge. Assign the bridge the permanent MAC address of the OOB and the OOB a random local MAC.
```
network:
    version: 2
    ethernets:
        $OOB_INTERFACE:
            dhcp4: no
            macaddress: $RANDOM_MAC
    bridges:
        $BRIDGE_NAME:
            interfaces: [$OOB_INTERFACE]
            dhcp4: yes
            critical: true
            dhcp-identifier: mac
            macaddress: $PERMANENT_MAC
```
5. Apply Netplan Configuration using ```netplan apply```
6. Ensure the DHCP IP is removed from the OOB uplink interface if it still exists.

##### Script
```setup-bridge.sh```:
```
#!/bin/bash

# Variables
BRIDGE_NAME="br-dpu"
NETPLAN_CONFIG_FILE="/etc/netplan/99-bridge-cfg.yaml"

# Function to handle errors
error_exit() {
    echo "Error: $1"
    exit 1
}

# Check if the 'active-route' service is running
if systemctl is-active --quiet active-route; then
    echo "This script cannot run while the 'active-route' service is running."
    exit 1
fi

# Discover the host uplink via default route
OOB_INTERFACE=$(ip route | awk '/default/ && /proto dhcp/ {print $5; exit}')
if [ -z "$OOB_INTERFACE" ]; then
    error_exit "Failed to discover host OOB uplink."
fi

echo "Discovered host uplink: $OOB_INTERFACE."

# Check if the script is run as root
if [ "$EUID" -ne 0 ]; then
    error_exit "This script must be run as root. Use sudo."
fi

# Get the permanent MAC address of the OOB interface
PERMANENT_MAC=$(ethtool -P $OOB_INTERFACE | awk '{print $3}')
if [ -z "$PERMANENT_MAC" ]; then
    error_exit "Failed to get the permanent MAC address of $OOB_INTERFACE."
fi

echo "Permanent MAC address of $OOB_INTERFACE: $PERMANENT_MAC."

# Generate a local random MAC address for the OOB interface
generate_local_mac() {
    hexchars="0123456789ABCDEF"
    echo "02:$(for i in {1..5}; do echo -n ${hexchars:$(( $RANDOM % 16 )):1}${hexchars:$(( $RANDOM % 16 )):1}; [ $i -lt 5 ] && echo -n ":"; done)"
}

RANDOM_MAC=$(generate_local_mac)
echo "Generated random MAC address for $OOB_INTERFACE: $RANDOM_MAC."

# Create a Netplan configuration file for the bridge
echo "Creating Netplan configuration..."
cat <<EOF | tee $NETPLAN_CONFIG_FILE > /dev/null
network:
    version: 2
    ethernets:
        $OOB_INTERFACE:
            dhcp4: no
            macaddress: $RANDOM_MAC
    bridges:
        $BRIDGE_NAME:
            interfaces: [$OOB_INTERFACE]
            dhcp4: yes
            critical: true
            dhcp-identifier: mac
            macaddress: $PERMANENT_MAC
EOF

if [ $? -ne 0 ]; then
    error_exit "Failed to create Netplan configuration file."
fi

chmod 600 $NETPLAN_CONFIG_FILE

# Apply the Netplan configuration
echo "Applying Netplan configuration..."
netplan apply

if [ $? -ne 0 ]; then
    error_exit "Failed to apply Netplan configuration."
fi

sleep 20

# Check and delete default route via OOB interface if it exists
DEFAULT_ROUTE_OOB=$(ip route | awk '/default/ && /'"$OOB_INTERFACE"'/ {print}')
if [ -n "$DEFAULT_ROUTE_OOB" ]; then
    echo "Deleting default route via OOB interface: $DEFAULT_ROUTE_OOB"
    ip route del default dev "$OOB_INTERFACE"
else
    echo "No default route via OOB interface found."
fi

# Check that there is a default route via the bridge interface
DEFAULT_ROUTE_BRIDGE=$(ip route | awk '/default/ && /'"$BRIDGE_NAME"'/ {print}')
if [ -z "$DEFAULT_ROUTE_BRIDGE" ]; then
    error_exit "No default route via bridge interface found. Please ensure network connectivity through the bridge."
else
    echo "Default route via bridge interface exists: $DEFAULT_ROUTE_BRIDGE"
fi

```
#### Reverting the Configuration
To revert the process:
1. Delete the generated netplan configuration file:
```
rm -rf /etc/netplan/$NETPLAN_CONFIG_FILE
```
2. Restore the permanent MAC address of the OOB interface:
```
ip link set dev $OOB_INTERFACE down
ip link set dev $OOB_INTERFACE address $PERMANENT_MAC
ip link set dev $OOB_INTERFACE up
```
3. Delete ```br-dpu``` bridge:
```
ip link delete br-dpu
```
4. Execute:
 ```
 netplan apply
 ```
### Integrating to cloud-init
To integrate this configuration as a part of cloud-init operations add:
```
  ###DPF host networking bash script
  - path: /usr/local/bin/dpf-host-networking-script.sh
    permissions: '0755'
    content: |
      #!/bin/bash

     # Variables
     BRIDGE_NAME="br-dpu"
     NETPLAN_CONFIG_FILE="/etc/netplan/99-bridge-cfg.yaml"

     # Function to handle errors
     error_exit() {
         echo "Error: $1"
         exit 1
     }

     # Check if the 'active-route' service is running
     if systemctl is-active --quiet active-route; then
         echo "This script cannot run while the 'active-route' service is running."
         exit 1
     fi

     # Discover the host uplink via default route
     OOB_INTERFACE=$(ip route | awk '/default/ && /proto dhcp/ {print $5; exit}')
     if [ -z "$OOB_INTERFACE" ]; then
         error_exit "Failed to discover host OOB uplink."
     fi

     echo "Discovered host uplink: $OOB_INTERFACE."

     # Get the permanent MAC address of the OOB interface
     PERMANENT_MAC=$(ethtool -P $OOB_INTERFACE | awk '{print $3}')
     if [ -z "$PERMANENT_MAC" ]; then
         error_exit "Failed to get the permanent MAC address of $OOB_INTERFACE."
     fi

     echo "Permanent MAC address of $OOB_INTERFACE: $PERMANENT_MAC."

     # Generate a local random MAC address for the OOB interface
     generate_local_mac() {
         hexchars="0123456789ABCDEF"
         echo "02:$(for i in {1..5}; do echo -n ${hexchars:$(( $RANDOM % 16 )):1}${hexchars:$(( $RANDOM % 16 )):1}; [ $i -lt 5 ] && echo -n ":"; done)"
     }

     RANDOM_MAC=$(generate_local_mac)
     echo "Generated random MAC address for $OOB_INTERFACE: $RANDOM_MAC."

     # Create a Netplan configuration file for the bridge
     echo "Creating Netplan configuration..."
     cat <<EOF | tee $NETPLAN_CONFIG_FILE > /dev/null
     network:
         version: 2
         ethernets:
             $OOB_INTERFACE:
                 dhcp4: no
                 macaddress: $RANDOM_MAC
         bridges:
             $BRIDGE_NAME:
                 interfaces: [$OOB_INTERFACE]
                 dhcp4: yes
                 critical: true
                 dhcp-identifier: mac
                 macaddress: $PERMANENT_MAC
     EOF

     if [ $? -ne 0 ]; then
         error_exit "Failed to create Netplan configuration file."
     fi

     chmod 600 $NETPLAN_CONFIG_FILE

     # Apply the Netplan configuration
     echo "Applying Netplan configuration..."
     netplan apply

     if [ $? -ne 0 ]; then
         error_exit "Failed to apply Netplan configuration."
     fi

     sleep 20

     # Check and delete default route via OOB interface if it exists
     DEFAULT_ROUTE_OOB=$(ip route | awk '/default/ && /'"$OOB_INTERFACE"'/ {print}')
     if [ -n "$DEFAULT_ROUTE_OOB" ]; then
         echo "Deleting default route via OOB interface: $DEFAULT_ROUTE_OOB"
         ip route del default dev "$OOB_INTERFACE"
     else
         echo "No default route via OOB interface found."
     fi

     # Check that there is a default route via the bridge interface
     DEFAULT_ROUTE_BRIDGE=$(ip route | awk '/default/ && /'"$BRIDGE_NAME"'/ {print}')
     if [ -z "$DEFAULT_ROUTE_BRIDGE" ]; then
         error_exit "No default route via bridge interface found. Please ensure network connectivity through the bridge."
     else
         echo "Default route via bridge interface exists: $DEFAULT_ROUTE_BRIDGE"
     fi


  ###DPF host networking systemd service
  - path: /etc/systemd/system/dpf-host-networking-script.service
    permissions: '0644'
    content: |
      [Unit]
      Description=DPF Host Networking Script Runner
      After=systemd-networkd.service network-online.target
      Wants=network-online.target

      [Service]
      Type=oneshot
      ExecStart=/usr/local/bin/dpf-host-networking-script.sh
      Restart=on-failure
      RestartSec=5s

      [Install]
      WantedBy=multi-user.target
```