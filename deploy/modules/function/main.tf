locals {
    image = "iad.ocir.io/ociateam/ociateam-tools/licenseexcluded:0.1.1"
    image_digest = "sha256:ab0583139f1a48af0bb3e3ef56c929af4000468cb00e80825e149ad3b65246f3"
}

resource "oci_functions_application" "function_application" {
    compartment_id = var.compartment_id
    display_name = "LicenseExcludedApplication"
    subnet_ids = var.subnet_ids
}

resource "oci_functions_function" "function" {
    application_id = oci_functions_application.function_application.id
    display_name = "LicenseExcluded"
    memory_in_mbs = 256
    image = local.image
    image_digest = local.image_digest
}