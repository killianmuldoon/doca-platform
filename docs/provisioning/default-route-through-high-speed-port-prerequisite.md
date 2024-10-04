# Default Route Management Prerequisite

## Overview
To dynamically manage the default network route on a Kubernetes (x86) worker node, ensuring optimal traffic flow through a high-speed port (DPU) for hardware offloading, while maintaining fallback capabilities to the management interface. Ensures a symmetric path for traffic flows, such as external requests to NodePort/LB services leading to host network pods.

## Assumption
This configuration is intended to be implemented during the host provisioning phase, prior to its integration into the Kubernetes cluster.

### Default Route Managemnet Configuration
The solution involves a systemd service that periodically checks the reachability of the management network via the high-speed port by curling NGC address.
If the high-speed port is reachable, the service sets it as the default route with a lower metric (indicating higher priority).
If the high-speed port is not reachable, the service defaults to using the management interface for routing.

_Note:_
This service will not work if the original default route the possible lowest metric (1).

## route-management Service
systemd service ```/etc/systemd/system/route-management.service```:
```
[Unit]
Description=Check Network Reachability and Set Default Route
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/check-high-speed-conn.sh
Restart=always
Type=simple

[Install]
WantedBy=multi-user.target
```

```/usr/local/bin/check-high-speed-conn.sh```:
```
#!/bin/bash

# Function to find the high-speed network interface
get_high_speed_interface() {
    for interface in /sys/class/net/*f0np0; do
        if [[ -d "$interface" ]]; then
            local iface_name
            iface_name=$(basename "$interface")
            local device_id
            device_id=$(cat "/sys/class/net/$iface_name/device/device")
            if [[ "$device_id" == "0xa2dc" ]]; then
                echo "$iface_name"
                return 0
            fi
        fi
    done
    echo ""
}

# Function to get IP address of a given interface
get_interface_dhcp_ip() {
    local iface=$1
    ip addr show "$iface" | grep "dynamic" | awk '/inet / {print $2}' | cut -d/ -f1
}

# Function to get the default gateway for a given interface's subnet
get_default_gateway() {
    local iface=$1
    ip route | awk -v iface="$iface" '$0 ~ iface && /default/ {print $3}'
}

# Function that adjusts the metric of a default route
adjust_default_route_metric() {
   local interface=$1
   local metric=$2
    local gateway=$3

   remove_default_route $1
   sleep 2
   add_default_route $1 $2 $3
}

# Function to add default route with a specified metric
add_default_route() {
    local interface=$1
    local metric=$2
    local gateway=$3
    ip route add default dev "$interface" via "$gateway" metric "$metric"
}

# Function to remove default route
remove_default_route() {
    local interface=$1
    ip route del default dev "$interface"
}

# Function to check if a default route exists for an interface
route_exists() {
    local interface=$1
    local metric=$2
    ip route show | grep -q "default .* dev $interface metric $metric"
}

# Find the high-speed interface before entering the loop
HIGH_SPEED_INTERFACE=$(get_high_speed_interface)

if [[ -z "$HIGH_SPEED_INTERFACE" ]]; then
    logger "ERROR: No suitable interface found for pinging."
    exit 1
fi

PING_COUNT=2

CURRENT_METRIC=$(ip route show default | head -n 1 | awk -F 'metric' '{print $2}' | tr -d " ")
NEW_METRIC=$((CURRENT_METRIC - 1))
NEW_METRIC=$((NEW_METRIC < 1 ? 1 : NEW_METRIC))
NEW_LOW_METRIC=$NEW_METRIC
NEW_HIGH_METRIC=$((NEW_METRIC + 2))

while true; do
    INTERFACE_IP=$(get_interface_dhcp_ip "$HIGH_SPEED_INTERFACE")

    if [[ -z "$INTERFACE_IP" ]]; then
        logger "WARNING: No IP address assigned to $HIGH_SPEED_INTERFACE."
        sleep 2
        continue
    fi

    DEFAULT_GATEWAY=$(get_default_gateway "$HIGH_SPEED_INTERFACE")

    if [[ -z "$DEFAULT_GATEWAY" ]]; then
        logger "WARNING: No default gateway found for $HIGH_SPEED_INTERFACE."
        sleep 2
        continue
    fi

    if ping -I "$INTERFACE_IP" -c "$PING_COUNT" "$DEFAULT_GATEWAY" > /dev/null; then
        logger "INFO: Default gateway $DEFAULT_GATEWAY is reachable via $HIGH_SPEED_INTERFACE."
        if ! route_exists "$HIGH_SPEED_INTERFACE" "$NEW_LOW_METRIC"; then
            adjust_default_route_metric "$HIGH_SPEED_INTERFACE" "$NEW_LOW_METRIC" "$DEFAULT_GATEWAY"
            logger "INFO: Added default route via $HIGH_SPEED_INTERFACE with metric $NEW_LOW_METRIC."
        else
            logger "INFO: Default route via $HIGH_SPEED_INTERFACE already exists."
        fi
    else
        logger "ERROR: Default gateway $DEFAULT_GATEWAY is not reachable via $HIGH_SPEED_INTERFACE."
        if route_exists "$HIGH_SPEED_INTERFACE" "$NEW_HIGH_METRIC"; then
            adjust_default_route_metric "$HIGH_SPEED_INTERFACE" "$NEW_HIGH_METRIC" "$DEFAULT_GATEWAY"
            logger "INFO: Removed default route via $HIGH_SPEED_INTERFACE."
        else
            logger "INFO: No default route to remove for $HIGH_SPEED_INTERFACE."
        fi
    fi

    sleep 2

done
```
