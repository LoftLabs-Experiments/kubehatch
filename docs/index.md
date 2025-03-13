# KubeHatch - Minimalistic Internal Kubernetes platform
![KubeHatch Logo](images/kubehatch.jpg "KubeHatch Logo")

KubeHatch simplifies creating virtual Kubernetes clusters (vClusters) dynamically using a user-friendly web UI, automating deployment and management tasks.

## Overview

This CLI helps you easily create and manage isolated ephemeral Kubernetes clusters (vClusters) for quick and efficient testing, validation, and automation scenarios. You can provide your own kubeconfig or rely on a default kubeconfig of the cluster on which the CLI is running.

## Architecture
![](images/architecture.png)


## Features

- Create isolated Kubernetes clusters (vClusters).
- Optional High Availability (HA) setup.
- Uses user-provided or default kubeconfig.
- Automated exposure of vClusters via LoadBalancer services.
- Complete self-hosted web UI.


## Try out now
Go the the [Quickstart](QUICKSTART.md)




