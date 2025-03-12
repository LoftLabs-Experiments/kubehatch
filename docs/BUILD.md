# 🛠️ Building KubeHatch Locally

This guide will walk you through setting up and building the KubeHatch CLI from source.

---

## 📌 Prerequisites

Before you start, ensure you have the following installed:

- **Go** (1.22 or later) → [Download Go](https://go.dev/dl/)
- **Git** (for cloning the repository) → [Install Git](https://git-scm.com/)
- **A Kubernetes cluster** (for testing, optional)
- **Kubectl & Helm** (optional, for cluster interaction)

---

## 🚀 Step 1: Clone the Repository

First, clone the repository from GitHub:

```sh
git clone https://github.com/LoftLabs-Experiments/kubehatch.git
cd kubehatch
```
## ⚙️ Step 2: Install Dependencies
Navigate to the backend/ directory and fetch the required Go modules:
```
cd backend
go mod github.com/YOUR-USERNAME/kubehatch
go mod tidy
```

🏗️ Step 3: Build the KubeHatch CLI
Run the following command inside the backend/ directory to compile the binary:
```
go build -o kubehatch main.go

```
This will generate an executable named kubehatch inside the backend/ directory.

## Step 4: Verify Installation
Check if the CLI is built correctly by running:
```
./kubehatch
```
You should see the available commands and usage.

## 📝 Additional Notes
If you make changes to dependencies, always run go mod tidy before building.

To cross-compile for different operating systems, use:
```
GOOS=linux GOARCH=amd64 go build -o kubehatch-linux main.go
```
🎉 Congratulations! You have successfully built the KubeHatch CLI from source. 🚀

For further details, visit the **[Quickstart Guide](QUICKSTART.md)**.