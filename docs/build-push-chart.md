

## **build-push-chart.yml**

This workflow builds and pushes Helm charts to Amazon ECR.

## **Triggers**

- **Manual Trigger**: Can be manually triggered via `workflow_dispatch`.
- **Pull Request**: Runs when a pull request is closed and merged into `main`, `default`, or `develop` branches.
- **Push**: Runs on pushes to the `develop` branch.

## **Key Steps**

1. **Checkout repo**: Checks out the repository code.
2. **Install Helm**: Installs Helm.
3. **Configure AWS credentials**: Sets up AWS credentials for accessing ECR.
4. **Login to Amazon ECR**: Logs in to Amazon ECR provided repo.
5. **Build, tag, and push Helm chart**: Lints, packages, and pushes the Helm chart to Amazon ECR.


## **Secrets Required**
- `AWS_ACCOUNT_ID`
- `AWS_ROLE_NAME`
- `AWS_REGION`
- `Name` (Helm chart name)


