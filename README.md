# seata-k8s

关联项目:

https://github.com/seata/seata

https://github.com/seata/seata-samples/tree/docker/springboot-dubbo-fescar

https://github.com/seata/seata-docker

## 方式一: 查看官网在WIN/MAC 快速体验


## 方式二: 使用帮助
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

