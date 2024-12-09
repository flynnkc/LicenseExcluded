provider "oci" {
  tenancy_ocid         = var.tenancy_ocid
  user_ocid            = var.user_ocid
  private_key_path     = var.private_key_path
  fingerprint          = var.fingerprint
  private_key_password = var.private_key_password
  region               = "us-ashburn-1"
}

provider "oci" {
  alias = "home"
  tenancy_ocid         = var.tenancy_ocid
  user_ocid            = var.user_ocid
  private_key_path     = var.private_key_path
  fingerprint          = var.fingerprint
  private_key_password = var.private_key_password
  region               = local.regions_map[local.home_region_key]
}

terraform {

  required_providers {
    oci = {
      source  = "oracle/oci"
      version = ">= 4.80.0"
    }
  }
} 