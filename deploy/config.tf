module "network" {
  source         = "./modules/network"
  compartment_id = var.compartment_id
}

module "function" {
  source = "./modules/function"
  compartment_id = var.compartment_id
  subnet_ids = [ module.network.subnet_id ]
}

module "resource_scheduler" {
  source = "./modules/resource_scheduler"
  function_id = module.function.function_id
  compartment_id = var.tenancy_ocid
}

module "iam" {
  source = "./modules/iam"
  compartment_id = var.tenancy_ocid
  schedule_id =module.resource_scheduler.schedule_id
  function_id = module.function.function_id
  providers = {
    oci = oci.home
  }
}