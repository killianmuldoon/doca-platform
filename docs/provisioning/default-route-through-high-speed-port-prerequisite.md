# Default Route Management Prerequisite

## Overview
This guide outlines the configuration required to dynamically manage the default network route on a worker node. The goal is to ensure optimal traffic flow through a high-speed DPU port for hardware offloading, while maintaining fallback capabilities to the management interface. This configuration helps maintain a symmetric traffic path, particularly for traffic flows such as external requests to NodePort or LoadBalancer (LB) services, which are routed to host network pods.

## Assumption
This configuration is designed to be applied during the host provisioning phase, before the node is integrated into the Kubernetes cluster.

### Default Route Managemnet Configuration
The solution involves deploying a systemd service that periodically checks the reachability of the management network via the high-speed port (DPU) by pinging its default gateway IP address. Based on the results of this check, the systemd service dynamically adjusts the default route to prioritize the high-speed port when available, or default back to the management interface.

### Limitations
The service is designed to operate on hosts with a single DPU.

This service cannot run if any of the following conditions are true:
1. No Default Route Exists: The service requires at least one default route to function.
2. No Metric for Default Route: If the default route has no metric (i.e., metric 0), typically in cases where the default route is static, the service will not work.
3. Lowest Possible Metric (1): If the original default route has the lowest possible metric (1), the service cannot assign a higher-priority route.

### Deployment Steps
#### 1. Create the systemd Service File
```sudo vim /etc/systemd/system/manage-default-route.service```

Add the following content to the file:
```
[Unit]
Description=Check Network Reachability and Set Default Route
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/check-high-speed-conn.sh
Restart=always
RestartSec=1
Type=simple

[Install]
WantedBy=multi-user.target
```
#### 2. Create the Route Management Script
```sudo vim /usr/local/bin/check-high-speed-conn.sh```

Include the following logic:
```bash
#!/bin/bash

set -euo pipefail

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

get_interface_dhcp_ip() {
    local iface=$1
    local ip_address

    # Extract the dynamic IP address of the interface
    ip_address=$(ip addr show "$iface" | grep "dynamic" | awk '/inet / {print $2}' | cut -d/ -f1)

    # Check if the IP address is empty, return "" if no IP is found
    if [[ -z "$ip_address" ]]; then
        echo ""
    else
        echo "$ip_address"
    fi
}

get_default_gateway() {
    local iface=$1
    ip route | awk -v iface="$iface" '$0 ~ iface && /default/ {print $3}'
}

get_default_gateway_or_ip() {
    local iface=$1

    # Check if it's a default route and get the gateway
    local default_gateway=$(ip route | awk -v iface="$iface" '$0 ~ iface && /default/ {print $3}')

    if [[ -n "$default_gateway" ]]; then
        echo "$default_gateway"
        return 0
    fi

    # If no default gateway, check for specific routes.
    local route_ip=$(ip route | awk -v iface="$iface" '$0 ~ iface && !/\/.*src/ {print $1}' || true)

    if [[ -n "$route_ip" ]]; then
        echo "$route_ip"
        return 0
    fi

    logger "No matching route found for interface $iface"
    return 1
}

add_default_route() {
    local interface=$1
    local metric=$2
    local gateway=$3
    local retries=5  # Number of retries
    local attempt=0

    logger "INFO: Adding default route via $gateway on interface $interface with metric $metric..."

    while (( attempt < retries )); do
        output=$(ip route add default dev "$interface" via "$gateway" metric "$metric" 2>&1) || EXIT_STATUS=$?

        if [[ ${EXIT_STATUS:-0} -eq 0 ]]; then
            logger "INFO: Default route added successfully."
            return 0
        fi

        if [[ "${output}" == *"RTNETLINK answers: File exists"* ]]; then
            logger "INFO: Default route already exists"
            return 0
        fi

        if [[ ${EXIT_STATUS:-0} -eq 2 ]]; then
            logger "WARNING: RTNETLINK error 2 (No such file or directory). Retrying in 1 second..."
            sleep 1
            ((attempt++))
        else
            logger "ERROR: Failed to add default route. Exit status: ${EXIT_STATUS}. Output: ${output}"
            exit ${EXIT_STATUS}
        fi
    done

    logger "ERROR: Failed to add default route after $retries attempts."
    exit 2
}

remove_default_route() {
    local interface=$1
    logger "INFO: Removing all default routes for interface $interface"

    ip route show default dev "$interface" | while read -r route; do
        ip route del $route
        logger "INFO: Removed route: $route"
    done
}

route_exists() {
    local interface=$1
    ip route show | grep -q "default .* dev $interface"
}

need_new_metric() {
    local interface=$1
    local new_low_metric=$2

    # Extract the current metric for the default route on the specified interface
    current_metric=$(ip route show default dev "$interface" | awk '/default/ {print $NF; exit}')

    # Check if the current metric is higher than the new low metric
    if [[ -n "$current_metric" && "$current_metric" -gt "$new_low_metric" ]]; then
        return 0
    else
        return 1
    fi
}

check_first_default_route_metric() {
    # Try to get the first default route and check if it has a metric
    if ! ip route show default 2>/dev/null | head -n 1 | grep -q "metric"; then
        logger "ERROR: First default route does not have a metric or no default route found."
        exit 1
    fi
}

get_lowest_metric() {
    # Get the lowest metric or set to 100 if no default routes exist
    local current_lowest_metric

    # Get all default routes with metrics
    current_lowest_metric=$(ip route show default | awk '/default/ {for (i=1; i<=NF; i++) if ($i == "metric") print $(i+1); exit}')

    # If no default routes found, set metric to 100
    if [[ -z "${current_lowest_metric:-}" ]]; then
        current_lowest_metric=100
    fi

    # Check if the current lowest metric is 1 and exit with an error if true
    if [[ "$current_lowest_metric" -eq 1 ]]; then
        logger "ERROR: Current lowest metric is 1. Exiting."
        exit 1
    fi

    # Return the lowest metric
    echo "$current_lowest_metric"
}

HIGH_SPEED_INTERFACE=$(get_high_speed_interface)

if [[ -z "$HIGH_SPEED_INTERFACE" ]]; then
    logger "ERROR: No suitable interface found for pinging."
    exit 1
fi

PING_COUNT=2

check_first_default_route_metric

CURRENT_LOWEST_METRIC=$(get_lowest_metric)
TMP_LOWEST_METRIC=$((CURRENT_LOWEST_METRIC - 1))
TMP_LOWEST_METRIC=$((TMP_LOWEST_METRIC < 1 ? 1 : TMP_LOWEST_METRIC))
NEW_LOW_METRIC=$TMP_LOWEST_METRIC
NEW_HIGH_METRIC=$((TMP_LOWEST_METRIC + 2))

while true; do
    INTERFACE_IP=$(get_interface_dhcp_ip "$HIGH_SPEED_INTERFACE")

    if [[ -z "$INTERFACE_IP" ]]; then
        logger "WARNING: No IP address assigned to $HIGH_SPEED_INTERFACE."
        if route_exists "$HIGH_SPEED_INTERFACE"; then
            remove_default_route "$HIGH_SPEED_INTERFACE"
            logger "INFO: No INTERFACE_IP:  Removed default route via $HIGH_SPEED_INTERFACE."
        fi
        sleep 2
        continue
    fi

    DEFAULT_GATEWAY=$(get_default_gateway_or_ip "$HIGH_SPEED_INTERFACE")

    if [[ -z "$DEFAULT_GATEWAY" ]]; then
        logger "WARNING: No default gateway found for $HIGH_SPEED_INTERFACE."
        if route_exists "$HIGH_SPEED_INTERFACE"; then
            remove_default_route "$HIGH_SPEED_INTERFACE"
            logger "INFO: No DEFAULT_GATEWAY:  Removed default route via $HIGH_SPEED_INTERFACE."
        fi
        sleep 2
        continue
    fi

    if ping -I "$INTERFACE_IP" -B -c "$PING_COUNT" "$DEFAULT_GATEWAY" > /dev/null; then
        logger "INFO: Default gateway $DEFAULT_GATEWAY is reachable via $HIGH_SPEED_INTERFACE."
        if ! route_exists "$HIGH_SPEED_INTERFACE"; then
            add_default_route "$HIGH_SPEED_INTERFACE" "$NEW_LOW_METRIC" "$DEFAULT_GATEWAY"
            logger "INFO: Added default route via $HIGH_SPEED_INTERFACE with metric $NEW_LOW_METRIC."
        else
            if need_new_metric "$HIGH_SPEED_INTERFACE" "$NEW_LOW_METRIC"; then
            remove_default_route "$HIGH_SPEED_INTERFACE"
            sleep 1
            add_default_route "$HIGH_SPEED_INTERFACE" "$NEW_LOW_METRIC" "$DEFAULT_GATEWAY"
            logger "INFO: Added default route via $HIGH_SPEED_INTERFACE with metric $NEW_LOW_METRIC."
            else
            logger "INFO: Default route via $HIGH_SPEED_INTERFACE already exists."
            fi
        fi
    else
        logger "WARNING: Default gateway $DEFAULT_GATEWAY is not reachable via $HIGH_SPEED_INTERFACE."
        if route_exists "$HIGH_SPEED_INTERFACE"; then
            remove_default_route "$HIGH_SPEED_INTERFACE"
            sleep 1
            add_default_route "$HIGH_SPEED_INTERFACE" "$NEW_HIGH_METRIC" "$DEFAULT_GATEWAY"
            logger "INFO: not reachable: Replaced default route via $HIGH_SPEED_INTERFACE to metric "$NEW_HIGH_METRIC"."
        else
            logger "INFO: No default route to remove for $HIGH_SPEED_INTERFACE."
        fi
    fi

    sleep 2

done
```
Make the script executable:

```sudo chmod +x /usr/local/bin/check-high-speed-conn.sh```

#### 3. Enable and Start the Service
```bash
sudo systemctl daemon-reload
sudo systemctl enable manage-default-route.service
sudo systemctl start manage-default-route.service
```
You can verify the service status with:
```bash
sudo systemctl status manage-default-route.service
sudo journalctl -u manage-default-route.service
```