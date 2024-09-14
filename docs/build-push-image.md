## **build-push-image.yml**

This workflow builds and pushes Docker images to Amazon ECR.

### Triggers

- **Manual Trigger**: Can be manually triggered via `workflow_dispatch`.
- **Pull Request**: Runs when a pull request is closed and merged into `main`, `default`, or `develop` branches.
- **Push**: Runs on pushes to the `develop` branch.

### Key Steps

1. **Checkout repo**: Checks out the repository code.
2. **Configure AWS credentials**: Sets up AWS credentials for accessing ECR.
3. **Login to Amazon ECR**: Logs in to Amazon ECR.
4. **Get build date**: Retrieves the current build date.
5. **Build, tag, and push Docker image**: Builds, tags, and pushes the Docker image to Amazon ECR.

### Secrets Required

- `AWS_ACCOUNT_ID`
- `AWS_ROLE_NAME`
- `AWS_REGION`
- `REPOSITORY_NAME`
