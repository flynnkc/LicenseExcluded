locals {
    image = "iad.ocir.io/ociateam/ociateam-tools/licenseexcluded:0.1.13"
    image_digest = "sha256:8da6e5e23b95db98041f4208d29eb2200583fdad779751afc7ec10a8f7abbde6"
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