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

关联项目:

https://github.com/seata/seata

https://github.com/seata/seata-samples/tree/docker/springboot-dubbo-fescar

https://github.com/seata/seata-docker



## 方式一: 使用 Operator



### Usage

想要体验 Operator 方式部署 Seata Server 可以参照以下方式进行：

1. 克隆本仓库

   ```shell
   git clone https://github.com/apache/incubator-seata-k8s.git
   ```

3. 部署 Controller, CRD, RBAC 等资源到 Kubernetes 集群

   ```shell
   make deploy
   kubectl get deployment -n seata-k8s-controller-manager  # check if exists
   ```

4. 此时即可发布你的 CR 到集群当中了，示例可以在这里找到 [seata-server-cluster.yaml](deploy/seata-server-cluster.yaml)

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
   
   对于上面这个 CR 的例子而言，如果一切正常的话，controller 将会部署 3 个 StatefulSet 资源和一个 Headless Service 到集群中；在集群中你可以通过 seata-server-0.seata-server-cluster.default.svc 对 Seata Server 集群进行访问。

### Reference

关于 CRD 可以访问  [operator.seata.apache.org_seataservers.yaml](config/crd/bases/operator.seata.apache.org_seataservers.yaml) 以查看详细定义，这里列举出一些重要的配置并进行解读。

1. `serviceName`: 用于定义 controller 部署的 Headless Service 的名称，这会影响你访问 server 集群的方式，比如在之前的示例中，你可以通过 seata-server-0.seata-server-cluster.default.svc 进行访问。

2. `replicas`: 用于定义 Seata Server 的副本数量，你只需要调整该字段即可实现扩缩容，而不需要额外的 HTTP 请求去更改 Seata raft 集群列表

3. `image`: 定义了 Seata Server 的镜像名称

4. `ports`: 属性下会有三个端口需要设定，分别是 `consolePort`,`servicePort`,  `raftPort`，默认分别为 7091, 8091, 9091

5. `resources`: 用于定义容器的资源要求

6. `persistence.spec`: 用于定义挂载的存储资源要求

7. `persistence.volumeReclaimPolicy`: 用于控制存储回收行为，允许的选项有 `Retain` 或者 `Delete`，分别代表了在 CR 删除之后保存存储卷或删除存储卷

8. `env`: 传递给容器的环境变量，可以通过此字段去定义 Seata Server 的配置，比如：

   ```yaml
   apiVersion: operator.seata.apache.org/v1alpha1
   kind: SeataServer
   metadata:
     name: seata-server
     namespace: default
   spec:
     image: apache/seata-server:latest
     store:
       resources:
         requests:
           storage: 5Gi
     env:
     - name: console.user.username
       value: seata
     - name: console.user.password
       valueFrom:
         secretKeyRef:
           name: seata
           key: password
   ---
   apiVersion: v1
   kind: Secret
   metadata:
     name: seata
   type: Opaque
   data:
     password: seata
   ```

   

### For Developer

要在本地调试此 Operator，我们建议您使用像 Minikube 这样的测试 k8s 环境。

1. 方法 1：修改代码并构建控制器镜像：

   假设您正在使用 Minikube 进行测试，

   ```shell
   eval $(minikube docker-env)
   make docker-build deploy
   ```

2. 方法 2：不构建镜像进行本地调试

   您需要使用 Telepresence 将流量代理到 k8s 集群，参见[Telepresence 教程](https://www.telepresence.io/docs/latest/quick-start/)来安装其 CLI 工具和[Traffic Manager](https://www.getambassador.io/docs/telepresence/latest/install/manager#install-the-traffic-manager)。安装 Telepresence 后，可以按照以下命令连接到 Minikube：

   ```shell
   telepresence connect
   # 检查流量管理器是否连接
   telepresence status
   ```

   通过执行上述命令，您可以使用集群内 DNS 解析并将请求代理到集群。然后您可以使用 IDE 在本地运行或调试：

   ```shell
   # 首先确保生成适当的资源
   make manifests generate fmt vet
   
   go run .
   # 或者您也可以使用 IDE 在本地运行
   ```

## 方式二: 不使用 Operator 的示例

由于一些原因, seata docker 镜像使用暂不提供容器外部调用 ,那么需要案例相关项目也在容器内部 和 seata 镜像保持link模式

```sh
## 启动 seata deployment (nacos,seata,mysql)
kubectl create -f deploy/seata-deploy.yaml
## 启动 seata service (nacos,seata,mysql)
kubectl create -f deploy/seata-service.yaml 
## 上面会得到一个nodeport ip ( kubectl get service )
### seata-service           NodePort    10.108.3.238   <none>        8091:31236/TCP,3305:30992/TCP,8848:30093/TCP   12m
## 把ip修改到examples/examples-deploy中 用于dns寻址
## 连接到mysql 导入表结构
## 启动 example deployment (samples-account,samples-storage)
kubectl create -f example/example-deploy.yaml
## 启动 example service (samples-account,samples-storage)
kubectl create -f example/example-service.yaml
## 启动 order deployment (samples-order)
kubectl create -f example/example-deploy.yaml
## 启动 order service (samples-order)
kubectl create -f example/example-service.yaml
## 启动 business deployment (samples-dubbo-business-call)
kubectl create -f example/business-deploy.yaml 
## 启动 business deployment (samples-dubbo-service-call)
kubectl create -f example/business-service.yaml 
```

### 浏览器 打开 nacos 控制台 http://localhost:8848/nacos/ 看看所有实例是否注册成功
### 测试
```sh
# 账户服务  扣费
curl  -H "Content-Type: application/json" -X POST --data "{\"id\":1,\"userId\":\"1\",\"amount\":100}"   cluster-ip:8102/account/dec_account
# 库存服务 扣库存
curl  -H "Content-Type: application/json" -X POST --data "{\"commodityCode\":\"C201901140001\",\"count\":100}"   cluster-ip:8100/storage/dec_storage
# 订单服务 添加订单 扣费
curl  -H "Content-Type: application/json" -X POST --data "{\"userId\":\"1\",\"commodityCode\":\"C201901140001\",\"orderCount\":10,\"orderAmount\":100}"   cluster-ip:8101/order/create_order
# 业务服务 客户端seata版本太低
curl  -H "Content-Type: application/json" -X POST --data "{\"userId\":\"1\",\"commodityCode\":\"C201901140001\",\"count\":10,\"amount\":100}"   cluster-ip:8104/business/dubbo/buy
```

