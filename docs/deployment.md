#  Installation Guide

This guide will walk you through the process of installing the Opsy AMI Operator in your Kubernetes cluster.

## Prerequisites

Before starting the installation, ensure you have the following:

- A running Kubernetes cluster (EKS recommended)
- `kubectl` configured to communicate with your cluster
- Helm v3 installed
- AWS CLI configured with appropriate permissions
- Git installed (for cloning the repository)

## Installation Steps

- **Clone the Repository**

   ```bash
   git clone https://github.com/your-org/opsy-ami-operator.git
   cd opsy-ami-operator
   ```

- **Install Custom Resource Definitions (CRDs)**

   Apply the CRDs to your cluster:

   ```bash
   kubectl apply -k config/crd/
   ```

- **Configure IRSA (IAM Roles for Service Accounts)**

   Create an IAM role for the operator and associate it with a Kubernetes service account:

   ```bash
   export ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
   export OIDC_PROVIDER=$(aws eks describe-cluster --name YOUR_CLUSTER_NAME --query "cluster.identity.oidc.issuer" --output text | sed -e "s/^https:\/\///")

   # Create IAM role
   aws iam create-role --role-name opsy-ami-operator-role --assume-role-policy-document file://trust-relationship.json

   # Attach necessary policies (adjust as needed)
   aws iam attach-role-policy --role-name opsy-ami-operator-role --policy-arn arn:aws:iam::aws:policy/AmazonEC2FullAccess

   # Create Kubernetes service account
   kubectl create serviceaccount opsy-ami-operator -n $NAMESPACE 

   # Annotate the service account with the IAM role ARN
   kubectl annotate serviceaccount opsy-ami-operator -n $NAMESPACE eks.amazonaws.com/role-arn=arn:aws:iam::${ACCOUNT_ID}:role/opsy-ami-operator-role
   ```

- **Configure Helm Values**

   Create a `values.yaml` file or edit the existing one:

   ```yaml
   image:
     repository: your-registry/opsy-ami-operator
     tag: v1.0.0

   serviceAccount:
     create: false
     name: opsy-ami-operator

   config:
     clusterName: YOUR_CLUSTER_NAME
     region: YOUR_AWS_REGION
   ```

- **Install with Helm**

   ```bash
   helm upgrade --install opsy-ami-operator ./charts/opsy-ami-operator \
     --namespace $NAMESPACE \
     --values values.yaml
   ```
- **Create PackerBuilder and AMIRefresher Resources**

   Apply sample resources or create your own:

   ```bash
   kubectl apply -f config/samples/packerbuilder_sample.yaml
   kubectl apply -f config/samples/amirefresher_sample.yaml
   ```

## **Troubleshooting**

- If pods fail to start, check the logs:
  ```bash
  kubectl logs -n  $NAMESPACE deployment/opsy-ami-operator
  ```

- Ensure IAM roles and policies are correctly set up
- Verify that your cluster has the necessary permissions to pull images and create resources
- Ensure you have provided credentials for github access.  

## **Upgrading**

To upgrade the operator, update the image tag in your `values.yaml` and run the Helm upgrade command:

```bash
helm upgrade opsy-ami-operator ./charts/opsy-ami-operator \
  --namespace  $NAMESPACE \
  --values values.yaml
```

## **Uninstallation**

To remove the Opsy AMI Operator:

```bash
helm uninstall opsy-ami-operator -n $NAMESPACE 
kubectl delete -k config/crd/
```