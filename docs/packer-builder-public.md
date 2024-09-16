## **PackerBuilder**
## Sourcing from Public Repository

```yaml
apiVersion: ami.refresh.ops/v1alpha1
kind: PackerBuilder
metadata:
  labels:
  name: packer-builder-remote
spec:
  clusterName: "opsy-gitops"
  timeOuts:
    expiresIn: "1h"
    controllerTimer: "5m"
  gitSync:
    image: "registry.k8s.io/git-sync/git-sync:v4.2.3"
    name: "git-sync"
    secret: "git-sync"
  region: "us-west-2"
  builder:
    repoURL:  "https://github.com/aws-samples/amazon-eks-custom-amis.git"
    branch: "main"
    image: "hashicorp/packer:latest"
    dir: "packer"
    secret: "aws-creds"
    commands:
    - subCommand: "build"
      variablesFile: "al2_amd64.pkrvars.hcl"
      variables:
        eks_version: "1.30"
        region: "us-west-2"
        # subnet_id: "subnet-01abc23"
      workingDir: "."
    - subCommand: "init"
      args:
        - "-upgrade"
      workingDir: "."
    - subCommand: "validate"
      args:
      workingDir: "."
```