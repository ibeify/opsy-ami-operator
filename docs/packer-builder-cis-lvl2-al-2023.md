## Using Ansible-Lock-Down for CIS lvl 2 Hardened AL2023

```yaml
apiVersion: ami.opsy.dev/v1alpha1
kind: PackerBuilder
metadata:
  labels:
  name: packer-builder-cis-lvl2-al-2023
spec:
  amiFilters:
    - name: "name"
      values: ["amazon-eks-node-al2023-x86_64-standard-1.30*"]
    - name: "owner-id"
      values: ["602401143452"]
  clusterName: "opsy-gitops"
  timeOuts:
    expiresIn: "2h"
    controllerTimer: "2m"
  notifier:
    slack:
      channelIDs: ["C055ZJPM2QN"]
      secret: "slack-token"
  gitSync:
    image: "registry.k8s.io/git-sync/git-sync:v4.2.3"
    name: "git-sync"
    secret: "git-sync"
  region: "us-west-2"
  builder:
    repoURL: "https://github.com/ibeify/opsy-ami-operator"
    branch: "main"
    image: "hashicorp/packer:latest"
    dir: "packer/al2023-cis-lvl2"
    commands:
      - subCommand: "build"
        workingDir: "packer/al2023-cis-lvl2"
      - subCommand: "init"
        args:
          - "-upgrade"
        workingDir: "packer/al2023-cis-lvl2"
      - subCommand: "validate"
        args:
        workingDir: "packer/al2023-cis-lvl2"

```
