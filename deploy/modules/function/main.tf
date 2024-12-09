locals {
    image = "iad.ocir.io/ociateam/ociateam-tools/licenseexcluded:0.1.14"
    image_digest = "sha256:5676f95d285b84ff605cf8946ef163323da3e04c98079c9f5039790f2dc4aad6"
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