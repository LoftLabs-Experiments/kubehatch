package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// VclusterConfig describes the structure of the vcluster.yaml file.
type VclusterConfig struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Replicas int `yaml:"replicas"`
		Service  struct {
			Type string `yaml:"type,omitempty"`
		} `yaml:"service,omitempty"`
	} `yaml:"spec"`
}

// ServiceJSON is used to parse Kubernetes service JSON output.
type ServiceJSON struct {
	Status struct {
		LoadBalancer struct {
			Ingress []struct {
				IP       string `json:"ip"`
				Hostname string `json:"hostname"`
			} `json:"ingress"`
		} `json:"loadBalancer"`
	} `json:"status"`
	Spec struct {
		Ports []struct {
			Port int `json:"port"`
		} `json:"ports"`
	} `json:"spec"`
}

// VclusterResponse is the JSON response that includes the generated kubeconfig.
type VclusterResponse struct {
	Kubeconfig string `json:"kubeconfig"`
}

func main() {
	http.HandleFunc("/api/vcluster", corsMiddleware(vclusterHandler))
	http.HandleFunc("/download", corsMiddleware(downloadHandler))
	log.Println("Backend API running on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func filterEnv(env []string, keys []string) []string {
	filtered := []string{}
	for _, e := range env {
		skip := false
		for _, key := range keys {
			if strings.HasPrefix(e, key+"=") {
				skip = true
				break
			}
		}
		if !skip {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func vclusterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Error parsing multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}
	clusterName := r.FormValue("clusterName")
	ha := r.FormValue("ha") == "on"
	useLoadBalancer := r.FormValue("loadbalancer") == "on" // Changed to "loadbalancer" to match frontend
	if clusterName == "" {
		http.Error(w, "clusterName is required", http.StatusBadRequest)
		return
	}

	reqID := strconv.FormatInt(time.Now().UnixNano(), 10)
	workingDir := filepath.Join(".", "requests", reqID)
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		http.Error(w, "Error creating working directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var hostKubeconfig string
	file, _, err := r.FormFile("kubeconfigFile")
	if err == nil && file != nil {
		defer file.Close()
		uploadPath := filepath.Join(workingDir, "uploaded.yaml")
		outFile, err := os.Create(uploadPath)
		if err != nil {
			http.Error(w, "Error creating file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer outFile.Close()
		if _, err = io.Copy(outFile, file); err != nil {
			http.Error(w, "Error saving uploaded file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hostKubeconfig, err = filepath.Abs(uploadPath)
		if err != nil {
			http.Error(w, "Error determining absolute path: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		hostKubeconfig = "/var/secrets/kubeconfig"
		if _, err := os.Stat(hostKubeconfig); os.IsNotExist(err) {
			http.Error(w, "No kubeconfig uploaded and default /var/secrets/kubeconfig not found", http.StatusBadRequest)
			return
		}
	}

	// Enhanced logging to debug form values
	log.Printf("Request %s: Received clusterName=%s, HA=%v, LoadBalancer=%v, kubeconfig=%s", reqID, clusterName, ha, useLoadBalancer, hostKubeconfig)
	log.Printf("DEBUG: Raw form values - clusterName: %s, ha: %s, loadbalancer: %s", r.FormValue("clusterName"), r.FormValue("ha"), r.FormValue("loadbalancer"))

	if err := createVclusterYAML(workingDir, clusterName, ha, useLoadBalancer); err != nil {
		http.Error(w, fmt.Sprintf("Error creating YAML: %v", err), http.StatusInternalServerError)
		return
	}

	if err := createVirtualCluster(workingDir, clusterName, hostKubeconfig, useLoadBalancer); err != nil {
		http.Error(w, fmt.Sprintf("Error creating virtual cluster: %v", err), http.StatusInternalServerError)
		return
	}
	log.Printf("Request %s: Waiting for 1 minute for the cluster to be ready...", reqID)
	time.Sleep(1 * time.Minute)

	if err := fetchAndPatchKubeconfigFromSecret(workingDir, clusterName, hostKubeconfig, useLoadBalancer); err != nil {
		http.Error(w, fmt.Sprintf("Error fetching kubeconfig from secret: %v", err), http.StatusInternalServerError)
		return
	}

	kcPath := filepath.Join(workingDir, ".vcluster", clusterName, "kubeconfig.yaml")
	kcData, err := os.ReadFile(kcPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading kubeconfig: %v", err), http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:  "reqid",
		Value: reqID,
		Path:  "/",
	})
	resp := VclusterResponse{
		Kubeconfig: string(kcData),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("reqid")
	if err != nil {
		http.Error(w, "Request ID not set", http.StatusBadRequest)
		return
	}
	clusterName := r.URL.Query().Get("clusterName")
	if clusterName == "" {
		http.Error(w, "clusterName query parameter required", http.StatusBadRequest)
		return
	}
	kcPath := filepath.Join(".", "requests", cookie.Value, ".vcluster", clusterName, "kubeconfig.yaml")
	w.Header().Set("Content-Disposition", "attachment; filename=kubeconfig.yaml")
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, kcPath)
}

func createVclusterYAML(workingDir, clusterName string, ha, useLoadBalancer bool) error {
	cfg := VclusterConfig{
		APIVersion: "v1",
		Kind:       "VirtualCluster",
	}
	cfg.Metadata.Name = clusterName
	if ha {
		cfg.Spec.Replicas = 3
	} else {
		cfg.Spec.Replicas = 1
	}
	if useLoadBalancer {
		cfg.Spec.Service.Type = "LoadBalancer"
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("error marshalling YAML: %v", err)
	}
	yamlPath := filepath.Join(workingDir, "vcluster.yaml")
	if err := os.WriteFile(yamlPath, data, 0644); err != nil {
		return fmt.Errorf("error writing vcluster.yaml: %v", err)
	}
	log.Println("Generated vcluster.yaml:")
	log.Println(string(data))
	return nil
}

func getFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func createVirtualCluster(workingDir, clusterName, hostKubeconfig string, useLoadBalancer bool) error {
	args := []string{
		"create", clusterName,
		"--config", "vcluster.yaml",
		"--connect=false",
		"--skip-wait",
		"--debug",
	}
	if useLoadBalancer {
		args = append(args, "--expose")
	}
	cmd := exec.Command("vcluster", args...)
	cmd.Dir = workingDir
	env := filterEnv(os.Environ(), []string{"KUBERNETES_SERVICE_HOST", "KUBERNETES_SERVICE_PORT", "KUBERNETES_PORT"})
	env = append(env, "KUBECONFIG="+hostKubeconfig)
	cmd.Env = env
	log.Println("DEBUG: executing command:", cmd.Args, "in", workingDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("DEBUG: vcluster create command output:\n%s", string(out))
		return fmt.Errorf("vcluster create failed: %v\nOutput:\n%s", err, string(out))
	}
	log.Println("DEBUG: vcluster create command finished, output:")
	log.Println(string(out))
	return nil
}

func fetchAndPatchKubeconfigFromSecret(workingDir, clusterName, hostKubeconfig string, useLoadBalancer bool) error {
	namespace := "vcluster-" + clusterName
	secretName := "vc-" + clusterName
	var kcData []byte

	retryTimeout := time.After(3 * time.Minute)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		cmd := exec.Command("kubectl", "--kubeconfig", hostKubeconfig, "get", "secret", secretName, "-n", namespace, "--template={{.data.config}}")
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("DEBUG: failed to get secret %s in namespace %s: %v, output: %s", secretName, namespace, err, string(out))
		} else {
			decoded, err := base64.StdEncoding.DecodeString(string(out))
			if err != nil {
				log.Printf("DEBUG: failed to decode base64 kubeconfig: %v", err)
			} else if len(decoded) > 0 {
				kcData = decoded
				log.Println("DEBUG: successfully retrieved kubeconfig from secret")
				break
			}
		}
		select {
		case <-retryTimeout:
			return fmt.Errorf("timed out waiting for vcluster secret %s in namespace %s", secretName, namespace)
		case <-ticker.C:
			log.Println("DEBUG: secret not ready yet, retrying...")
		}
	}

	if useLoadBalancer {
		log.Println("DEBUG: polling for external endpoint of virtual cluster...")
		externalEndpoint, err := pollForExternalEndpoint(hostKubeconfig, clusterName)
		if err != nil {
			return fmt.Errorf("failed to get external endpoint: %v", err)
		}
		kcData, err = updateKubeconfigEndpoint(kcData, externalEndpoint)
		if err != nil {
			return fmt.Errorf("failed to update kubeconfig: %v", err)
		}
	}

	newDir := filepath.Join(workingDir, ".vcluster", clusterName)
	newPath := filepath.Join(newDir, "kubeconfig.yaml")
	if err := os.MkdirAll(newDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(newPath, kcData, 0644); err != nil {
		return fmt.Errorf("failed to write kubeconfig file: %v", err)
	}
	log.Println("DEBUG: kubeconfig written to", newPath)
	return nil
}

func pollForExternalEndpoint(hostKubeconfig, clusterName string) (string, error) {
	ns := "vcluster-" + clusterName
	svcName := clusterName
	timeout := time.After(3 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-timeout:
			return "", fmt.Errorf("timed out waiting for external endpoint")
		case <-ticker.C:
			cmd := exec.Command("kubectl", "--kubeconfig", hostKubeconfig, "get", "svc", svcName, "-n", ns, "-o", "json")
			out, err := cmd.CombinedOutput()
			if err != nil {
				log.Println("DEBUG: kubectl get svc error:", err, "output:", string(out))
				continue
			}
			var svc ServiceJSON
			if err := json.Unmarshal(out, &svc); err != nil {
				log.Println("DEBUG: error unmarshaling service JSON:", err)
				continue
			}
			if len(svc.Status.LoadBalancer.Ingress) > 0 {
				ing := svc.Status.LoadBalancer.Ingress[0]
				var external string
				if ing.IP != "" {
					external = ing.IP
				} else if ing.Hostname != "" {
					external = ing.Hostname
				} else {
					continue
				}
				if len(svc.Spec.Ports) == 0 {
					continue
				}
				port := svc.Spec.Ports[0].Port
				var endpoint string
				if port == 443 {
					endpoint = "https://" + external
				} else {
					endpoint = "https://" + external + ":" + strconv.Itoa(port)
				}
				log.Println("DEBUG: found external endpoint:", endpoint)
				return endpoint, nil
			}
			log.Println("DEBUG: external endpoint not available yet; polling...")
		}
	}
}

func updateKubeconfigEndpoint(kcData []byte, newEndpoint string) ([]byte, error) {
	var config map[string]interface{}
	if err := yaml.Unmarshal(kcData, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kubeconfig: %v", err)
	}
	clusters, ok := config["clusters"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("kubeconfig missing 'clusters' field")
	}
	for _, c := range clusters {
		clusterEntry, ok := c.(map[interface{}]interface{})
		if !ok {
			continue
		}
		clusterData, ok := clusterEntry["cluster"].(map[interface{}]interface{})
		if !ok {
			continue
		}
		clusterData["server"] = newEndpoint
	}
	updated, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated kubeconfig: %v", err)
	}
	return updated, nil
}
