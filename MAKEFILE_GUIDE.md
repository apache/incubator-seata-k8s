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

# Makefile 使用指南

## 快速开始

### 查看帮助

```bash
make help          # 显示所有可用命令
make info          # 显示构建信息
make docs          # 显示完整文档
```

### 开发工作流

```bash
# 1. 设置开发环境
make setup         # 安装所有构建依赖

# 2. 运行开发环境
make run           # 运行控制器（实时重新加载）

# 3. 快速测试
make quick-test    # 快速测试（无覆盖率）
make test          # 完整测试（含覆盖率）

# 4. 代码检查
make fmt           # 格式化代码
make vet           # 代码分析
make lint          # 完整检查（fmt + vet）
```

## 常用命令

### 编译和构建

```bash
# 快速编译（无测试）
make quick-build

# 完整编译（包含所有检查）
make build

# Docker 镜像构建
make docker-build

# 跨平台 Docker 构建（需要 buildx）
make docker-buildx

# 全平台支持
make docker-buildx PLATFORMS=linux/amd64,linux/arm64
```

### Helm 操作

```bash
# 打包 Helm Chart
make helm-package

# 生成 Helm 值文件
make helm-values

# 安装到集群
make helm-install

# 升级集群中的 Chart
make helm-upgrade

# 卸载 Chart
make helm-uninstall

# 验证 Chart
helm lint helm/seata-server
```

### 测试和检查

```bash
# 运行单元测试
make test

# 快速测试（跳过长运行测试）
make quick-test

# 生成覆盖率报告
make coverage

# 运行所有检查
make check-all
```

### 代码质量

```bash
# 格式化代码
make fmt

# 代码分析
make vet

# 运行所有 linting
make lint

# 生成覆盖率（HTML）
make coverage
# 打开 coverage.html 查看
```

## 发布工作流

### 完整发布流程

```bash
# 1. 设置版本
export VERSION=1.0.0

# 2. 构建发布物
make build-release

# 3. 验证发布物
ls -la dist/

# 4. 创建 Git 标签
git tag v$(VERSION)
git push origin v$(VERSION)

# 5. 推送所有工件（Docker、Bundle、Catalog）
make release-all

# 或者一步完成
make VERSION=1.0.0 release-all
```

### 单独操作

```bash
# 只构建本地工件
make build-release

# 只推送 Docker 镜像
make docker-push

# 推送 Bundle 和 Catalog
make bundle-push
make catalog-push

# 一次性推送所有
make release-push
```

## CI/CD 集成

### GitHub Actions 示例

```yaml
name: Build and Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2
      
      - name: Login to Docker Hub
        uses: docker/login-action@v2
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_PASSWORD }}
      
      - name: Build and release
        run: |
          VERSION=${GITHUB_REF#refs/tags/v}
          make VERSION=$VERSION release-all
```

## 环境变量

### 常用环境变量

```bash
# 版本控制
VERSION=1.0.0                           # 项目版本
GIT_COMMIT=$(git rev-parse --short HEAD) # Git commit
GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD) # Git branch

# Docker 镜像
IMG=docker.io/apache/seata-controller:latest  # 控制器镜像
BUNDLE_IMG=docker.io/apache/seata-bundle:1.0.0
CATALOG_IMG=docker.io/apache/seata-catalog:1.0.0

# Helm
HELM_CHART_DIR=helm/seata-server        # Chart 目录
RELEASE_DIR=dist                        # 发布目录

# 构建
PLATFORMS=linux/amd64,linux/arm64       # 目标平台
```

### 设置环境变量

```bash
# 导出环境变量（临时）
export VERSION=1.0.0
make build-release

# 或者在命令行指定
make VERSION=1.0.0 build-release

# 或者创建 .env 文件（本地开发）
echo "VERSION=1.0.0" > .env
source .env
```

## 发布目录结构

```
dist/
├── seata-server-1.0.0.tgz      # Helm Chart 包
├── values-1.0.0.yaml           # Helm 值文件
├── coverage.html                # 覆盖率报告
└── ...
```

## 完整命令参考

### 开发

| 命令 | 说明 |
|-----|------|
| `make setup` | 安装所有依赖 |
| `make run` | 运行控制器 |
| `make test` | 运行完整测试 |
| `make quick-test` | 快速测试 |
| `make quick-build` | 快速编译 |
| `make build` | 完整编译 |

### 代码质量

| 命令 | 说明 |
|-----|------|
| `make fmt` | 代码格式化 |
| `make vet` | 代码分析 |
| `make lint` | 完整 linting |
| `make check-all` | 所有检查 |
| `make coverage` | 覆盖率报告 |

### Docker

| 命令 | 说明 |
|-----|------|
| `make docker-build` | 构建 Docker 镜像 |
| `make docker-push` | 推送 Docker 镜像 |
| `make docker-buildx` | 跨平台构建 |

### Helm

| 命令 | 说明 |
|-----|------|
| `make helm-package` | 打包 Chart |
| `make helm-values` | 生成值文件 |
| `make helm-install` | 安装到集群 |
| `make helm-upgrade` | 升级集群 |
| `make helm-uninstall` | 卸载 |

### 发布

| 命令 | 说明 |
|-----|------|
| `make build-release` | 构建发布物 |
| `make release-push` | 推送所有镜像 |
| `make release-all` | 完整发布 |

### CI/CD

| 命令 | 说明 |
|-----|------|
| `make ci` | CI 流程 |
| `make cd` | CD 流程 |

### 信息

| 命令 | 说明 |
|-----|------|
| `make help` | 帮助信息 |
| `make info` | 构建信息 |
| `make docs` | 文档 |
| `make version` | 版本信息 |

### 清理

| 命令 | 说明 |
|-----|------|
| `make clean` | 清理构建物 |

## 工作流示例

### 示例 1: 本地开发

```bash
# 1. 安装依赖
make setup

# 2. 开发和测试循环
make fmt
make vet
make quick-test
make run

# 3. 完整验证
make check-all
```

### 示例 2: 发布新版本

```bash
# 1. 确保所有测试通过
make test

# 2. 构建发布物
make VERSION=1.1.0 build-release

# 3. 验证发布物
ls -la dist/

# 4. 标记版本
git tag v1.1.0
git push origin v1.1.0

# 5. 推送所有工件
make VERSION=1.1.0 release-all
```

### 示例 3: Docker 镜像构建

```bash
# 本地构建
make IMG=myregistry/seata:1.0.0 docker-build
make IMG=myregistry/seata:1.0.0 docker-push

# 跨平台构建
make IMG=myregistry/seata:1.0.0 PLATFORMS=linux/amd64,linux/arm64 docker-buildx
```

### 示例 4: Helm 集群部署

```bash
# 开发部署
make helm-package
make helm-install

# 验证安装
helm list -n seata-k8s-controller-manager

# 更新代码后升级
make helm-upgrade

# 清理
make helm-uninstall
```

## 故障排查

### 问题 1: 命令找不到

```bash
# 错误：command not found: controller-gen
# 解决：
make setup
# 或者
make controller-gen
```

### 问题 2: 权限错误

```bash
# 错误：permission denied
# 解决：
chmod +x bin/*
# 或重新安装
make setup --always-make
```

### 问题 3: Docker 构建失败

```bash
# 错误：docker: command not found
# 解决：
# 1. 安装 Docker
# 2. 启动 Docker daemon
docker ps  # 验证 Docker 是否运行

# 对于跨平台构建，需要 buildx
docker run --rm --privileged tonistiigi/binfmt --install all
```

### 问题 4: Helm Chart 验证失败

```bash
# 错误：chart validation failed
# 解决：
helm lint helm/seata-server  # 查看详细错误
# 修复 values.yaml 或 templates/
make helm-package
```

## 最佳实践

1. **开发前**：运行 `make setup` 安装所有依赖
2. **提交前**：运行 `make check-all` 确保代码质量
3. **发布前**：运行 `make build-release` 构建所有工件
4. **CI/CD**：使用 `make ci` 和 `make cd` 自动化流程

## 自定义

### 创建自定义目标

在 Makefile 中添加：

```makefile
.PHONY: my-target
my-target: ## My custom target
	@echo "Running my custom target"
	# 你的命令
```

### 覆盖默认值

```bash
# 命令行
make VERSION=2.0.0 IMG=myregistry/controller:2.0.0 build-release

# 或创建 Makefile.local
# Makefile.local:
# VERSION = 2.0.0
# IMG = myregistry/controller:2.0.0
```

## 更新日志

| 版本 | 日期 | 说明 |
|-----|-----|------|
| 1.0 | 2025-11-29 | 初始版本 |

---

## 相关文档

- [Kubernetes Operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [Helm](https://helm.sh/docs/)
- [Docker Buildx](https://docs.docker.com/build/buildx/)
- [operator-sdk](https://sdk.operatorframework.io/)

