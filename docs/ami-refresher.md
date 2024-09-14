# AMIRefresher

The AMIRefresher is a Custom Resource Definition (CRD) that automates the process of updating and refreshing Amazon EKS node groups with the latest Amazon Machine Images (AMIs).

## **Overview**

The AMIRefresher controller automates the AMI update process for Amazon EKS node groups. It manages launch template updates and triggers instance refreshes, ensuring your Kubernetes cluster consistently runs on the latest, most secure Amazon Machine Images (AMIs).


## **Key Components**

###  AMI Selection

- Can use a specific AMI ID or discover AMIs using filters
- Managed AMIs are tagged with metadata like creation timestamp and base AMI ID
- Only non-expired, active AMIs are eligible for deployment

### AMI Validation

- Checks for the `status: active` tag
- Verifies the AMI has not expired
- Ensures only one valid AMI per AMIRefresher instance

###  Node Group Management

- Updates launch templates for node groups in the cluster
- Initiates instance refresh processes
- Allows exclusion of specific node groups

## **AMIRefresher Specification**

```yaml
apiVersion: ami.refresh.ops/v1alpha1
kind: AMIRefresher
metadata:
  name: node-group-ami-refresh
  labels:
    app.kubernetes.io/name: amirefresher
    app.kubernetes.io/instance: amirefresher-sample
    app.kubernetes.io/part-of: opsy-ami-operator
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/created-by: opsy-ami-operator
spec:
  region: "us-west-2"
  amiFilters:
    - name: "name"
      values: ["amazon-eks-node-al2023-x86_64-standard-1.30*"]
    - name: "owner-id"
      values: ["602401143452"]
  clusterName: "ibeify-gitops"
  expiresIn: "5m" # 1h, 1d, 1w, 1m, 3m, 6m, 1y
  exclude:
    - "ingress-dev"
```

## **Key Fields Explained**

- `region`: AWS region where the EKS cluster is located
- `amiFilters`: Criteria for selecting the appropriate AMI
  - Can filter by AMI name, owner ID, or other attributes
- `clusterName`: Name of the EKS cluster to manage
- `expiresIn`: Time duration after which the AMI is considered expired
  - Supports various time units (e.g., minutes, hours, days, weeks, months, years)
- `exclude`: List of node groups to exclude from the refresh process

## **AMI Selection Process**

- The controller first checks if a specific AMI ID is provided in the spec
- If not, it uses the provided filters to discover a suitable AMI
- Validates the AMI:
   - Checks for the `status: active` tag
   - Ensures the AMI has not expired based on the `expiresIn` setting
- If a valid AMI is found, it's used to update the launch templates

## **Node Group Refresh Process**

- Identifies all node groups in the specified cluster
- Excludes any node groups listed in the `exclude` field
- Updates the launch template for each eligible node group with the new AMI ID
- Initiates an instance refresh process for each updated node group
