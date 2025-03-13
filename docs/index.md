# KubeHatch - Minimalistic Internal Kubernetes platform

KubeHatch simplifies creating virtual Kubernetes clusters (vClusters) dynamically using a user-friendly web UI, automating deployment and management tasks.

## Overview

This CLI helps you easily create and manage isolated ephemeral Kubernetes clusters (vClusters) for quick and efficient testing, validation, and automation scenarios. You can provide your own kubeconfig or rely on a default kubeconfig of the cluster on which the CLI is running.

## Features

- Create isolated Kubernetes clusters (vClusters).
- Optional High Availability (HA) setup.
- Uses user-provided or default kubeconfig.
- Automated exposure of vClusters via LoadBalancer services.
- Complete self-hosted web UI.

## Prerequisites

- Kubernetes Cluster
- Helm CLI
- vCluster CLI

## Step-by-Step Setup

### 1. Clone the Repository
```
git clone https://github.com/LoftLabs-Experiments/kubehatch.git
cd kubehatch
```

### Step 2: Build and Push Docker Image

Ensure you're logged in to your container registry (e.g., Docker Hub).
```
docker build -t ttl.sh/kubehatch-backend -f backend/Dockerfile.backend backend/
docker push ttl.sh/kubehatch-backend
docker build -t ttl.sh/kubehatch-frontend -f frontend/Dockerfile.frontend frontend/
docker push ttl.sh/kubehatch-frontend:latest
```
Replace `ttl.sh` with your registry and image name.

### Step 3: Deploy to Kubernetes

Backend Deployment

Apply Kubernetes manifests:
```
kubectl apply -f backenddeploy.yaml
kubectl apply -f backendsvc.yaml
```
## Step 3: Configure Frontend and Ingress

Edit your ingress.yaml to point to the correct backend service. Then apply it:
```
kubectl apply -f ingress.yaml
```
Ensure your domain points to your ingress controller IP.

## Step 4: Using the UI

Open your UI hosted via ingress in your browser:

```http://<your-ingress-domain>```

## Using KubeHatch

Create vCluster with custom kubeconfig

- Open the UI.
- Enter a vCluster name.
- Optional: Select kubeconfig file (if not provided, the default cluster kubeconfig is used).
- Click Create.
- Wait for vCluster creation to complete. Your generated kubeconfig will appear, and you can download it for use.

#### Create vCluster without custom kubeconfig

Simply provide a name and click Create. The system will automatically use the default kubeconfig from the cluster.

## Cleanup

Delete virtual clusters manually using:

```
vcluster delete <cluster-name> -n vcluster-<cluster-name>
```

##Troubleshooting

Verify RBAC permissions are correctly set.

Check logs:

```
kubectl logs deploy/vcluster-backend
```


Enjoy creating ephemeral Kubernetes environments seamlessly with KubeHatch!
