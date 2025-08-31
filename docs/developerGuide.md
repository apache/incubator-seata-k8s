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

# Seata - Kubernetes Developer Guide

# Introduction

This guide aims to assist new contributors in quickly getting started, lowering the development barrier by providing detailed instructions on how to modify Custom Resource Definitions (CRDs), utilize relevant development tools, understand the source code structure, and perform debugging procedures.

---

# Catalogue

1. Custom Resource Definition (CRDs) Modification Guide

2. Development Tools Overview
3. Source Code Structure Analysis
4. Debugging Technical Guide

---

# Custom Resource Definition (CRDs) Modification Guide

## Example

In Kubernetes, Custom Resource Definitions (CRDs) extend the Kubernetes API by defining custom resources with their own structure and behavior. A CRD typically consists of two main sections:

1. **Spec (Specification)**: This section defines the desired state of the resource - what you want the system to maintain. It contains all the configuration parameters that users can set when creating or updating the resource.
2. **Status**: This section reflects the current state of the resource as observed by the controller. It provides information about the actual state of the resource, including conditions, phase, and other runtime details that help users understand the resource's current situation.

We'll begin by adding a `Labels` field, which is typically used to attach labels to resources.

**Step 1**: We need to add the `Labels` field to `SeataServerSpec` in the `api/v1alpha1/seataserver_types.go` file.

```go
// SeataServerSpec defines the desired state of SeataServer
type SeataServerSpec struct {

		//Existing fields...

		/*New field*/
    // +kubebuilder:validation:Optional
    Labels map[string]string `json:"labels,omitempty"` 
}
```

- Explanation:
  - `// +kubebuilder:validation:Optional`: Indicates that users can choose whether to provide this field when creating or updating `Kubernetes` resources.
  - `json:"labels,omitempty"`: `labels` indicates that when encoding the struct to `JSON`, this field will be named `labels`; `omitempty` means that if the `Labels` field is empty, it will be omitted from the generated `JSON`.

**Step 2**, Use the `Kubebuilder` tool to regenerate the `CRD`.

```bash
bash# Generate deepcopy 
make generate

# Generate CRD YAML 
make manifests
```

This will update the `config/crd/bases/operator.seata.apache.org_seataservers.yaml` file.

**Step 3**, We need to verify to ensure the generated `CRD` meets our expectations.

Note that running `make install` essentially uses `kubectl apply` under the hood, so you'll need to have the kubectl command-line tool installed and a properly configured kubeconfig file pointing to your cluster.

```bash
bash# Install CRD to the cluster
make install

# Verify the CRD is correctly installed
kubectl get crd seataservers.operator.seata.apache.org
```

**Step 4**, Create a new `YAML` file that defines a `SeataServer` resource and uses the newly added `Labels` field, for example in deploy/test-seata-server.yaml.

```yaml
apiVersion: operator.seata.apache.org/v1alpha1
kind: SeataServer
metadata:
  name: test-seata-server
  namespace: default
spec:
  serviceName: seata-server-cluster
  replicas: 1
  image: apache/seata-server:latest
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

**Step 5**, Deploy the test resource and verify if it was successfully created.

Use the `kubectl` command to deploy the above `YAML` file to your `Kubernetes` cluster.

```bash
kubectl apply -f test-seata-server.yaml
```

Then verify if the resource was successfully created.

```bash
kubectl get seataservers.operator.seata.apache.org test-seata-server -n default
```

After completing these operations, you should see something like this:

```bash
bash# kubectl apply -f deploy/test-seata-server.yaml
seataserver.operator.seata.apache.org/test-seata-server created
bash# kubectl get seataservers.operator.seata.apache.org test-seata-server
NAME                AGE
test-seata-server   49s
```

**Congratulations! You have successfully completed a CRD modification task!**

## Summary

So let's summarize the steps for modifying a `CRD`:

**1. Modify the CRD Type Definition**

Edit the `api/v1alpha1/seataserver_types.go` file

- Modify existing fields
- Add new fields, and add appropriate comments and validation tags for each field

**2. Generate Updated CRD Files**

After modifying the type definition, use the `Kubebuilder` tool to regenerate the `CRD`

```bash
bash# Generate deepcopy 
make generate

# Generate CRD YAML 
make manifests
```

This will update the `config/crd/bases/operator.seata.apache.org_seataservers.yaml` file

**3. Verify the CRD**

Ensure the generated `CRD` meets expectations

```bash
bash# Install CRD to the cluster
make install

# Verify the CRD is correctly installed
kubectl get crd seataservers.operator.seata.apache.org
```

**4. Construct a YAML File with the New Fields for Deployment**

Create based on your modifications

**5. Deploy and Test**

```bash
bashkubectl apply -f test-seata-server.yaml
kubectl get seataservers.operator.seata.apache.org test-seata-server -n default
```

**6. Delete Test Resources**

```bash
bashkubectl delete -f deploy/test-seata-server.yaml
```

# Development Tools Overview

## Makefile

- A tool for automating the build and management of projects, containing predefined commands to simplify the development workflow
- Common commands:
  - `make build`: Compile the project
  - `make deploy`: Deploy resources to a `Kubernetes` cluster
  - `make clean`: Clean up generated files and build artifacts
- Usage example: [Deployment operations in readme](https://github.com/apache/incubator-seata-k8s/blob/master/README.md)

## Kubebuilder

- A powerful framework for quickly building Kubernetes API extensions and controllers, which creates a complete project structure with necessary configuration files, code templates, and makefiles. It can automatically generate a large amount of template code using the `controller-gen` tool.

- Usage:

  - Initialize a new project structure, including necessary configuration files and code templates:

    ```bash
    bashkubebuilder init --domain example.com
    ```

  - Define API versions and types for custom resources in the `api` directory, example: [api/v1alpha1/seataserver_types.go](https://github.com/apache/incubator-seata-k8s/blob/master/api/v1alpha1/seataserver_types.go)

  - Use annotations starting with `+kubebuilder:` to describe the properties and behaviors of custom resources:

    ```go
    go// +kubebuilder:validation:Optional
    // +kubebuilder:default=1
    Replicas int32 `json:"replicas"`
    ```

## controller-gen

- A code generation tool used to automate the generation of boilerplate code in the Kubernetes controller development process

- Usage:

  - Generate CRDs
    The `manifests` target is defined in the `Makefile`:

    ```makefile
    makefile.PHONY: manifests
    manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
        $(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
    ```

    When executing the `make manifests` command, `controller-gen` scans all Go files in the project, generates corresponding CRD files based on the `+kubebuilder` annotations, and outputs them to the `config/crd/bases` directory.

  - Generate `DeepCopy` methods
    The `generate` target is defined in the `Makefile`:

    ```makefile
    makefile.PHONY: generate
    generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
        $(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
    ```

    When executing the `make generate` command, `controller-gen` scans all Go files in the project and generates corresponding DeepCopy methods based on the struct definitions.

There are other common development tools such as `Docker`, etc., which will not be elaborated further.

---

# Source Code Structure Analysis

## /api Directory

Contains API definitions, such as v1alpha1 version:

- common.go - Shared API types and constants
- groupversion_info.go - API group and version information
- seataserver_types.go - Type definitions for the SeataServer CRD
- zz_generated.deepcopy.go - Automatically generated deepcopy functions

## /config Directory

Contains Kubernetes configuration files, mainly using kustomize for configuration management:

- /crd - Custom Resource Definitions
- /default - Default configurations
- /manager - Operator manager configurations
- /manifests - Resource manifests for packaged deployment
- /prometheus - Monitoring configurations
- /rbac - Role-Based Access Control configurations
- /samples - Sample configurations
- /scorecard - Operator SDK scorecard configurations

## /controllers Directory

Contains controller logic, responsible for monitoring and reconciling Kubernetes resources:

- seataserver_controller.go - Controller implementation for SeataServer resources

## /deploy Directory

Contains deployment-related YAML files:

- seata-server-cluster.yaml - Cluster mode deployment configuration
- seata-server-standalone.yaml - Standalone mode deployment configuration

## /example Directory

Contains example applications and configurations:

- /client - Client examples (business, order services, etc.)
- /server - Server deployment examples

## /hack Directory

Contains development and build tools:

- boilerplate.go.txt - Source code file header template

## /pkg Directory

Contains core functionality packages:

- /seata - Seata-related functionality (resource retrieval, generation, synchronization logic)
- /utils - Common utility functions

## Root Directory Files

- .asf.yaml - Apache Software Foundation configuration file
- .dockerignore - Docker build ignore file
- .gitignore - Git ignore file
- Dockerfile - Definition for building the operator image
- LICENSE - Project license
- Makefile - Commands for building and managing the project
- PROJECT - Operator SDK project configuration
- [README.md](http://README.md) - English documentation
- [README.zh.md](http://README.zh.md) - Chinese documentation
- go.mod and go.sum - Go module dependency management
- main.go - Program entry point

# Debugging Technical Guide

## Local Environment Setup and Deployment Verification

Here I'll mainly discuss local environment setup. The prerequisite for verifying controller deployment and other resources in the [readme](https://github.com/apache/incubator-seata-k8s/blob/master/README.md) is having a `kubernetes` environment. If you don't have one yet, I recommend using [kind (kubernetes in docker)](https://kind.sigs.k8s.io/docs/user/quick-start/). You'll need to configure [docker](https://docs.docker.com/get-started/get-docker/) and [kubectl](https://kubernetes.io/docs/tasks/tools/) first. The documentation in these links is already very detailed, so you can just follow along, or you can search for tutorials online based on the tool names mentioned above.

I recommend reading the official documentation to develop the habit of consulting development documentation when learning tools.

## Log Analysis

- If a Seata Server has issues, you can check the error messages in the logs for problems like transaction rollbacks, connection failures, etc.

- First, you can check the StatefulSet to get the pod names:

  ```bash
  kubectl get statefulset -n <namespace>
  ```

- Then use `kubectl logs` to view the `Seata` `Pod` logs:

  ```bash
  kubectl logs <seata-pod-name> -n <namespace>
  ```

### Checking Kubernetes Resource Status

- `Pod` Status
  If the `Seata Server` `Pod` is in an `Error` or other abnormal state, you can use the following command to check the `Pod` status:

  ```bash
  kubectl get pods -n <namespace>
  ```

- Check Services and Ports
  Ensure that services are exposing ports correctly:

  ```bash
  kubectl get svc -n <namespace>
  ```

### Network Communication Issues

- Check Network Connections Between `Pods`
  Network issues may occur in communication between `Seata Server` and databases or `Seata Clients`. In this case, enter the `Pod` to see if you can ping other resources:

  ```bash
  kubectl exec -it <pod-name> -n <namespace> -- /bin/bash
  ping <other-pod-name>
  ```

### Resource Management and Limitations

- In `Kubernetes`, resource limits like CPU and memory may be set for `Seata Server`. If resources are not configured properly, issues like `Pod` crashes may occur. View detailed information about the `Seata Pod`:

  ```bash
  kubectl describe pod <seata-pod-name> -n <namespace>
  ```
