## **deploy-chart.yml**

This workflow deploys a Helm chart to an EKS cluster.

### Triggers

- **Workflow Run**: Runs when the `Build-Push-Chart` or `Build-Push-Image` workflows complete.
- **Manual Trigger**: Can be manually triggered via `workflow_dispatch`.

### Key Steps

1. **Checkout repo**: Checks out the repository code.
2. **Install Helm**: Installs Helm.
3. **Configure AWS credentials**: Sets up AWS credentials for accessing EKS.
4. **Login to Amazon ECR**: Logs in to Amazon ECR.
5. **Set up kubectl**: Sets up kubectl.
6. **Update kubeconfig**: Updates the kubeconfig for the EKS cluster.
7. **Install Helm chart from OCI registry**: Installs the Helm chart from the OCI registry.

### Secrets Required

- `AWS_ACCOUNT_ID`
- `AWS_ROLE_NAME`
- `AWS_REGION`
- `EKS_CLUSTER_NAME`
- `REGISTRY`
- `REPOSITORY_NAME`
- `PACKER_AMI_ROLE`