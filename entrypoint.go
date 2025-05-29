package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

const (
	configMapTemplate = "# See: https://linux.die.net/man/5/wpa_supplicant.conf"
	certPath          = "/etc/cert"
)

var hostname string

func main() {
	// Set the global hostname variable from the environment
	hostname = os.Getenv("HOSTNAME")
	configMapName := fmt.Sprintf("%s-wifi-config", hostname)
	wpaConfPath := "/etc/wpa_supplicant/wpa_supplicant.conf"

	// Detect WiFi interface
	fmt.Println("Detecting WiFi interface...")
	wifiIface := getWifiInterface()
	if wifiIface == "" {
		fmt.Println("No WiFi interface found. Defaulting to wlan0.")
		wifiIface = "wlan0"
	}
	fmt.Printf("Using WiFi interface: %s\n", wifiIface)

	// Check for the ConfigMap
	fmt.Printf("Looking for configmap: %s\n", configMapName)
	if !kubectlExists(configMapName) {
		fmt.Println("No existing configmap found. Checking kernel parameters...")
		ssid, password := getKernelParams()

		if ssid != "" && password != "" {
			fmt.Printf("Creating configmap from kernel parameters (SSID: %s)\n", ssid)

			// Create configmap from kernel parameters
			err := createConfigMap(configMapName, ssid, password, wifiIface)
			if err != nil {
				fmt.Println("Error creating configmap:", err)
				return
			}
		} else {
			fmt.Println("No SSID or PASSWORD found in kernel parameters, creating blank configmap from template")

			// Create a blank configmap from template
			err := createBlankConfigMap(configMapName, wifiIface)
			if err != nil {
				fmt.Println("Error creating blank configmap:", err)
				return
			}
		}
	} else {
		fmt.Printf("Configmap %s already exists\n", configMapName)
	}

	// Check and create secrets for certificates if necessary
	checkAndCreateSecret(fmt.Sprintf("%s-wifi-ca-cert", hostname), certPath+"/ca.pem")
	checkAndCreateSecret(fmt.Sprintf("%s-wifi-client-cert", hostname), certPath+"/user.pem")
	checkAndCreateSecret(fmt.Sprintf("%s-wifi-client-key", hostname), certPath+"/user.prv")

	// Fetch configmap and write to WPA conf path
	err := fetchConfigMap(configMapName, wpaConfPath)
	if err != nil {
		fmt.Println("Error fetching configmap:", err)
		return
	}

	// Read wifi interface from configmap
	wifiIfaceFromConfigMap, err := getConfigMapField(configMapName, "wifi_interface")
	if err != nil || wifiIfaceFromConfigMap == "" {
		fmt.Println("Error: wifi_interface not found in configmap")
		return
	}
	fmt.Printf("Using WiFi interface from configmap: %s\n", wifiIfaceFromConfigMap)

	// Mount the secrets into the appropriate files
	err = mountSecrets()
	if err != nil {
		fmt.Println("Error mounting secrets:", err)
		return
	}

	// Start wpa_supplicant
	fmt.Printf("Starting wpa_supplicant with interface: %s\n", wifiIface)
	startWpaSupplicant(wifiIface, wpaConfPath)
	// Errors are handled inside of startWpaSupplicant. 
}

func getWifiInterface() string {
	cmd := exec.Command("iwconfig")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error running iwconfig:", err)
		return ""
	}
	// Parse the output to find WiFi interfaces
	lines := strings.Split(out.String(), "\n")
	for _, line := range lines {
		if strings.Contains(line, "IEEE 802.11") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				return parts[0]
			}
		}
	}
	return ""
}

func getKernelParams() (ssid, password string) {
	cmd := exec.Command("cat", "/proc/cmdline")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error reading kernel params:", err)
		return "", ""
	}

	// Parse the kernel command line for wifi parameters
	params := strings.Fields(out.String())
	for _, param := range params {
		if strings.HasPrefix(param, "--wifi-ssid=") {
			ssid = strings.Split(param, "=")[1]
		} else if strings.HasPrefix(param, "--wifi-password=") {
			password = strings.Split(param, "=")[1]
		}
	}
	return ssid, password
}

func kubectlExists(name string) bool {
	cmd := exec.Command("kubectl", "get", "configmap", name)
	err := cmd.Run()
	return err == nil
}

func createConfigMap(name, ssid, password, iface string) error {
	cmd := exec.Command("kubectl", "create", "configmap", name,
		"--from-literal=wpa_supplicant.conf=ctrl_interface=/var/run/wpa_supplicant\nnetwork={\n\tssid=\"" + ssid + "\"\n\tpsk=\"" + password + "\"\n}\n",
		"--from-literal=wifi_interface="+iface)
	return cmd.Run()
}

func createBlankConfigMap(name, iface string) error {
	cmd := exec.Command("kubectl", "create", "configmap", name,
		"--from-literal=wpa_supplicant.conf="+configMapTemplate,
		"--from-literal=wifi_interface="+iface)
	return cmd.Run()
}

func fetchConfigMap(name, wpaConfPath string) error {
	cmd := exec.Command("kubectl", "get", "configmap", name, "-o", "jsonpath={.data.wpa_supplicant\\.conf}")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(wpaConfPath, out.Bytes(), 0644)
}

func getConfigMapField(name, field string) (string, error) {
	cmd := exec.Command("kubectl", "get", "configmap", name, "-o", fmt.Sprintf("jsonpath={.data.%s}", field))
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

func checkAndCreateSecret(secretName, filePath string) {
	cmd := exec.Command("kubectl", "get", "secret", secretName)
	err := cmd.Run()
	if err != nil {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			fmt.Printf("Error: Certificate file %s does not exist.\n", filePath)
			return
		}
		createSecret(secretName, filePath)
	}
}

func createSecret(secretName, filePath string) {
	cmd := exec.Command("kubectl", "create", "secret", "generic", secretName, "--from-literal="+filePath+"=\"\"")
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error creating secret:", err)
		fmt.Println("kubectl create secret generic "+secretName+" --from-literal="+filePath+"=\"\"")
	}
	fmt.Printf("Created Kubernetes Secret for %s\n", secretName)
}

func mountSecrets() error {
	// Ensure the cert directory exists
	err := os.MkdirAll(certPath, 0755)
	if err != nil {
		return err
	}

	// Mount the secrets into files
	err = mountSecretToFile(fmt.Sprintf("%s-wifi-ca-cert", hostname), certPath+"/ca.pem")
	if err != nil {
		return err
	}
	err = mountSecretToFile(fmt.Sprintf("%s-wifi-client-cert", hostname), certPath+"/user.pem")
	if err != nil {
		return err
	}
	err = mountSecretToFile(fmt.Sprintf("%s-wifi-client-key", hostname), certPath+"/user.prv")
	return err
}

func mountSecretToFile(secretName, filePath string) error {
	cmd := exec.Command("kubectl", "get", "secret", secretName, "-o", fmt.Sprintf("jsonpath={.data.%s}", secretName))
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return err
	}
	data := out.Bytes()
	return ioutil.WriteFile(filePath, data, 0644)
}

func startWpaSupplicant(wifiIface, wpaConfPath string) {
	// Prepare the command to start wpa_supplicant without -B (no background)
	cmd := exec.Command("/sbin/wpa_supplicant", "-i", wifiIface, "-c", wpaConfPath)

	// Set up pipes to capture stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error setting up stdout pipe:", err)
		return
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		fmt.Println("Error setting up stderr pipe:", err)
		return
	}

	// Start the wpa_supplicant process
	err = cmd.Start()
	if err != nil {
		fmt.Println("Error starting wpa_supplicant:", err)
		return
	}

	// Use goroutines to read stdout and stderr and print to console in real-time
	go func() {
		io.Copy(os.Stdout, stdoutPipe)
	}()
	go func() {
		io.Copy(os.Stderr, stderrPipe)
	}()

	// Wait for the wpa_supplicant process to finish
	err = cmd.Wait()
	if err != nil {
		fmt.Println("wpa_supplicant process exited with error:", err)
	} else {
		fmt.Println("wpa_supplicant process finished successfully.")
	}
}
