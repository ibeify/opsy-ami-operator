# Project Bootstrap 

```bash 
operator-sdk init --domain opsy.dev --repo github.com/ibeify/opsy-ami-operator
kubebuilder edit --multigroup=true
operator-sdk create api --group ami --version v1alpha1 --kind PackerBuilder --resource --controller

operator-sdk create api --group ami --version v1alpha1 --kind AMIRefresher --resource --controller
```