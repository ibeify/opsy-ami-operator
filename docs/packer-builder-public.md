## **PackerBuilder**
## Sourcing from Public Repository

```yaml
apiVersion: ami.opsy.dev/v1alpha1
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
  region: "us-west-2"
  builder:
    repoURL: "https://github.com/aws-samples/amazon-eks-custom-amis.git"
    branch: "main"
    image: "hashicorp/packer:latest"
    secret: "aws-creds" # Ensure If secrets not set IRSA is used
    commands:
      - subCommand: "build"
        variablesFile: "al2_amd64.pkrvars.hcl"
        variables:
          eks_version: "1.30"
          region: "us-west-2"
        workingDir: "."
      - subCommand: "init"
        args:
          - "-upgrade"
        workingDir: "."
      - subCommand: "validate"
        args:
        workingDir: "."
```
