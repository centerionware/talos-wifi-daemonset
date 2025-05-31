# talos-wifi

To enable WiFi with Talos a kernel must be compiled with the wireless modules enabled. Linux should be able to automatically detect if any are used and setup the environment

The test_deployment file will be erased. This is meant to run as a daemonset.

The idea is it will use kernel parameters if found to generate an initial wpa_supplicant configuration, otherwise configuration is done via kubernetes with configmaps and secrets. 

On subsequent starts it will read from the configmaps for a wpa_supplicant.conf combined with the nodes hostname, this way each host can get it's own updates to wifi settings.

The container has iw-tools and wpa_supplicant installed. This allows full configuration of enterprise security wifi if there are wifi interfaces found on the system.
The container must run with elevated priviledges and on the host network of each node.

Each node will have a configMap and three secrets available for wifi configuration

Config map contains the wpa_supplicant.conf file as well as the name of the wifi interface that was autodetected. Currently only one wifi interface is supported at a time.

Config map name:
```
$HOSTNAME-wifi-config
```

Secrets for enterprise wifi: 
```
$HOSTNAME-wifi-client-key
$HOSTNAME-wifi-client-cert
$HOSTNAME-wifi-ca-cert
```

All the secrets and config will be stored in the kube-system namespace.

Entrypoint.sh was the original idea, I've moved it to a go program that should do the exact same stuff.

## Containers
I modified the talos linux kernel following their documentation to generate a kernel image after enabling the networking/wireless options, adding the config80211 extensions to the kernel, and enabling every wifi driver as a module with any available options (eg: promiscuous mode) enabled.

ghcr.io/centerionware/talos-wifi-daemonset:kernel-v1.10.0-16-g39b9c9f-dirty

And a compiled image of this repo for the daemonset

ghcr.io/centerionware/talos-wifi-daemonset:latest

## How to use
 *NOTE* This whole repo is mostly untested as of time of writing.

Use the provided kernel image or a self built kernel image that enables wifi


Create an iso that includes the kernel

### Extremely untested, probably wont work
Adding the kernel flags `--wifi-ssid=YOUR_SSID` and `--wifi-password=YOUR_WIFI_PASSWORD` - There are no options to configure enterprise security at bootoup. 

### This is probably going to have a better shot at working
To configure wifi, first bootstrap the node with ethernet and join it to a cluster, add the daemonset, then modify the kubernetes configMap in the namespace the daemonset lives in (kube-system by default) for the node (It will be named $HOSTNAME-wifi-config), and drop in a wpa_supplicant.conf. The certificates from the kubernetes secrets ($HOSTNAME-wifi-certname) will be placed into the container at /etc/cert/

```s
CERT_PATH="/etc/cert"

# Define the paths for certificates
CA_CERT="$CERT_PATH/ca.pem"
CLIENT_CERT="$CERT_PATH/user.pem"
PRIVATE_KEY="$CERT_PATH/user.prv"
```


## How to use with Omni

idk

## Missing features

* Multiple wifi connections from a single node
* Disabling - If you need this modify the daemonset and use labels so it won't run on nodes with a specific label such as NO-WIFI or DISABLE-WIFI or whatever.


