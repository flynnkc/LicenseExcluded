# https://docs.oracle.com/en-us/iaas/Content/terraform/configuring.htm#api-key-auth
tenancy_ocid     = "ocid1.tenancy.oc1..abcdefghijklmnopqrstuvwxyz"
fingerprint      = "your:key:fingerprint:here"
user_ocid        = "ocid1.user.oc1..abcdefghijklmnopqrstuvwxyz"
private_key_path = "~/.oci/mykey.pem"
private_key_password = "mypassword"
region = "your-home-region" # https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm#About

# Compartment OCID to deploy network & serverless function
compartment_id = "ocid1.compartment.oc1..abcdefghijklmnopqrstuvwxyz"