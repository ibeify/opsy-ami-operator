# Opsy AMI Operator

<div class="grid cards" markdown>
<div class="grid-item" markdown>
<img src="assets/opsy-ami-bot.svg" width="100%" text-align="left"/> 
</div>
 The Opsy AMI Operator is a sophisticated Kubernetes controller designed to streamline and automate the lifecycle management of Amazon Machine Images (AMIs) for AWS-based Kubernetes clusters. This powerful tool seamlessly integrates with both managed Amazon EKS clusters and self-managed Kubernetes environments on AWS.
</div>





!!! abstract "Features and Benefits"

   
    Key features include:

    - **Automated AMI Building**: Leverages HashiCorp Packer to create custom, up-to-date AMIs tailored to your specific requirements.

    - **Instance Refresh Management**: Efficiently updates and refreshes instances within your Kubernetes node groups, ensuring your cluster always runs on the latest AMI.

    - **Lifecycle Automation**: Manages the entire AMI lifecycle, from creation and deployment to retirement, reducing manual intervention and potential human errors.

    - **Kubernetes-Native Approach**: Operates as a native Kubernetes controller, allowing for seamless integration with your existing Kubernetes workflows and GitOps practices.


By automating these critical processes, the Opsy AMI Operator significantly enhances cluster security, reduces operational overhead, and ensures consistency across your Kubernetes environments on AWS.

