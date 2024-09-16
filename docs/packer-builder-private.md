## Sourcing from a Private Repository
```yaml
apiVersion: ami.refresh.ops/v1alpha1
kind: PackerBuilder
metadata:
  labels:
  name: packer-builder-private
spec:
  amiFilters:
    - name: "name"
      values: ["amazon-eks-node-al2023-x86_64-standard-1.30*"]
    - name: "owner-id"
      values: ["602401143452"]
  clusterName: "opsy-gitops"
  timeOuts:
    expiresIn: "48h"
    controllerTimer: "5m"
  gitSync:
    image: "registry.k8s.io/git-sync/git-sync:v4.2.3"
    name: "git-sync"
    secret: "git-sync"
  region: "us-west-2"
  builder:
    repoURL: "https://github.com/ibeify/eks-node-group-ami-refresh"
    branch: "main"
    image: "hashicorp/packer:latest"
    dir: "packer/generic-fips"
    secret: "aws-creds"
    commands:
      - subCommand: "build"
        args:
          - "-force"
        color: false
        debug: true
        onError: "cleanup"
```