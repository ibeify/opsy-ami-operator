# PackerBuilder

The PackerBuilder is a Custom Resource Definition (CRD) that automates the process of building Amazon Machine Images (AMIs) using HashiCorp Packer within a Kubernetes environment.

## **Overview**

The packer-build-controller manages PackerBuilder resources, creating Kubernetes Jobs to run Packer builds. It only initiates a build when it detects that an AMI with the appropriate tags doesn't exist.

## **Key Components**

### **AMI Tagging**

The controller uses specific tags to manage AMIs:

- Creation Timestamp: Indicates when the AMI was created
- Status: Set to "active" for usable AMIs
- Build ID: Links the AMI to its PackerBuilder instance
- Management Tags: Identify AMIs managed by this operator

These tags prevent unnecessary rebuilds and maintain AMI lifecycle management.

### **Base AMI Discovery**

- Uses provided filters to find a base AMI
- Alternatively, you can run self-contained Packer projects without filters

###  **Packer Configuration**

- Sourced from a Git repository (public or private)
- Uses [git-sync](https://github.com/kubernetes/git-sync) to pull the repository

### **Build Process**

- Can Adopt Existing AMI which meet all criteria
- Can discover running jobs in case of interrupted service.
- Configurable reconciliation schedule and AMI expiration

###  **Flexibility**

- PackerBuilder can be used independently of the AMI refresh controller. PackerBuilder has on task and one task only, AMI production.

## **PackerBuilder Specification**

```yaml
apiVersion: ami.refresh.ops/v1alpha1
kind: PackerBuilder
metadata:
  name: packer-builder-local
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
  notifier:
    slack:
      channelIDs: ["SLACK_CHANNEL_IDS"]
      secret: "slack-token"
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

## **Status Reporting**

The PackerBuilder resource maintains detailed status information:

```yaml
status:
  buildID: 728d7a64-ca85-42c9-9acd-60c681d4efe3
  command: packer init -upgrade . && packer validate . && packer build -var 'eks_version=1.30' -var 'region=us-west-2' -var-file=al2_amd64.pkrvars.hcl -color=false .
  conditions:
    - lastTransitionTime: "2024-09-01T18:05:40Z"
      message: Latest Created AMI ami-0d2587d8e405f376c has not expired and is active
      reason: FlightCheckFailed
      status: "True"
      type: FlightCheckFailed
  failedJobCount: 0
  jobID: 3cb1b
  jobName: packer-builder-remote-3cb1b
  jobStatus: completed
  lastRun: "2024-09-01T18:30:48Z"
  lastRunBaseImageID: ami-05da30e98c637628c
  lastRunBuiltImageID: ami-0d2587d8e405f376c
  lastRunMessage: Reconciliation in progress
  lastRunStatus: running
```

## **Packer Command Execution**

Default command if none specified:
```bash
packer init . && packer validate . && packer build -color=false .
```

The controller always executes in this order: `init`, `validate`, `build`.

## **Notifications**

Supports state transition notifications:

- Slack (implemented)
- SNS, SES, Discord (planned)

## **Image Adoption**

The controller can adopt existing AMIs that meet specific criteria:

- Has an "active" status tag
- Is not expired
- Contains required management tags





Required tags for adoption:
```bash
"brought-to-you-by":  "opsy-the-ami-operator"
"cluster-name":       "opsy-eks"
"created-by":         "name-of-your-crd-instance"
"packer-repo":        "repo-where-packer-code-was-sourced"
"packer-repo-branch": "branchable-branch"
```


## Prerequisites

- **Kubernetes Cluster**: A functional Kubernetes cluster to run the controller and its jobs.

- **IRSA Role for Packer**: An IAM Role for Service Accounts (IRSA) configured for Packer jobs.
    - [See IAM Policy details](https://developer.hashicorp.com/packer/integrations/hashicorp/amazon#iam-task-or-instance-role)

    - Example ServiceAccount configuration:
      ```yaml
      apiVersion: v1
      kind: ServiceAccount
      metadata:
        name: eks-ami-builder
        namespace: default
        annotations:
          eks.amazonaws.com/role-arn: arn:aws:iam::012345678910:role/eks-packer-builder
      ```

- **Git Repository Access**: The project uses [git-sync](https://github.com/kubernetes/git-sync) to pull repositories for Packer jobs.

    - Example Secret for GitHub token:
      ```yaml
      apiVersion: v1
      kind: Secret
      metadata:
        name: git-credentials
        namespace: default  # Adjust namespace as needed
      type: Opaque
      data:
        username: c2VjcmLTQ1Nzg5=
        token: Y2hwZ0kxRDU5Q0VrVUtPZ0hzTEdjemFqeTM1UEJhYg==
      ```
      For more details, refer to the [git-sync manual](https://github.com/kubernetes/git-sync)


-  **Packer Configuration**: Set `ami_id` and `region` as variables if you plan to use filters for finding the Base AMI.

        Example:
        ```hcl
        variable "ami_id" {
            type    = string
            default = "ami-0c2b8ca1dad447f8a"
        }

        variable "region" {
            type    = string
            default = "us-east-1"
        }
        ```
