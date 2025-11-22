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

## 项目概述

seata-k8s 是一个用于在 Kubernetes 上部署和管理 [Apache Seata](https://github.com/seata/seata) 分布式事务服务器的 Kubernetes Operator。它提供了一种简化的方式来在 Kubernetes 上部署 Seata Server 集群，并支持自动扩缩容、持久化存储管理和运维简化。

## 主要特性

- 🚀 **快速部署**：使用 Kubernetes CRD 快速部署 Seata Server 集群
- 📈 **自动扩缩容**：通过简单的副本配置实现集群扩缩容
- 💾 **持久化存储**：内置持久化卷支持
- 🔐 **RBAC 支持**：完整的基于角色的访问控制
- 🛠️ **开发友好**：包含调试和开发工具

## 关联项目

- [Apache Seata](https://github.com/seata/seata) - 分布式事务框架
- [Seata 示例](https://github.com/seata/seata-samples/tree/docker/springboot-dubbo-fescar) - 示例实现
- [Seata Docker](https://github.com/seata/seata-docker) - Docker 镜像仓库

## 目录

- [方式一：使用 Operator](#方式一使用-operator)
  - [使用指南](#使用指南)
  - [CRD 配置参考](#crd-配置参考)
  - [开发者指南](#开发者指南)
- [方式二：直接部署](#方式二直接部署)
  - [部署步骤](#部署步骤)
  - [测试验证](#测试验证)

---

## 方式一：使用 Operator

### 前置要求

- Kubernetes 1.16+ 集群
- kubectl 已配置可访问集群
- Make 和 Docker（用于构建镜像）

### 使用指南

#### 第一步：克隆仓库

```shell
git clone https://github.com/apache/incubator-seata-k8s.git
cd incubator-seata-k8s
```

#### 第二步：部署 Operator

将 Controller、CRD、RBAC 等资源部署到 Kubernetes 集群：

```shell
make deploy
```

验证 Operator 部署：

```shell
kubectl get deployment -n seata-k8s-controller-manager
kubectl get pods -n seata-k8s-controller-manager
```

#### 第三步：部署 Seata Server 集群

创建 SeataServer 资源。以下是基于 [seata-server-cluster.yaml](deploy/seata-server-cluster.yaml) 的示例：

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

将其应用到集群：

```shell
kubectl apply -f seata-server.yaml
```

如果一切正常，Operator 将会：
- 创建 3 个 StatefulSet 副本
- 创建一个名为 `seata-server-cluster` 的 Headless Service
- 设置持久化存储卷

在 Kubernetes 集群内访问 Seata Server 集群：

```
seata-server-0.seata-server-cluster.default.svc
seata-server-1.seata-server-cluster.default.svc
seata-server-2.seata-server-cluster.default.svc
```

查看 Pod 状态：

```shell
kubectl get pods -l app=seata-server
kubectl logs -f seata-server-0
```

### CRD 配置参考

详见 [seataservers_crd.yaml](config/crd/bases/v1/seataservers_crd.yaml)。

#### 关键配置字段

| 字段 | 描述 | 默认值 | 示例 |
|------|------|--------|------|
| `serviceName` | Headless Service 名称 | - | `seata-server-cluster` |
| `replicas` | Seata Server 副本数 | 1 | 3 |
| `image` | 容器镜像 | - | `apache/seata-server:latest` |
| `ports.consolePort` | 控制台端口 | 7091 | 7091 |
| `ports.servicePort` | 服务端口 | 8091 | 8091 |
| `ports.raftPort` | Raft 一致性端口 | 9091 | 9091 |
| `resources` | 容器资源请求/限制 | - | 见下例 |
| `persistence.volumeReclaimPolicy` | 卷回收策略 | Retain | Retain 或 Delete |
| `persistence.spec.resources.requests.storage` | 持久化卷大小 | - | 5Gi |
| `env` | 环境变量 | - | 见下例 |

#### 环境变量和 Secret 配置

通过环境变量和 Kubernetes Secret 配置 Seata Server：

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

### 开发者指南

在本地调试 Operator 时，建议使用 Minikube 或相似的本地 Kubernetes 环境。

#### 方式 1：构建并部署 Docker 镜像

修改代码后重新构建 Controller 镜像：

```shell
# 启动 minikube 并设置 Docker 环境
minikube start
eval $(minikube docker-env)

# 构建并部署
make docker-build deploy

# 验证部署
kubectl get deployment -n seata-k8s-controller-manager
```

#### 方式 2：使用 Telepresence 本地调试

使用 [Telepresence](https://www.telepresence.io/) 在本地调试，无需构建容器镜像。

**前置要求：**
- 安装 [Telepresence CLI](https://www.telepresence.io/docs/latest/quick-start/)
- 安装 [Traffic Manager](https://www.getambassador.io/docs/telepresence/latest/install/manager#install-the-traffic-manager)

**操作步骤：**

1. 连接 Telepresence 到集群：

```shell
telepresence connect
telepresence status  # 验证连接
```

2. 生成代码资源：

```shell
make manifests generate fmt vet
```

3. 在本地运行 Controller（使用 IDE 或命令行）：

```shell
go run .
```

现在您的本地开发环境可以访问 Kubernetes 集群的 DNS 和服务。

---

## 方式二：直接部署

此方式直接使用 Kubernetes 清单部署 Seata Server，不使用 Operator。注意 Seata Docker 镜像目前需要在容器间使用 link 模式进行通信。

### 前置要求

- MySQL 数据库
- Nacos 注册中心
- Kubernetes 集群访问权限

### 部署步骤

#### 第一步：部署 Seata 及相关服务

部署 Seata 服务器、Nacos 和 MySQL：

```shell
kubectl apply -f deploy/seata-deploy.yaml
kubectl apply -f deploy/seata-service.yaml
```

#### 第二步：获取服务信息

```shell
kubectl get service
# 记录 Seata 和 Nacos 的 NodePort IP 和端口
```

#### 第三步：配置 DNS 地址

使用上一步获取的 NodePort IP 更新 `example/example-deploy.yaml` 中的地址。

#### 第四步：初始化数据库

```shell
# 连接到 MySQL 并导入 Seata 表结构
# 用实际 MySQL 服务 IP 替换 CLUSTER_IP
mysql -h <CLUSTER_IP> -u root -p < path/to/seata-db-schema.sql
```

#### 第五步：部署示例应用

部署示例微服务：

```shell
# 部署账户和库存服务
kubectl apply -f example/example-deploy.yaml
kubectl apply -f example/example-service.yaml

# 部署订单服务
kubectl apply -f example/order-deploy.yaml
kubectl apply -f example/order-service.yaml

# 部署业务服务
kubectl apply -f example/business-deploy.yaml
kubectl apply -f example/business-service.yaml
```

### 验证

打开 Nacos 控制台验证服务注册：

```
http://localhost:8848/nacos/
```

检查是否所有服务均已注册：
- account-service（账户服务）
- storage-service（库存服务）
- order-service（订单服务）
- business-service（业务服务）

### 测试验证

使用以下 curl 命令测试分布式事务场景：

#### 测试 1：账户服务 - 扣费

```shell
curl -H "Content-Type: application/json" \
  -X POST \
  --data '{"id":1,"userId":"1","amount":100}' \
  http://<CLUSTER_IP>:8102/account/dec_account
```

#### 测试 2：库存服务 - 扣库存

```shell
curl -H "Content-Type: application/json" \
  -X POST \
  --data '{"commodityCode":"C201901140001","count":100}' \
  http://<CLUSTER_IP>:8100/storage/dec_storage
```

#### 测试 3：订单服务 - 创建订单

```shell
curl -H "Content-Type: application/json" \
  -X POST \
  --data '{"userId":"1","commodityCode":"C201901140001","orderCount":10,"orderAmount":100}' \
  http://<CLUSTER_IP>:8101/order/create_order
```

#### 测试 4：业务服务 - 执行事务

```shell
curl -H "Content-Type: application/json" \
  -X POST \
  --data '{"userId":"1","commodityCode":"C201901140001","count":10,"amount":100}' \
  http://<CLUSTER_IP>:8104/business/dubbo/buy
```

用实际 NodePort 服务的 IP 地址替换 `<CLUSTER_IP>`。

---

## 故障排查

### Pod 无法启动

```shell
# 查看 Pod 日志
kubectl logs <pod-name>

# 查看 Pod 详情
kubectl describe pod <pod-name>
```

### 服务无法连接

```shell
# 测试 DNS 解析
kubectl run -it --rm debug --image=busybox --restart=Never -- nslookup seata-server-0.seata-server-cluster.default.svc
```

### 持久化卷问题

```shell
# 查看 PVC 状态
kubectl get pvc

# 查看 PV 状态
kubectl get pv
```

## 更多信息

- [Seata 官方文档](https://seata.apache.org/)
- [Kubernetes 文档](https://kubernetes.io/docs/)
- [Operator SDK 文档](https://sdk.operatorframework.io/)
