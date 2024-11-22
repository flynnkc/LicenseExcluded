# License Excluded

This project is intended to manage licenses in [Oracle Cloud Infrastructure](https://www.oracle.com/cloud/) (OCI). Licenses can be either included as a part of the cost of OCI services, or existing licenses can be used with the "Bring Your Own License" option. If you have enough licenses available, there is no good reason to select "License Included", but it's a common mistake.

This project switches licensing on select services from License Included to BYOL.

![Business people making it rain money](./images/pexels-pavel-danilyuk.jpg)
[Photo by Pavel Danilyuk](https://www.pexels.com/photo/paper-money-falling-around-the-people-in-business-attire-in-the-office-7654628/)

## Resource Types Supported

- Autonomous Database
- Oracle Base Database
- Analytics Cloud Instances

## Architecture

This script is meant to be run in an [OCI Function](https://docs.oracle.com/en-us/iaas/Content/Functions/Concepts/functionsoverview.htm). Funcions are serverless tools that are useful for short jobs. In addition, it prevents needing to store long-lived credentials as the Function will be able to dynamically retrive access tokens when needed.

### Terraform Deployment

See the [README](deploy/README.md) in *deploy* to create the stack with [Terraform](https://www.terraform.io/)/[OpenTofu](https://opentofu.org/).

### Manual Deployment

1. Set up Fn development environment
1. Create Function Application with network access
1. Deploy Function from *src* directory to Function Application
1. Create a Dynamic Group with the function as a member
1. Write Policies to allow the Function to interact with services
    - "Allow dynamic-group ${oci_identity_dynamic_group} to use autonomous-databases in tenancy"
    - "Allow dynamic-group ${oci_identity_dynamic_group} to manage db-systems in tenancy"
    - "Allow dynamic-group ${oci_identity_dynamic_group} to manage analytics-instance in tenancy where all {request.permission != 'ANALYTICS_INSTANCE_CREATE', request.permission != 'ANALYTICS_INSTANCE_DELETE', request.permission != 'ANALYTICS_INSTANCE_MOVE'}"
1. Set up Resource Scheduler to invoke function on a schedule
1. Write policy allowing Resource Scheduler to invoke Function
