# seata-k8s

[中文文档](README.zh.md) 

Associated Projects:

- [https://github.com/seata/seata](https://github.com/seata/seata)
- [https://github.com/seata/seata-samples/tree/docker/springboot-dubbo-fescar](https://github.com/seata/seata-samples/tree/docker/springboot-dubbo-fescar)
- [https://github.com/seata/seata-docker](https://github.com/seata/seata-docker)

## Method 1: Using Operator

### Usage

To experience deploying Seata Server using the Operator method, follow these steps:

1. Clone this repository:

   ```shell
   git clone https://github.com/apache/incubator-seata-k8s.git
   ```

2. (Optional) Build and publish the controller image to a private registry:

   > This step can be skipped, the operator use seataio/seata-controller:latest as controller image by default.

   ```shell
   IMG=${IMAGE-TO-PUSH} make docker-build docker-push
   ```

   If you are using minikube for testing, you can skip the above publishing process with the following command:

   ```shell
   eval $(minikube docker-env)
   IMG=${IMAGE-TO-PUSH} make docker-build
   ```

3. Deploy Controller, CRD, RBAC, and other resources to the Kubernetes cluster:

   ```shell
   make deploy
   kubectl get deployment -n seata-k8s-controller-manager  # check if exists
   ```

4. You can now deploy your CR to the cluster. An example can be found here [seata-server-cluster.yaml](deploy/seata-server-cluster.yaml):

   ```yaml
   apiVersion: operator.seata.io/v1alpha1
   kind: SeataServer
   metadata:
     name: seata-server
     namespace: default
   spec:
     serviceName: seata-server-cluster
     replicas: 3
     image: seataio/seata-server:latest
     store:
       resources:
         requests:
           storage: 5Gi
   ```

   For the example above, if everything is correct, the controller will deploy 3 StatefulSet resources and a Headless Service to the cluster. You can access the Seata Server cluster in the cluster through `seata-server-0.seata-server-cluster.default.svc`.

### Reference

For CRD details, you can visit [operator.seata.io_seataservers.yaml](config/crd/bases/operator.seata.io_seataservers.yaml). Here are some important configurations:

1. `serviceName`: Used to define the name of the Headless Service deployed by the controller. This will affect how you access the server cluster. In the example above, you can access the Seata Server cluster through `seata-server-0.seata-server-cluster.default.svc`.

2. `replicas`: Defines the number of Seata Server replicas. Adjusting this field achieves scaling without the need for additional HTTP requests to change the Seata raft cluster list.

3. `image`: Defines the Seata Server image name.

4. `ports`: Three ports need to be set under the `ports` property: `consolePort`, `servicePort`, and `raftPort`, with default values of 7091, 8091, and 9091, respectively.

5. `resources`: Used to define container resource requirements.

6. `store.resources`: Used to define mounted storage resource requirements.

7. `env`: Environment variables passed to the container. You can use this field to define Seata Server configuration. For example:

   ```yaml
   apiVersion: operator.seata.io/v1alpha1
   kind: SeataServer
   metadata:
     name: seata-server
     namespace: default
   spec:
     image: seataio/seata-server:latest
     store:
       resources:
         requests:
           storage: 5Gi
     env:
       console.user.username: seata
       console.user.username: seata
   ```

## Method 2: Example without Using Operator

Due to certain reasons, Seata Docker images currently do not support external container calls. Therefore, the example projects should also be kept in link mode with the Seata image inside the container.

```sh
# Start Seata deployment (nacos,seata,mysql)
kubectl create -f deploy/seata-deploy.yaml
# Start Seata service (nacos,seata,mysql)
kubectl create -f deploy/seata-service.yaml
# Get a NodePort IP (kubectl get service)
# Modify the IP in examples/examples-deploy for DNS addressing
# Connect to MySQL and import table structure
# Start example deployment (samples-account,samples-storage)
kubectl create -f example/example-deploy.yaml
# Start example service (samples-account,samples-storage)
kubectl create -f example/example-service.yaml
# Start order deployment (samples-order)
kubectl create -f example/example-deploy.yaml
# Start order service (samples-order)
kubectl create -f example/example-service.yaml
# Start business deployment (samples-dubbo-business-call)
kubectl create -f example/business-deploy.yaml
# Start business deployment (samples-dubbo-service-call)
kubectl create -f example/business-service.yaml
```

### Open the Nacos console in your browser [http://localhost:8848/nacos/] to check if all instances are registered successfully.

### Testing

```sh
# Account service - Deduct amount
curl -H "Content-Type: application/json" -X POST --data "{\"id\":1,\"userId\":\"1\",\"amount\":100}" cluster-ip:8102/account/dec_account
# Storage service - Deduct stock
curl -H "Content-Type: application/json" -X POST --data "{\"commodityCode\":\"C201901140001\",\"count\":100}" cluster-ip:8100/storage/dec_storage
# Order service - Add order and deduct amount
curl -H "Content-Type: application/json" -X POST --data "{\"userId\":\"1\",\"commodityCode\":\"C201901140001\",\"orderCount\":10,\"orderAmount\":100}" cluster-ip:8101/order/create_order
# Business service - Client Seata version too low
curl -H "Content-Type: application/json" -X POST --data "{\"userId\":\"1\",\"commodityCode\":\"C201901140001\",\"count\":10,\"amount\":100}" cluster-ip:8104/business/dubbo/buy
```
