<!--
    Licensed to the Apache Software Foundation (ASF) under one or more
    contributor license agreements.  See the NOTICE file distributed with
    this work for additional information regarding copyright ownership.
    The ASF licenses this file to You under the Apache License, Version 2.0
    (the "License"); you may not use this file except in compliance with
    the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
    
    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.
-->

# seata-k8s

[中文文档](README.zh.md) | [English](README.md)

## Overview

seata-k8s is a Kubernetes operator for deploying and managing [Apache Seata](https://github.com/seata/seata) distributed transaction servers. It provides a streamlined way to deploy Seata Server clusters on Kubernetes with automatic scaling, persistence management, and operational simplicity.

## Features

- 🚀 **Easy Deployment**: Deploy Seata Server clusters using Kubernetes CRDs
- 📈 **Auto Scaling**: Simple scaling through replica configuration
- 💾 **Persistence Management**: Built-in support for persistent volumes
- 🔐 **RBAC Support**: Comprehensive role-based access control
- 🛠️ **Developer Friendly**: Includes debugging and development tools

## Related Projects

- [Apache Seata](https://github.com/seata/seata) - Distributed transaction framework
- [Seata Samples](https://github.com/seata/seata-samples/tree/docker/springboot-dubbo-fescar) - Example implementations
- [Seata Docker](https://github.com/seata/seata-docker) - Docker image repository

## Table of Contents

- [Method 1: Using Operator](#method-1-using-operator)
  - [Usage](#usage)
  - [CRD Reference](#crd-reference)
  - [Development Guide](#development-guide)
- [Method 2: Direct Kubernetes Deployment](#method-2-direct-kubernetes-deployment)
  - [Deployment Steps](#deployment-steps)
  - [Testing](#testing)

## Method 1: Using Operator

### Prerequisites

- Kubernetes 1.16+ cluster
- kubectl configured with access to your cluster
- Make and Docker (for building images)

### Usage

To deploy Seata Server using the Operator method, follow these steps:

#### Step 1: Clone the Repository

```shell
git clone https://github.com/apache/incubator-seata-k8s.git
cd incubator-seata-k8s
```

#### Step 2: Deploy Operator to Cluster

Deploy the controller, CRD, RBAC, and other required resources:

```shell
make deploy
```

Verify the deployment:

```shell
kubectl get deployment -n seata-k8s-controller-manager
kubectl get pods -n seata-k8s-controller-manager
```

#### Step 3: Deploy Seata Server Cluster

Create a SeataServer resource. Here's an example based on [seata-server-cluster.yaml](deploy/seata-server-cluster.yaml):

```yaml
apiVersion: operator.seata.apache.org/v1alpha1
kind: SeataServer
metadata:
  name: seata-server
  namespace: default
spec:
  serviceName: seata-server-cluster
  replicas: 3
  image: apache/seata-server:latest
  persistence:
    volumeReclaimPolicy: Retain
    spec:
      resources:
        requests:
          storage: 5Gi
```

Apply it to your cluster:

```shell
kubectl apply -f seata-server.yaml
```

If everything is working correctly, the operator will:
- Create 3 StatefulSet replicas
- Create a Headless Service named `seata-server-cluster`
- Set up persistent volumes

Access the Seata Server cluster within your Kubernetes network:

```
seata-server-0.seata-server-cluster.default.svc
seata-server-1.seata-server-cluster.default.svc
seata-server-2.seata-server-cluster.default.svc
```

### CRD Reference

For complete CRD definitions, see [operator.seata.apache.org_seataservers.yaml](config/crd/bases/operator.seata.apache.org_seataservers.yaml).

#### Key Configuration Properties

| Property | Description | Default | Example |
|----------|-------------|---------|---------|
| `serviceName` | Name of the Headless Service | - | `seata-server-cluster` |
| `replicas` | Number of Seata Server replicas | 1 | `3` |
| `image` | Seata Server container image | - | `apache/seata-server:latest` |
| `ports.consolePort` | Console port | `7091` | `7091` |
| `ports.servicePort` | Service port | `8091` | `8091` |
| `ports.raftPort` | Raft consensus port | `9091` | `9091` |
| `resources` | Container resource requests/limits | - | See example below |
| `persistence.volumeReclaimPolicy` | Volume reclaim policy | `Retain` | `Retain` or `Delete` |
| `persistence.spec.resources.requests.storage` | Persistent volume size | - | `5Gi` |
| `env` | Environment variables | - | See example below |

#### Environment Variables & Secrets

Configure Seata Server settings using environment variables and Kubernetes Secrets:

```yaml
apiVersion: operator.seata.apache.org/v1alpha1
kind: SeataServer
metadata:
  name: seata-server
  namespace: default
spec:
  image: apache/seata-server:latest
  replicas: 1
  persistence:
    spec:
      resources:
        requests:
          storage: 5Gi
  env:
  - name: console.user.username
    value: seata
  - name: console.user.password
    valueFrom:
      secretKeyRef:
        name: seata-credentials
        key: password
---
apiVersion: v1
kind: Secret
metadata:
  name: seata-credentials
  namespace: default
type: Opaque
stringData:
  password: your-secure-password
```



### Development Guide

To debug and develop this operator locally, we recommend using Minikube or a similar local Kubernetes environment.

#### Option 1: Build and Deploy Docker Image

Modify the code and rebuild the controller image:

```shell
# Start minikube and set docker environment
minikube start
eval $(minikube docker-env)

# Build and deploy
make docker-build deploy

# Verify deployment
kubectl get deployment -n seata-k8s-controller-manager
```

#### Option 2: Local Debug with Telepresence

Use [Telepresence](https://www.telepresence.io/) to debug locally without building container images.

**Prerequisites:**
- Install [Telepresence CLI](https://www.telepresence.io/docs/latest/quick-start/)
- Install [Traffic Manager](https://www.getambassador.io/docs/telepresence/latest/install/manager#install-the-traffic-manager)

**Steps:**

1. Connect Telepresence to your cluster:

```shell
telepresence connect
telepresence status  # Verify connection
```

2. Generate code resources:

```shell
make manifests generate fmt vet
```

3. Run the controller locally using your IDE or command line:

```shell
go run .
```

Now your local development environment has access to the Kubernetes cluster's DNS and services.

   


## Method 2: Direct Kubernetes Deployment

This method deploys Seata Server directly using Kubernetes manifests without the operator. Note that Seata Docker images currently require link-mode for container communication.

### Prerequisites

- MySQL database
- Nacos registry server
- Access to Kubernetes cluster

### Deployment Steps

#### Step 1: Deploy Seata and Dependencies

Deploy Seata server, Nacos, and MySQL:

```shell
kubectl apply -f deploy/seata-deploy.yaml
kubectl apply -f deploy/seata-service.yaml
```

#### Step 2: Retrieve Service Information

```shell
kubectl get service
# Note the NodePort IPs and ports for Seata and Nacos
```

#### Step 3: Configure DNS Addressing

Update `example/example-deploy.yaml` with the NodePort IP addresses obtained above.

#### Step 4: Initialize Database

```shell
# Connect to MySQL and import Seata table schema
# Replace CLUSTER_IP with your MySQL service IP
mysql -h <CLUSTER_IP> -u root -p < path/to/seata-db-schema.sql
```

#### Step 5: Deploy Example Applications

Deploy the sample microservices:

```shell
# Deploy account and storage services
kubectl apply -f example/example-deploy.yaml
kubectl apply -f example/example-service.yaml

# Deploy order service
kubectl apply -f example/order-deploy.yaml
kubectl apply -f example/order-service.yaml

# Deploy business service
kubectl apply -f example/business-deploy.yaml
kubectl apply -f example/business-service.yaml
```

### Verification

Open Nacos console to verify service registration:

```
http://localhost:8848/nacos/
```

Check that all services are registered:
- account-service
- storage-service
- order-service
- business-service

### Testing

Test the distributed transaction scenarios using the following curl commands:

#### Test 1: Account Service - Deduct Amount

```shell
curl -H "Content-Type: application/json" \
  -X POST \
  --data '{"id":1,"userId":"1","amount":100}' \
  http://<CLUSTER_IP>:8102/account/dec_account
```

#### Test 2: Storage Service - Deduct Stock

```shell
curl -H "Content-Type: application/json" \
  -X POST \
  --data '{"commodityCode":"C201901140001","count":100}' \
  http://<CLUSTER_IP>:8100/storage/dec_storage
```

#### Test 3: Order Service - Create Order

```shell
curl -H "Content-Type: application/json" \
  -X POST \
  --data '{"userId":"1","commodityCode":"C201901140001","orderCount":10,"orderAmount":100}' \
  http://<CLUSTER_IP>:8101/order/create_order
```

#### Test 4: Business Service - Execute Transaction

```shell
curl -H "Content-Type: application/json" \
  -X POST \
  --data '{"userId":"1","commodityCode":"C201901140001","count":10,"amount":100}' \
  http://<CLUSTER_IP>:8104/business/dubbo/buy
```

Replace `<CLUSTER_IP>` with the actual NodePort IP address of your service.



