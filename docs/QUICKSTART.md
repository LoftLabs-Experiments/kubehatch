# 🚀 Quickstart Guide: Deploying KubeHatch on Kubernetes

This guide walks you through deploying KubeHatch on a Kubernetes cluster.

## 🛠 Prerequisites
Ensure you have the following tools installed before proceeding:
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/docs/intro/install/)
- Access to a Kubernetes cluster
- (Optional) [Kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/) if using overlays

---

## 🔐 Step 1: Create Kubernetes Secret for Authentication and default Kubeconfig
Before deploying, create a **Kubernetes Secret** to store sensitive credentials.

```sh
kubectl create secret generic vcluster-basic-auth  --from-literal=username=admin  --from-literal=password=password
kubectl create secret generic vcluster-default-kubeconfig --from-file=kubeconfig=/root/.kube/config -n default
```
NOTE - Make sure to replace the path of your kubeconfig file.

## Build the Frontend and backend Images

## 🚀 Step 2: Deploy the frontend and backend manifests
Replace the backedna dn frontend deployment with the images you created and then deploy the manifest from the k8s folder.
```

```
## Deploy ingress nginx controller 
```
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.9.4/deploy/static/provider/cloud/deploy.yaml

```

## 
🏗 Step 3: Deploy Using Helm (Alternative)
If you prefer Helm, add the Helm repository and install KubeHatch:

sh
Copy
Edit
helm repo add kubehatch https://loftlabs-experiments.github.io/kubehatch/
helm repo update
helm install kubehatch kubehatch/kubehatch -n kubehatch --create-namespace
Verify the deployment:

sh
Copy
Edit
kubectl get deployments -n kubehatch
🔍 Step 4: Access the KubeHatch Service
To access the service locally, use kubectl port-forward:

sh
Copy
Edit
kubectl port-forward svc/kubehatch-service 8080:80 -n kubehatch
Then, visit http://localhost:8080 in your browser.

🎯 Next Steps
Check logs: kubectl logs -f deployment/kubehatch -n kubehatch
Expose externally with an Ingress:
sh
Copy
Edit
kubectl apply -f ingress.yaml
Read the Build from Source Guide for local development.
✅ Done! 🎉 You have successfully deployed KubeHatch on Kubernetes.