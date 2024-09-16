# opsy-ami-operator

A Helm chart for The Opsy AMI Operator is designed to automate the process of building AMIs using Packer and refreshing AMIs for EKS node groups.

![Version: 1.12.1](https://img.shields.io/badge/Version-1.12.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 1.12.1](https://img.shields.io/badge/AppVersion-1.12.1-informational?style=flat-square)

## Additional Information

- Automatically checks for the latest AMI, of your base AMI using provided filters.
- Creates and monitors Kubernetes jobs for AMI building if necessary using packer.
- Ensures node groups always use the `status:active` ami when under management.
- Node Group instance refreshing with an option to exclude node groups.

## Installing the Chart

```console
helm repo add opsy-ami-operator https://ibeify.github.io/opsy-ami-operator
helm repo update
```

```console
helm install [RELEASE_NAME] opsy-ami-operator/opsy-ami-operator
```
The command deploys opsy-ami-operator on the Kubernetes cluster in the default configuration.

## Uninstall Chart

```console
helm uninstall [RELEASE_NAME]
```
This removes all the Kubernetes components associated with the chart and deletes the release.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| controllerManager.kubeRbacProxy.args[0] | string | `"--secure-listen-address=0.0.0.0:8443"` |  |
| controllerManager.kubeRbacProxy.args[1] | string | `"--upstream=http://127.0.0.1:8080/"` |  |
| controllerManager.kubeRbacProxy.args[2] | string | `"--logtostderr=true"` |  |
| controllerManager.kubeRbacProxy.args[3] | string | `"--v=0"` |  |
| controllerManager.kubeRbacProxy.containerSecurityContext.allowPrivilegeEscalation | bool | `false` |  |
| controllerManager.kubeRbacProxy.containerSecurityContext.capabilities.drop[0] | string | `"ALL"` |  |
| controllerManager.kubeRbacProxy.image.repository | string | `"gcr.io/kubebuilder/kube-rbac-proxy"` |  |
| controllerManager.kubeRbacProxy.image.tag | string | `"v0.15.0"` |  |
| controllerManager.kubeRbacProxy.resources.limits.cpu | string | `"500m"` |  |
| controllerManager.kubeRbacProxy.resources.limits.memory | string | `"128Mi"` |  |
| controllerManager.kubeRbacProxy.resources.requests.cpu | string | `"5m"` |  |
| controllerManager.kubeRbacProxy.resources.requests.memory | string | `"64Mi"` |  |
| controllerManager.manager.args[0] | string | `"--health-probe-bind-address=:8081"` |  |
| controllerManager.manager.args[1] | string | `"--metrics-bind-address=127.0.0.1:8080"` |  |
| controllerManager.manager.args[2] | string | `"--leader-elect"` |  |
| controllerManager.manager.containerSecurityContext.allowPrivilegeEscalation | bool | `false` |  |
| controllerManager.manager.containerSecurityContext.capabilities.drop[0] | string | `"ALL"` |  |
| controllerManager.manager.image.repository | string | `"controller"` |  |
| controllerManager.manager.image.tag | string | `"latest"` |  |
| controllerManager.manager.resources.limits.cpu | string | `"500m"` |  |
| controllerManager.manager.resources.limits.memory | string | `"128Mi"` |  |
| controllerManager.manager.resources.requests.cpu | string | `"10m"` |  |
| controllerManager.manager.resources.requests.memory | string | `"64Mi"` |  |
| controllerManager.podSecurityContext.runAsNonRoot | bool | `true` |  |
| controllerManager.replicas | int | `1` |  |
| controllerManager.serviceAccount.annotations | object | `{}` |  |
| kubernetesClusterDomain | string | `"cluster.local"` |  |
| metricsService.ports[0].name | string | `"https"` |  |
| metricsService.ports[0].port | int | `8443` |  |
| metricsService.ports[0].protocol | string | `"TCP"` |  |
| metricsService.ports[0].targetPort | string | `"https"` |  |
| metricsService.type | string | `"ClusterIP"` |  |
