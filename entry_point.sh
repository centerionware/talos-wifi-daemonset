#!/bin/sh
set -e

CONFIGMAP_NAME="${HOSTNAME}-wifi-config"
WPA_CONF_PATH="/etc/wpa_supplicant/wpa_supplicant.conf"
TEMPLATE="# See: https://linux.die.net/man/5/wpa_supplicant.conf"
echo "Detecting WiFi interface..."
WIFI_IFACE=$(iwconfig 2>&1 | grep 'IEEE 802.11' | awk '{print $1; exit}')
CERT_PATH="/etc/cert"

# Define the paths for certificates
CA_CERT="$CERT_PATH/ca.pem"
CLIENT_CERT="$CERT_PATH/user.pem"
PRIVATE_KEY="$CERT_PATH/user.prv"

# Step 2: If no WiFi interface found, default to wlan0
if [ -z "$WIFI_IFACE" ]; then
    echo "No WiFi interface found. Defaulting to wlan0."
    WIFI_IFACE="wlan0"
fi

echo "Using WiFi interface: $WIFI_IFACE"

echo "Looking for configmap: $CONFIGMAP_NAME"

if ! kubectl get configmap "$CONFIGMAP_NAME" >/dev/null 2>&1; then
    echo "No existing configmap found. Checking kernel parameters..."

    SSID=""
    PASSWORD=""

    for param in $(cat /proc/cmdline); do
        case "$param" in
            --wifi-ssid=*)
                SSID="${param#*=}"
                ;;
            --wifi-password=*)
                PASSWORD="${param#*=}"
                ;;
        esac
    done

    if [ -n "$SSID" ] && [ -n "$PASSWORD" ]; then
        echo "Creating configmap from kernel parameters (SSID: $SSID)"

        cat <<EOF > /tmp/wpa_supplicant.conf
ctrl_interface=/var/run/wpa_supplicant
network={
    ssid="$SSID"
    psk="$PASSWORD"
}
EOF

        kubectl create configmap "$CONFIGMAP_NAME" --from-file=wpa_supplicant.conf=/tmp/wpa_supplicant.conf
    else
        echo "No SSID or PASSWORD found in kernel parameters, creating blank configmap from template"

        kubectl create configmap "$CONFIGMAP_NAME" --from-literal=wpa_supplicant.conf="$TEMPLATE" \
            --from-literal=wifi_interface="$WIFI_IFACE"
    fi
else
    echo "Configmap $CONFIGMAP_NAME already exists"
fi



echo "Checking for required certificates..."

# CA Certificate Secret
if ! kubectl get secret $HOSTNAME-wifi-ca-cert >/dev/null 2>&1; then
    if [ ! -f "$CA_CERT" ]; then
        echo "Error: CA certificate file $CA_CERT does not exist."
    fi
    kubectl create secret generic $HOSTNAME-wifi-ca-cert --from-literal=ca.pem=""
    echo "Created Kubernetes Secret for CA certificate."
else
    echo "CA certificate secret exists."
fi

# Client Certificate Secret
if ! kubectl get secret $HOSTNAME-wifi-client-cert >/dev/null 2>&1; then
    if [ ! -f "$CLIENT_CERT" ]; then
        echo "Error: Client certificate file $CLIENT_CERT does not exist."
    fi
    kubectl create secret generic $HOSTNAME-wifi-client-cert --from-literal=user.pem=""
    echo "Created Kubernetes Secret for Client certificate."
else
    echo "Client certificate secret exists."
fi

# Private Key Secret
if ! kubectl get secret $HOSTNAME-wifi-client-key >/dev/null 2>&1; then
    if [ ! -f "$PRIVATE_KEY" ]; then
        echo "Error: Private key file $PRIVATE_KEY does not exist."
    fi
    kubectl create secret generic $HOSTNAME-wifi-client-key --from-literal=user.prv=""
    echo "Created Kubernetes Secret for Private key."
else
    echo "Client key secret exists."
fi

echo "Fetching configmap and writing to $WPA_CONF_PATH"

kubectl get configmap "$CONFIGMAP_NAME" -o jsonpath='{.data.wpa_supplicant\.conf}' > "$WPA_CONF_PATH"

WIFI_IFACE=$(kubectl get configmap "$CONFIGMAP_NAME" -o jsonpath='{.data.wifi_interface}')

if [ -z "$WIFI_IFACE" ]; then
    echo "Error: wifi_interface not found in configmap"
    exit 1
fi

WIFI_IFACE=$(kubectl get configmap "$CONFIGMAP_NAME" -o jsonpath='{.data.wifi_interface}')

if [ -z "$WIFI_IFACE" ]; then
    echo "Error: wifi_interface not found in configmap"
    exit 1
fi

# Step 9: Mount the secrets into the appropriate files
echo "Mounting the secrets into files..."
mkdir $CERT_PATH || true
# Read the secrets and write them to their respective files
kubectl get secret $HOSTNAME-wifi-ca-cert -o jsonpath='{.data.ca\.pem}' | base64 -d > "$CA_CERT"
kubectl get secret $HOSTNAME-wifi-client-cert -o jsonpath='{.data.user\.pem}' | base64 -d > "$CLIENT_CERT"
kubectl get secret $HOSTNAME-wifi-client-key -o jsonpath='{.data.user\.prv}' | base64 -d > "$PRIVATE_KEY"

/sbin/wpa_supplicant -i "$WIFI_IFACE" -c "$WPA_CONF_PATH"

# Keep container running (or replace with desired behavior)
tail -f /dev/null
