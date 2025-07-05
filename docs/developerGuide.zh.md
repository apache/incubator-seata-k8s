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

Seata - K8s 开发者指南
=====================



# 引言

本指南旨在帮助新的贡献者快速入门，降低开发门槛，详细介绍如何修改自定义资源定义(CRD)、使用相关开发工具、理解源代码结构以及进行调试。

 ---

# 目录

1. 自定义资源定义（CRDs）修改指南

2. 相关开发工具说明

3. 源码结构解析

4. 调试技术指南

---

# 自定义资源定义（CRDs）修改指南

### CRD的整体结构

CRD 是 Kubernetes 的扩展机制，允许用户创建自定义资源类型（如SeataServer），就像使用内置资源（如Pod、Service）一样。CRD 定义了资源的结构、验证规则和行为

一个完整的CRD包含以下核心部分：

1. **元数据**（Metadata）：定义资源的名称、组、版本等
2. **Spec**：用户定义的期望状态
3. **Status**：控制器维护的实际状态
4. **验证规则**：使用OpenAPI v3 schema定义字段约束

### Spec与Status的作用

#### Spec（期望状态）
- 用户通过YAML文件定义，如`deploy/test-seata-server.yaml`
- 通常包含资源的配置参数（如副本数、镜像、环境变量等）
- 控制器根据Spec的内容执行操作（如创建Pod、配置服务）

#### Status（实际状态）
- 由控制器自动更新，反映资源的当前状态
- 包含运行时信息（如可用副本数、健康检查结果、事件历史）
- 用户可通过`kubectl get <resource> -o yaml`查看

### 示例：SeataServer CRD的Spec与Status

以下是简化的SeataServer资源结构：

```yaml
apiVersion: operator.seata.apache.org/v1alpha1
kind: SeataServer
metadata:
  name: test-seata-server
spec:
  replicas: 1
  image: seataio/seata-server:latest
  serviceName: seata-server-cluster
  persistence:
    volumeReclaimPolicy: Delete
status:
  replicas: 1
  availableReplicas: 1
  phase: Running
  conditions:
    - type: Available
      status: "True"
      lastTransitionTime: "2023-10-01T12:00:00Z"
```

### 修改示例

我们以添加一个`Labels`字段开始，这个字段通常用于给资源添加标签

**第一步**，我们应该在 `api/v1alpha1/seataserver_types.go` 文件里，为 `SeataServerSpec` 增加 `Labels` 字段   

```go
// SeataServerSpec defines the desired state of SeataServer
type SeataServerSpec struct {

    //现有字段...

    /*新增字段*/
    // +kubebuilder:validation:Optional
    Labels map[string]string `json:"labels,omitempty"` 
}
```

解释：  

- `// +kubebuilder:validation:Optional`：表示在创建|更新`Kubernetes`资源时，用户可以选择是否提供该字段

- `json:"labels,omitempty"` ：`labels`表示在将结构体编码为`JSON`时，该字段被命名为`labels`；`omitempty`表示若`Labels`字段为空，则把他从生成的`JSON`中省略

**第二步**，使用`Kubebuilder`工具重新生成`CRD`

```bash
# 生成 deepcopy 
make generate

# 生成 CRD YAML 
make manifests
```

这将更新`config/crd/bases/operator.seata.apache.org_seataservers.yaml`文件

**第三步**，我们要验证以确保生成的`CRD`符合预期

```bash
# 安装 CRD 到集群
make install

# `make install`本质上是通过`kubectl apply`命令将生成的CRD YAML文件（位于`config/crd/bases`目录）应用到Kubernetes集群中。执行该命令前需确保：  
# 1. 已安装`kubectl`命令行工具（用于与Kubernetes集群交互）；  
# 2. 本地环境已配置有效的`kubeconfig`（通常位于`~/.kube/config`，或通过`KUBECONFIG`环境变量指定），且该配置具有操作集群CRD的权限。

# 验证 CRD 是否正确安装
kubectl get crd seataservers.operator.seata.apache.org
```

**第四步**，创建一个新的`YAML`文件，定义一个`SeataServer`资源并使用新添加的`Labels`字段，示例为在deploy/test-seata-server.yaml

```yaml
apiVersion: operator.seata.apache.org/v1alpha1
kind: SeataServer
metadata:
  name: test-seata-server
  namespace: default
spec:
  serviceName: seata-server-cluster
  replicas: 1
  image: seataio/seata-server:latest
  persistence:
    volumeReclaimPolicy: Delete
  store:
    resources:
      requests:
        storage: 5Gi
  labels:
    app: seata
    env: test
```

**第五步**，部署测试资源并验证是否成功创建

用`kubectl`命令把上述`YAML`文件部署到`Kubernetes`集群中



```bash
kubectl apply -f test-seata-server.yaml
```



然后验证资源是否成功创建



```bash
kubectl get seataservers.operator.seata.apache.org test-seata-server -n default9s
```



如果你操作完差不多是这样的：   

```bash
flypiggyyoyoyo@LAPTOP-DTIVSINL:/mnt/d/OpenSource/SeataGo-k8s/incubator-seata-k8s$ kubectl apply -f deploy/test-seata-server.yaml
seataserver.operator.seata.apache.org/test-seata-server created
flypiggyyoyoyo@LAPTOP-DTIVSINL:/mnt/d/OpenSource/SeataGo-k8s/incubator-seata-k8s$ kubectl get seataservers.operator.seata.apache.org test-seata-server
NAME                AGE
test-seata-server   49s
```

**恭喜你已经成功完成一次修改CRD的任务啦！**



### 总结

所以接下来我们总结一下修改`CRD`的几个步骤：      

**1.修改CRD类型定义**

编辑`api/v1alpha1/seataserver_types.go`文件

- 修改现有字段

- 添加新字段，并为每个字段添加适当的注释和验证标签

**2.生成更新后的CRD文件**

修改类型定义后，使用`Kubebuilder`工具重新生成`CRD`

```bash
# 生成 deepcopy 
make generate

# 生成 CRD YAML 
make manifests
```

这将更新`config/crd/bases/operator.seata.apache.org_seataservers.yaml`文件

**3.验证CRD**

确保生成的`CRD`符合预期

```bash
# 安装 CRD 到集群
make install

# 验证 CRD 是否正确安装
kubectl get crd seataservers.operator.seata.apache.org
```

**4.构造部署含有新字段的YAML文件**  

根据修改创建

**5.部署并测试**

```bash
kubectl apply -f test-seata-server.yamld
kubectl get seataservers.operator.seata.apache.org test-seata-server -n default9s 
```

**6.删除测试资源**

```bash
kubectl delete -f deploy/test-seata-server.yaml
```

---

# 相关开发工具说明



### Makefile

- 一个用于自动化构建和管理项目的工具，包含预定义的命令来简化开发流程

- 常用命令：  
  
  - `make build`：编译项目
  
  - `make deploy`： 部署资源到`Kubernetes`集群
  
  - `make clean`： 清理生成的文件和构建产物

- 使用示例：[readme中部署资源操作]([incubator-seata-k8s/README.md at master · apache/incubator-seata-k8s · GitHub](https://github.com/apache/incubator-seata-k8s/blob/master/README.md))
  
  

### Kubebuilder

- 一个用于快速构建`Kubernetes API`扩展和控制器的强大框架，可创建一个包含必要的配置文件、代码模板、makefile的完整的项目结构，可借助`controller-gen`工具自动生成大量模板代码

- 用法：
  
  - 初始化一个新的项目结构，包含必要的配置文件和代码模板
    
    ```bash
    kubebuilder init --domain example.com
    ```
  
  - 在 `api` 目录下定义自定义资源的 API 版本和类型，示例：[api/v1alpha1/seataserver_types.go](https://github.com/apache/incubator-seata-k8s/blob/master/api/v1alpha1/seataserver_types.go)
  
  - 可以用`+kubebuilder：`开头的注解来描述自定义资源的属性和行为
    
    ```go
    // +kubebuilder:validation:Optional1
    // +kubebuilder:default=1
    Replicas int32 `json:"replicas"`
    ```

### controller-gen

- 一个代码生成工具，用于自动化生成`Kubernetes`控制器开发过程中的样板代码

- 用法：
  
  - 生成CRD     
    在`Makefile`中定义了`manifests`目标
    
    ```makefile
    .PHONY: manifests
    manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
        $(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
    ```
    
    执行`make manifests`命令时，`controller - gen` 会扫描项目中的所有 Go 文件，根据其中的 `+kubebuilder` 注解生成相应的 CRD 文件，并将其输出到 `config/crd/bases` 目录下
  
  - 生成`DeepCopy`方法   
    在`Makefile`中的定义了`generate`目标
    
    ```makefile
    .PHONY: generate
    generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
        $(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
    ```
    
    执行 `make generate` 命令时，`controller - gen` 会扫描项目中的所有 Go 文件，根据其中的结构体定义生成相应的 DeepCopy 方法
    
    

还有一些常用的开发工具如`Docker`等，笔者不再赘述

---

# 源码结构解析

### /api 目录

包含 API 定义，比如v1alpha1 版本：

* common.go - 共享的 API 类型和常量
* groupversion_info.go - API 组和版本信息
* seataserver_types.go - SeataServer CRD 的类型定义
* zz_generated.deepcopy.go - 自动生成的deepcopy函数

### /config 目录

包含 Kubernetes 配置文件，主要使用 kustomize 进行配置管理：

* /crd - 自定义资源定义
* /default - 默认配置
* /manager - 操作器管理器配置
* /manifests - 打包部署的资源清单
* /prometheus - 监控配置
* /rbac - 基于角色的访问控制配置
* /samples - 示例配置
* /scorecard - Operator SDK 评分卡配置

### /controllers 目录

包含控制器逻辑，负责监控和调协 Kubernetes 资源：

* seataserver_controller.go - SeataServer 资源的控制器实现

### /deploy 目录

包含部署相关的 YAML 文件：

* seata-server-cluster.yaml - 集群模式部署配置
* seata-server-standalone.yaml - 单机模式部署配置

### /example 目录

包含示例应用和配置：

* /client - 客户端示例（业务、订单服务等）
* /server - 服务器部署示例

### /hack 目录

包含开发和构建工具：

* boilerplate.go.txt - 源代码文件头模板

### /pkg 目录

包含核心功能包：

* /seata - Seata 相关功能（资源获取，生成，同步逻辑）
* /utils - 通用工具函数

### 根目录文件

* .asf.yaml - Apache 软件基金会配置文件
* .dockerignore - Docker 构建忽略文件
* .gitignore - Git 忽略文件
* Dockerfile - 构建操作器镜像的定义
* LICENSE - 项目许可证
* Makefile - 构建和管理项目的命令
* PROJECT - Operator SDK 项目配置
* README.md - 英文说明文档
* README.zh.md - 中文说明文档
* go.mod 和 go.sum - Go 模块依赖管理
* main.go - 程序入口点
  
  

# 针对Seata-k8s项目的调试技术指导

### 本地环境搭建与验证部署

这里主要说一下本地环境搭建，在[readme](https://github.com/apache/incubator-seata-k8s/blob/master/README.md)中验证部署控制器等资源的前提是有一个`kubernetes`环境。如果你还没有，推荐使用[kind(kubernetes in docker)启动]([kind – Quick Start]([kind – Quick Start](https://kind.sigs.k8s.io/docs/user/quick-start)))，需要先配置好[docker](https://docs.docker.com/get-started/get-docker/)和[kubectl](https://kubernetes.io/docs/tasks/tools/)，链接中的文档已经很详细了，跟着来即可，亦可根据上文提及的工具名词去网上自己找教程。     

其他错误请查阅kubebuilder官方文档

### 日志分析

- 如果某个 Seata Server 出现问题，可以查看日志输出的错误信息，检查是否有事务回滚、连接失败等问题

- 先通过 StatefulSet 获取 Seata 对应的 Pod 名称（Seata 通常以 StatefulSet 部署）

  ```bash
  kubectl get statefulset -n <namespace>  # 查看 Seata 的 StatefulSet 名称
  kubectl get pods -n <namespace> | grep <statefulset名称>  # 筛选出对应的 Pod
  ```

- 使用`kubectl logs`查看`Seata`的`Pod`日志
  
  ```bash
  kubectl logs <seata-pod-name> -n <namespace>
  ```

### 检查kubernets资源状态

- 检查服务和端口
  
  确保服务暴漏端口正确
  
  ```bash
  kubectl get svc -n <namespace>
  ```

### 网络通信问题

- 检查`Pod`之间的网络链接    
  
  `Seata Server`和数据库、`Seata Client`之间通信可能会出现网络问题，这时候进入`Pod`内部看看能不能`ping`通
  
  ```bash
  kubectl exec -it <pod-name> -n <namespace> -- /bin/bash
  ping <other-pod-name>
  ```

### 资源管理和限制

- `Kubernetes`中可能为`Seata Server`设置了`cpu`、内存等资源限制，若资源配置不当，可能出现`Pod`崩溃等问题，查看`Seata Pod`的详细信息
  
  ```bash
  kubectl describe pod <seata-pod-name> -n <namespace>
  ```


