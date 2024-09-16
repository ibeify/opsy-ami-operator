#  Installation Guide

This guide will walk you through the process of installing the Opsy AMI Operator in your Kubernetes cluster.

## Prerequisites

Before starting the installation, ensure you have the following:

- A running Kubernetes cluster (EKS recommended)
- `kubectl` configured to communicate with your cluster
- Helm v3 installed
- AWS CLI configured with appropriate permissions
- Git installed (for cloning the repository)

## Installation

- **Configure IRSA for controller (IAM Roles for Service Accounts)**

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
    `NOTE`: You can use hardcoded AWS creds, by passing them in as env variables.Just supply secrets name in `PackerBuilder.Spec.Builder.Secrets`

- **Build and push your image to the location specified by `IMG`:**

    ```sh
    make docker-build docker-push IMG=<some-registry>/opsy-ami-operator:tag
    ```
  **NOTE:** This image ought to be published in the personal registry you specified.
  And it is required to have access to pull the image from the working environment.
  Make sure you have the proper permission to the registry if the above commands don’t work.


- **Install with Helm**
  ```bash
    ❯ helm repo add opsy-ami-operator https://ibeify.github.io/opsy-ami-operator
    ❯ helm repo update
    ❯ helm upgrade --install opsy-ami opsy-ami-operator/opsy-ami-operator --version 1.7.0 \
      --namespace opsy --create-namespace \
      --set controllerManager.manager.image.repository=<repository> \
      --set controllerManager.manager.image.tag=<version> \
      --set-json 'controllerManager.serviceAccount.annotations={"eks.amazonaws.com/role-arn": "arn:aws:iam::012345678910:role/eks-packer-builder"}'

  ```


- **Configure IRSA Role for the packer jobs**.
  [See IAM Policy details ](https://developer.hashicorp.com/packer/integrations/hashicorp/amazon#iam-task-or-instance-role)
  ```yaml
    apiVersion: v1
    kind: ServiceAccount
    metadata:
    name: eks-ami-builder
    namespace: default
    annotations:
        eks.amazonaws.com/role-arn: arn:aws:iam::012345678910:role/eks-packer-builder
  ```

- **Configure GitSync Secrets**

  This project uses [git-sync](https://github.com/kubernetes/git-sync) to pull repos for the packer job. You'll need to provide credentials to access your private repos.Below is a simple example for github tokens. More on this can be found in the [manual](https://github.com/kubernetes/git-sync)
  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: git-credentials
  type: Opaque
  data:
    username: $USERNAME
    token: $TOKEN
  ```
- **Create PackerBuilder and AMIRefresher Resources**
  ```bash
  # Example
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
❯ helm upgrade --install opsy-ami opsy-ami-operator/opsy-ami-operator --version 1.7.0 \
  --namespace opsy --create-namespace \
  --set controllerManager.manager.image.repository=<repository> \
  --set controllerManager.manager.image.tag=<version> \
  --set-json 'controllerManager.serviceAccount.annotations={"eks.amazonaws.com/role-arn": "arn:aws:iam::012345678910:role/eks-packer-builder"}'
```

## **Uninstallation**

To remove the Opsy AMI Operator:

```bash
helm uninstall opsy-ami-operator -n $NAMESPACE
kubectl delete crd amirefreshers.ami.opsy.dev
kubetcl delete crd packerbuilders.ami.opsy.dev
```
