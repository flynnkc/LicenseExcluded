# Deploy LicenseExcluded with Terraform

## Requirements

A region subscription to the **Ashburn/us-ashburn-1/iad** region

## Instructions

### Terraform CLI

1. Edit *example.tfvars* with values from your tenancy
1. In *deploy* directory (if using terraform):
    1. `terraform init`
    1. `terraform plan -var-file example.tfvars -out tf.plan`
    1. `terraform apply "tf.plan"`

### Resource Manager

1. Upload **deploy** directory to Resource Manager and select the appropriate compartment.
1. *Plan* and *Apply*

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
