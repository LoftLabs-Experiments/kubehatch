package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// Configuration
const (
	defaultNamespace      = "default"
	kubeconfigSecretPath  = "/var/secrets/kubeconfig"
	maxRetries            = 5
	retryInterval         = 10 * time.Second
)

// Response structure for frontend
type VclusterResponse struct {
	Kubeconfig string `json:"kubeconfig"`
}

func main() {
	http.HandleFunc("/api/vcluster", corsMiddleware(vclusterHandler))
	http.HandleFunc("/download", corsMiddleware(downloadHandler))
	log.Println("✅ Backend API running on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

// Middleware to handle CORS
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

// Handles vCluster creation requests
func vclusterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Error parsing form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get user input
	clusterName := r.FormValue("clusterName")
	ha := r.FormValue("ha") == "on"
	loadBalancer := r.FormValue("loadbalancer") == "on"

	if clusterName == "" {
		http.Error(w, "clusterName is required", http.StatusBadRequest)
		return
	}

	// Create a unique working directory for this request
	reqID := strconv.FormatInt(time.Now().UnixNano(), 10)
	workingDir := filepath.Join(".", "requests", reqID)
	os.MkdirAll(workingDir, 0755)

	// Determine kubeconfig source
	kubeconfigPath := filepath.Join(workingDir, "uploaded.yaml")
	file, _, err := r.FormFile("kubeconfigFile")
	if err == nil {
		// User uploaded kubeconfig
		defer file.Close()
		outFile, err := os.Create(kubeconfigPath)
		if err == nil {
			io.Copy(outFile, file)
			outFile.Close()
		}
		log.Printf("✅ Using uploaded kubeconfig: %s", kubeconfigPath)
	} else {
		// Use default kubeconfig from Kubernetes secret
		kubeconfigPath = kubeconfigSecretPath
		log.Printf("⚠️ No kubeconfig uploaded. Using default: %s", kubeconfigPath)
	}

	// Generate vcluster.yaml
	err = createVclusterYAML(workingDir, clusterName, ha, loadBalancer)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating YAML: %v", err), http.StatusInternalServerError)
		return
	}

	// Create vCluster (only once)
	err = executeVClusterCreate(clusterName, kubeconfigPath, workingDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating vCluster: %v", err), http.StatusInternalServerError)
		return
	}

	// Wait for 1 minute instead of 2
	log.Printf("⏳ Waiting for 1 minute for vCluster %s to be ready...", clusterName)
	time.Sleep(1 * time.Minute)

	// Connect & patch kubeconfig
	err = connectAndRetrieveKubeconfig(workingDir, clusterName, kubeconfigPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error connecting to vCluster: %v", err), http.StatusInternalServerError)
		return
	}

	// Retrieve kubeconfig
	finalKubeconfigPath := filepath.Join(workingDir, ".vcluster", clusterName, "kubeconfig.yaml")
	kubeconfigData, err := os.ReadFile(finalKubeconfigPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading generated kubeconfig: %v", err), http.StatusInternalServerError)
		return
	}
	log.Printf("✅ Successfully retrieved generated kubeconfig: %s", finalKubeconfigPath)

	// Send kubeconfig to frontend
	resp := VclusterResponse{
		Kubeconfig: string(kubeconfigData),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Generates vcluster.yaml
func createVclusterYAML(workingDir, clusterName string, ha, loadBalancer bool) error {
	replicas := 1
	if ha {
		replicas = 3
	}

	// Base YAML structure
	vclusterYaml := fmt.Sprintf(`apiVersion: v1
kind: VirtualCluster
metadata:
  name: %s
spec:
  replicas: %d
`, clusterName, replicas)

	// Append LoadBalancer only if checked
	if loadBalancer {
		vclusterYaml += `  service:
    type: LoadBalancer
`
	}

	// Write YAML file
	err := os.WriteFile(filepath.Join(workingDir, "vcluster.yaml"), []byte(vclusterYaml), 0644)
	if err != nil {
		log.Printf("❌ Error writing vcluster.yaml: %v", err)
		return err
	}
	log.Println("✅ Generated vcluster.yaml successfully.")
	log.Println(vclusterYaml) // 🔹 Ensure vcluster.yaml is printed
	return nil
}

// Executes vCluster creation
func executeVClusterCreate(clusterName, kubeconfigPath, workingDir string) error {
	cmd := exec.Command("vcluster", "create", clusterName, "--config", "vcluster.yaml", "--connect=false", "--skip-wait", "--expose")
	cmd.Dir = workingDir
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)

	log.Println("🔹 Executing vcluster create command:", cmd.Args)
	output, err := cmd.CombinedOutput()
	log.Println("🔹 vcluster create output:\n", string(output))

	if err != nil {
		log.Printf("❌ Failed to create vCluster %s: %s", clusterName, err)
		return err
	}
	log.Printf("✅ Successfully created vCluster: %s", clusterName)
	return nil
}

// Connects to vCluster and retrieves kubeconfig
func connectAndRetrieveKubeconfig(workingDir, clusterName, kubeconfigPath string) error {
	port := "12345"
	namespace := "vcluster-" + clusterName
	cmd := exec.Command("vcluster", "connect", clusterName, "-n", namespace, "--print", "--local-port", port, "--debug")
	cmd.Dir = workingDir
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)

	log.Println("🔹 Executing vcluster connect command:", cmd.Args)
	output, err := cmd.CombinedOutput()
	log.Println("🔹 vcluster connect output:\n", string(output))

	if err != nil {
		log.Printf("❌ Failed to connect to vCluster %s: %s", clusterName, err)
		return err
	}

	// Save kubeconfig
	finalPath := filepath.Join(workingDir, ".vcluster", clusterName, "kubeconfig.yaml")
	os.MkdirAll(filepath.Dir(finalPath), 0755)
	err = os.WriteFile(finalPath, output, 0644)
	if err != nil {
		log.Printf("❌ Failed to save kubeconfig for %s: %s", clusterName, err)
		return err
	}
	log.Printf("✅ Kubeconfig saved: %s", finalPath)
	return nil
}

// Download kubeconfig
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	clusterName := r.URL.Query().Get("clusterName")
	if clusterName == "" {
		http.Error(w, "clusterName query parameter required", http.StatusBadRequest)
		return
	}

	reqID := r.URL.Query().Get("reqid")
	kcPath := filepath.Join(".", "requests", reqID, ".vcluster", clusterName, "kubeconfig.yaml")
	w.Header().Set("Content-Disposition", "attachment; filename=kubeconfig.yaml")
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, kcPath)
}
