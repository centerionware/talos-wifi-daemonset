# talos-wifi

To enable WiFi with Talos a kernel must be compiled with the wireless modules enabled. Linux should be able to automatically detect if any are used and setup the environment

The test_deployment file will be erased. This is meant to run as a daemonset.

The idea is it will use kernel parameters if found to generate an initial wpa_supplicant configuration, otherwise configuration is done via kubernetes with configmaps and secrets. 

On subsequent starts it will read from the configmaps for a wpa_supplicant.conf combined with the nodes hostname, this way each host can get it's own updates to wifi settings.

The container has iw-tools and wpa_supplicant installed. This allows full configuration of enterprise grade wifi if there are wifi interfaces found on the system.
The container must run with elevated priviledges and on the host network of each node.

Each node will have a configMap and three secrets available for wifi configuration

Config map contains the wpa_supplicant.conf file as well as the name of the wifi interface that was autodetected. Currently only one wifi interface is supported at a time.

Secrets for enterprise wifi: 
```
$HOSTNAME-wifi-client-key
$HOSTNAME-wifi-client-cert
$HOSTNAME-wifi-ca-cert
```

Entrypoint.sh was the original idea, I've moved it to a go program that does virtually the exact same stuff. 