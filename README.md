# IPShield

IPShield Operator for OpenShift simplifies IP whitelisting for routes by automating the process of applying whitelisting annotations. OpenShift routes support IP whitelisting via `haproxy.router.openshift.io/ip_whitelist` annotation, but manually managing these can be cumbersome. With IPShield, users specify a label selector and a set of IP ranges, and the operator automatically applies the necessary annotations to all matching routes. This ensures consistency and reduces operational overhead.

## Features

- **Automated IP Whitelisting:** Dynamically applies whitelisting annotations based on user-defined label selectors and IP ranges

- **Quick Enable/Disable:** Only applies to routes with the annotation ip-whitelist.stakater.cloud/enabled set to true.
- **Configurable Watch Namespace:** Users can configure the `WATCH_NAMESPACE` environment variable. Operator will apply CRDs only from this namespace.
- **Whitelist State Preservation:** If a whitelist annotation exists before the CRD is applied, it is stored in a ConfigMap and restored when the CRD is removed.
- **Continuous Monitoring:** Watches for changes in routes and updates annotations accordingly.
- **Reduced Manual Effort:** Eliminates the need for users to manually update route annotations.
- **Seamless Integration:** Works with existing OpenShift route configurations.

## Getting Started

### Prerequisites
- go version v1.21.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/ipshield-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/ipshield-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/ipshield-operator:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/ipshield-operator/<tag or branch>/dist/install.yaml
```
## Usage
1. Define a RouteWhitelist custom resource (CR) specifying the label selector and allowed IP ranges:
```yaml
apiVersion: networking.stakater.com/v1alpha1
kind: RouteWhitelist
metadata:
  labels:
    app.kubernetes.io/name: ipshield-operator
    app.kubernetes.io/managed-by: kustomize
  name: routewhitelist-sample
spec:
  labelSelector:
    matchLabels:
      app: "ip-test"
  ipRanges:
    - 10.100.110.11
    - 10.100.110.12
```
2. Apply the `ip-whitelist.stakater.cloud/enabled` label to the desired route e.g.
```sh
kubectl label routes nginx-deployment ip-whitelist.stakater.cloud/enabled=true -n mywebserver --overwrite
```
3. Apply this whitelist to a namespace watched by IPShield operator
```sh
kubectl apply -f whitelist.yaml -n $WATCH_NAMESPACE
```

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

