# Seata Server Helm Chart

一个用于在 Kubernetes 上部署 Apache Seata Server 的 Helm Chart。

## 前置要求

- Kubernetes 1.16+
- Helm 3+
- 如果使用 Cluster 模式，需要安装 seata-k8s operator

## 快速开始

### 1. 使用默认配置安装

```bash
helm install seata-server ./seata-server
```

### 2. 使用自定义命名空间安装

```bash
helm install seata-server ./seata-server -n seata --create-namespace
```

### 3. 使用自定义值安装

```bash
helm install seata-server ./seata-server -f values.yaml
```

## 配置

### 部署模式

#### Cluster 模式（推荐）

使用 SeataServer CRD 和 seata-k8s operator 进行部署：

```bash
helm install seata-server ./seata-server \
  --set seataServer.mode=cluster \
  --set seataServer.replicas=3
```

#### Standalone 模式

使用 Kubernetes Deployment 直接部署：

```bash
helm install seata-server ./seata-server \
  --set seataServer.mode=standalone \
  --set seataServer.replicas=1
```

### 常用配置参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `seataServer.mode` | cluster | 部署模式：cluster 或 standalone |
| `seataServer.replicas` | 3 | 副本数 |
| `seataServer.image.tag` | latest | 镜像标签 |
| `seataServer.persistence.size` | 5Gi | 持久化存储大小 |
| `seataServer.service.type` | ClusterIP | Service 类型 |
| `seataServer.nodePort.enabled` | false | 是否启用 NodePort |
| `secret.password` | seata123456 | Seata 控制台密码 |

### 完整配置示例

```yaml
# 集群模式配置
seataServer:
  mode: cluster
  replicas: 3
  image:
    repository: apache/seata-server
    tag: latest
  persistence:
    enabled: true
    size: 10Gi
    reclaimPolicy: Retain
  resources:
    requests:
      cpu: 200m
      memory: 512Mi
    limits:
      cpu: 1000m
      memory: 1Gi

# 启用 NodePort
seataServer:
  nodePort:
    enabled: true
    ports:
      service: 30091
      console: 30092
```

## 使用示例

### 示例 1：使用默认配置（Cluster 模式）

```bash
helm install seata-server ./seata-server
```

### 示例 2：使用 NodePort 访问

```bash
helm install seata-server ./seata-server \
  --set seataServer.nodePort.enabled=true
```

### 示例 3：使用自定义密码和存储

```bash
helm install seata-server ./seata-server \
  --set secret.password=my-secret-password \
  --set seataServer.persistence.size=20Gi
```

### 示例 4：使用 Standalone 模式

```bash
helm install seata-server ./seata-server \
  --set seataServer.mode=standalone \
  --set seataServer.replicas=1
```

## 常用命令

### 查看部署状态

```bash
# 查看 SeataServer 资源
helm list
kubectl get seataserver

# 查看 Pod 状态
kubectl get pods -l app=seata-server

# 查看服务
kubectl get service seata-server-cluster
```

### 升级发行版

```bash
helm upgrade seata-server ./seata-server \
  --set seataServer.replicas=5
```

### 卸载发行版

```bash
helm uninstall seata-server
```

### 查看生成的清单

```bash
helm template seata-server ./seata-server
helm template seata-server ./seata-server -f custom-values.yaml
```

### 验证配置

```bash
helm lint ./seata-server
```

## 访问 Seata Console

### ClusterIP 模式

```bash
kubectl port-forward svc/seata-server-cluster 7091:7091

# 访问: http://localhost:7091
```

### NodePort 模式

```bash
kubectl get nodes -o wide
# 使用 NodePort 和 Node IP 访问: http://<node-ip>:30092
```

## 故障排查

### 查看 Pod 日志

```bash
kubectl logs -f pod/seata-server-0
```

### 查看事件

```bash
kubectl describe seataserver seata-server
kubectl get events
```

### 查看 Status

```bash
kubectl get seataserver seata-server -o yaml | grep status -A 10
```

## 卸载

```bash
helm uninstall seata-server -n <namespace>
```

## 相关文档

- [Apache Seata 官方文档](https://seata.apache.org/)
- [seata-k8s GitHub 仓库](https://github.com/apache/incubator-seata-k8s)
- [Helm 文档](https://helm.sh/docs/)

## 许可证

Apache License 2.0

## 社区支持

- [Seata GitHub Issues](https://github.com/seata/seata/issues)
- [Seata Community](https://seata.apache.org/community)

