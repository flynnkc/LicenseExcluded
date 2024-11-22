# Deploy LicenseExcluded with Terraform

## Requirements

A region subscription to the **Ashburn/us-ashburn-1/iad** region

## Instructions

1. Edit *example.tfvars* with values from your tenancy
1. In *deploy* directory (if using terraform):
    1. `terraform init`
    1. `terraform plan -var-file example.tfvars -out tf.plan`
    1. `terraform apply "tf.plan"`

## End State

Resources display name *LicenseExcluded\<Resource>*

- Function Application
- Function
- Log Group
- Function Invoke Log
- Dynamic Group
- Resource Schedule
- Required IAM policies
- Virtual Cloud Network
  - Private Subnet
  - NAT Gateway
  - Service Gateway
  - Route Table
  - Security List
