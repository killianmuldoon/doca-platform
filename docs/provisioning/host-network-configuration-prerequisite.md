# Host Network Configuration Prerequisite

## Overview
To facilitate connectivity between the Data Processing Unit (DPU) and the Kubernetes control plane (API server) and the DHCP server on the management network, a fundamental network configuration is necessary on the host. The appropriate solution may vary depending on the specific environment. This guide provides a basic example for a host with Ubuntu 24.04 OS.

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
This configuration can be done statically or using a DHCP server to assign an IP to the bridge.
### Example Solution for Ubuntu 24.04
Utilize  [netplan](https://netplan.readthedocs.io/en/stable/) to configure the necessary network interfaces. Netplan configuration files are executed at boot time before the kubelet process.

#### Netplan Configuration Script Workflow
1. Checks if the bridge configuration already exists.
2. Identify the host's OOB uplink interface via the default route.
3. Determines which network manager is active and checks the IP type (static or DHCP) of the OOB interface.
4. Configures Netplan based on whether the IP type is static or DHCP.
5. Applies the Netplan configuration.
6. Ensure the IP is removed from the OOB uplink interface if it still exists and that the default route is set to the bridge.

_Notes:_
1. This script was tested only on Ubuntu 24.04.
2. This script assumes only one default route is present.
3. This script uses NetworkManager or systemd-networkd.
4. This script supports both static and dynamic IP.
5. For the dynamic IP allocation, this script assumes that the DHCP server is configured to assign IP addresses based on the MAC address of each device. This configuration relies on the MAC address as the DHCP identifier.

#### Integrating to cloud-init
An example of how to integrate this configuration as a part of cloud-init operations:

```cloud-init.yml```:
```
#cloud-config

write_files:
  ###DPF host networking bash script
  - path: /usr/local/bin/dpf-host-networking-script.sh
    permissions: '0755'
    content: |
#!/bin/bash

set -euo pipefail

# Variables
BRIDGE_NAME="br-dpu"
NETPLAN_CONFIG_FILE="/etc/netplan/99-bridge-cfg.yaml"
NETWORK_MANAGER_CONN_NAME="netplan-br-dpu"

error_exit() {
    logger "Error: $1"
    exit 1
}

is_dhcp() {
    local ip_method=$1

    if [ "$ip_method" == "auto" ]; then
        echo "dhcp"
    else
        echo "static"
    fi
}

determine_ip_type() {
    local interface=$1

    if is_service_running "NetworkManager"; then
        # Check if the connection exists and get its IP method
        if nmcli connection show "$NETWORK_MANAGER_CONN_NAME" &>/dev/null; then
            local ip_method
            ip_method=$(nmcli connection show "$NETWORK_MANAGER_CONN_NAME" | grep -i 'ipv4.method' | awk '{print $2}')

            is_dhcp "$ip_method"
            return 0
        fi
    fi
    if is_service_running "systemd-networkd"; then
        # Check if the link is up and if the IP is DHCP
        local ip_method
        ip_method=$(networkctl status "$interface" 2>/dev/null | grep -i 'DHCP4' | grep -q 'via' && echo "auto" || echo "manual")

        is_dhcp "$ip_method"
        return 0

    else
        error_exit "Neither NetworkManager nor systemd-networkd is active."
    fi
}

is_bridge_configuration_exist() {
    if ip link show "$BRIDGE_NAME" &> /dev/null; then
        if ip route | grep -q "default.*$BRIDGE_NAME"; then
            if [ -f "$NETPLAN_CONFIG_FILE" ]; then
                logger "The bridge '$BRIDGE_NAME' already exists, a default route through it is present, and the Netplan config file exists. Exiting script."
                exit 0
            else
                error_exit "Netplan configuration file '$NETPLAN_CONFIG_FILE' does not exist, but the bridge and default route are present."
            fi
        else
            error_exit "No default route through the bridge '$BRIDGE_NAME' found, but the bridge exists."
        fi
    fi
}

is_service_running() {
    local service_name=$1
    if systemctl is-active --quiet "$service_name"; then
        return 0
    else
        return 1
    fi
}

get_oob_interface() {
    local interface=$(ip route | awk '/default/ {print $5; exit}')
    if [ -z "$interface" ]; then
        error_exit "Failed to discover host OOB uplink."
    fi
    echo "$interface"
}

get_current_ip() {
    local interface=$1
    local current_ip

    current_ip=$(ip addr show "$interface" | awk '/inet / {print $2}')
    if [ -z "$current_ip" ]; then
        error_exit "Failed to retrieve the IP address for interface $interface."
    fi

    echo "$current_ip"
}

get_default_gw_ip() {
    local default_gw_ip

    default_gw_ip=$(ip route | awk '/default/ {print $3}')
    if [ -z "$default_gw_ip" ]; then
        error_exit "Failed to retrieve the default gateway IP."
    fi

    echo "$default_gw_ip"
}

generate_local_mac() {
    hexchars="0123456789ABCDEF"
    echo "02:$(for i in {1..5}; do echo -n ${hexchars:$(( $RANDOM % 16 )):1}${hexchars:$(( $RANDOM % 16 )):1}; [ $i -lt 5 ] && echo -n ":"; done)"
}

is_bridge_configuration_exist

# Check if the 'active-route' service is running and exit if it is
if is_service_running "active-route"; then
    error_exit "This script cannot run while the 'active-route' service is running."
fi

# Check if the script is run as root
if [ "$EUID" -ne 0 ]; then
    error_exit "This script must be run as root. Use sudo."
fi

# Discover the host uplink interface via default route
OOB_INTERFACE=$(get_oob_interface)
logger "Discovered host uplink: $OOB_INTERFACE."

# Determine which network manager is active and check IP type accordingly
IP_TYPE=$(determine_ip_type "$OOB_INTERFACE")

# Configure Netplan based on IP type (static or DHCP)
if [ "$IP_TYPE" == "static" ]; then
    logger "Static IP Assignment"
    CURRENT_IP=$(get_current_ip "$OOB_INTERFACE")
    DEFAULT_GW_IP=$(get_default_gw_ip)

    # Delete the default route from the OOB_INTERFACE
    ip route del default dev "$OOB_INTERFACE" || error_exit "Failed to delete default route from $OOB_INTERFACE."

    # Remove the IP address from the OOB_INTERFACE
    ip addr del "$CURRENT_IP" dev "$OOB_INTERFACE" || error_exit "Failed to remove IP address from $OOB_INTERFACE."
    
    # Static IP configuration for Netplan with OOB's current IP for the bridge 
    cat <<EOF | tee $NETPLAN_CONFIG_FILE > /dev/null || error_exit "Failed to create Netplan configuration file."
network:
  version: 2
  ethernets:
      $OOB_INTERFACE:
          dhcp4: no
          addresses: []
  bridges:
      $BRIDGE_NAME:
          interfaces: [$OOB_INTERFACE]
          addresses: [$CURRENT_IP]
          critical: true
          routes:
          - to: default 
            via: $DEFAULT_GW_IP 
EOF

elif [ "$IP_TYPE" == "dhcp" ]; then
    
    logger "DHCP IP Assignment"
    
	# Get the permanent MAC address of the OOB interface for DHCP case only 
	PERMANENT_MAC=$(ethtool -P $OOB_INTERFACE | awk '{print $3}')
	if [ -z "$PERMANENT_MAC" ]; then 
		error_exit "Failed to get the permanent MAC address of $OOB_INTERFACE." 
	fi
	
	logger "Permanent MAC address of $OOB_INTERFACE: $PERMANENT_MAC."

	RANDOM_MAC=$(generate_local_mac)
	
	# DHCP configuration for Netplan with a random MAC for OOB and permanent MAC for bridge 
	cat <<EOF | tee $NETPLAN_CONFIG_FILE > /dev/null || error_exit "Failed to create Netplan configuration file."
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
      dhcp-identifier: mac 
      macaddress: $PERMANENT_MAC 
      critical: true 
EOF

else 
	error_exit "Unable to determine IP type (static or DHCP) for interface $OOB_INTERFACE." 
fi

chmod 600 $NETPLAN_CONFIG_FILE || error_exit "Failed to set permissions on Netplan configuration file."

# Apply the Netplan configuration and handle errors if any occur during application.
logger "Applying Netplan configuration..."
netplan apply || error_exit "Failed to apply Netplan configuration."

sleep 20

# Check and delete default route via OOB interface if it exists.
DEFAULT_ROUTE_OOB=$(ip route | awk '/default/ && /'"$OOB_INTERFACE"'/ {print}')
if [ -n "$DEFAULT_ROUTE_OOB" ]; then 
	logger "Deleting default route via OOB interface: $DEFAULT_ROUTE_OOB"
	ip route del default dev "$OOB_INTERFACE" || error_exit "Failed to delete default route via OOB interface." 
else 
	logger "No default route via OOB interface found." 
fi 

# Check that there is a default route via the bridge interface.
DEFAULT_ROUTE_BRIDGE=$(ip route | awk '/default/ && /'"$BRIDGE_NAME"'/ {print}')
if [ -z "$DEFAULT_ROUTE_BRIDGE" ]; then 
	error_exit "No default route via bridge interface found. Please ensure network connectivity through the bridge." 
else 
	logger "Default route via bridge interface exists: $DEFAULT_ROUTE_BRIDGE"
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

runcmd:
  - [ systemctl, start, dpf-host-networking-script.service ]
```

#### Reverting the Configuration
To revert the process, run the following script as root user:

```revert-host-network-configuration.sh```:
```
#!/bin/bash

set -euo pipefail

# Variables
BRIDGE_NAME="br-dpu"
NETPLAN_CONFIG_FILE="/etc/netplan/99-bridge-cfg.yaml"
OOB_INTERFACE=""
NETWORK_MANAGER_CONN_NAME="netplan-br-dpu"

# Function to handle errors
error_exit() {
    echo "Error: $1"
    exit 1
}

is_service_running() {
    local service_name=$1
    if systemctl is-active --quiet "$service_name"; then
        return 0
    else
        return 1
    fi
}

check_bridge_exists() {
    # Check if the bridge exists
    if ! ip link show "$BRIDGE_NAME" &>/dev/null; then
        error_exit "Bridge $BRIDGE_NAME does not exist."
    fi
}

get_oob_interface() {
    OOB_INTERFACE=$(ip link show master "$BRIDGE_NAME" | awk -F: '/^[0-9]+:/{print $2}' | tr -d ' ')
    if [ -z "$OOB_INTERFACE" ]; then
        error_exit "Failed to determine OOB interface from bridge $BRIDGE_NAME."
    fi
}

is_dhcp() {
    local ip_method=$1

    if [ "$ip_method" == "auto" ]; then
        echo "dhcp"
    else
        echo "static"
    fi

    return 0
}

determine_ip_type() {
    local interface=$1

    if is_service_running "NetworkManager"; then
        # Check if the connection exists and get its IP method
        if nmcli connection show "$NETWORK_MANAGER_CONN_NAME" &>/dev/null; then
            local ip_method
            ip_method=$(nmcli connection show "$NETWORK_MANAGER_CONN_NAME" | grep -i 'ipv4.method' | awk '{print $2}')

            is_dhcp "$ip_method"
            return 0
        fi
    fi
    if is_service_running "systemd-networkd"; then
        # Check if the link is up and if the IP is DHCP
        local ip_method
        ip_method=$(networkctl status "$interface" 2>/dev/null | grep -i 'DHCP4' | grep -q 'via' && echo "auto" || echo "manual")

        is_dhcp "$ip_method"
        return 0

    else
        error_exit "Neither NetworkManager nor systemd-networkd is active."
    fi
}

get_current_ip() {
    local interface=$1
    local current_ip

    current_ip=$(ip addr show "$interface" | awk '/inet / {print $2}')
    if [ -z "$current_ip" ]; then
        error_exit "Failed to retrieve the IP address for interface $interface."
    fi

    echo "$current_ip"
}

get_default_gw_ip() {
    local default_gw_ip

    default_gw_ip=$(ip route | awk '/default/ {print $3}')
    if [ -z "$default_gw_ip" ]; then
        error_exit "Failed to retrieve the default gateway IP."
    fi

    echo "$default_gw_ip"
}

revert_static_ip() {
    echo "Reverting static IP configuration..."
    
    CURRENT_IP=$(get_current_ip "$BRIDGE_NAME")
    STATIC_GATEWAY_IP=$(get_default_gw_ip)
    
    # Bring down and delete the bridge
    ip link set "$BRIDGE_NAME" down || error_exit "Failed to bring down bridge $BRIDGE_NAME."
    ip link delete "$BRIDGE_NAME" type bridge || error_exit "Failed to delete bridge $BRIDGE_NAME."
    
    # Add IP to OOB Interface
    ip addr add "$CURRENT_IP" dev "$OOB_INTERFACE" || error_exit "Failed to add IP $CURRENT_IP to $OOB_INTERFACE."
    ip link set dev "$OOB_INTERFACE" up
    
    # Restore default route on OOB interface
    ip route add default via "$STATIC_GATEWAY_IP" dev "$OOB_INTERFACE" || error_exit "Failed to add default route via $STATIC_GATEWAY_IP on $OOB_INTERFACE."

    # Remove Netplan configuration file
    if [ -f "$NETPLAN_CONFIG_FILE" ]; then
        rm "$NETPLAN_CONFIG_FILE" || error_exit "Failed to remove Netplan configuration file."
    else
        echo "Netplan configuration file does not exist, skipping removal."
    fi

    # Apply Netplan changes
    netplan apply || error_exit "Failed to apply Netplan configuration."
}

revert_dhcp_ip() {
    echo "Reverting DHCP IP configuration..."

    # Bring down and delete the bridge
	ip link set "$BRIDGE_NAME" down || error_exit "Failed to bring down bridge $BRIDGE_NAME."
	ip link delete "$BRIDGE_NAME" type bridge || error_exit "Failed to delete bridge $BRIDGE_NAME."

	# Restore permanent MAC address on OOB_INTERFACE
	PERMANENT_MAC=$(ethtool -P $OOB_INTERFACE | awk '{print $3}')
	if [ -z "$PERMANENT_MAC" ]; then
		error_exit "Failed to get the permanent MAC address of $OOB_INTERFACE."
	fi
	
	ip link set "$OOB_INTERFACE" down || error_exit "Failed to bring down OOB $OOB_INTERFACE."
	ip link set dev "$OOB_INTERFACE" address "$PERMANENT_MAC" || error_exit "Failed to set permanent MAC address on $OOB_INTERFACE."
	ip link set "$OOB_INTERFACE" up || error_exit "Failed to bring up OOB $OOB_INTERFACE."

	# Remove Netplan configuration file
	if [ -f "$NETPLAN_CONFIG_FILE" ]; then
		rm "$NETPLAN_CONFIG_FILE" || error_exit "Failed to remove Netplan configuration file."
	else
		echo "Netplan configuration file does not exist, skipping removal."
	fi

	# Apply Netplan changes
	netplan apply || error_exit "Failed to apply Netplan configuration."
}

check_bridge_exists

# Get OOB interface from the bridge before proceeding with other operations.
get_oob_interface

# Determine IP type and revert accordingly.
IP_TYPE=$(determine_ip_type "$BRIDGE_NAME")

if [ "$IP_TYPE" == "static" ]; then
	revert_static_ip
elif [ "$IP_TYPE" == "dhcp" ]; then
	revert_dhcp_ip
else 
	error_exit "Unable to determine IP type (static or DHCP) for interface $BRIDGE_NAME."
fi
```
