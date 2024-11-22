resource "oci_logging_log" "function_invoke_log" {
    display_name = "LicenseExcludedInvoke"
    log_group_id = oci_logging_log_group.function_log_group.id
    log_type = "SERVICE"
    is_enabled = true

    configuration {
      compartment_id = var.compartment_id
      source {
        category = "invoke"
        resource = var.function_application_id
        service = "functions"
        source_type = "OCISERVICE"
      }
    }
}

resource "oci_logging_log_group" "function_log_group" {
  compartment_id = var.compartment_id
  display_name = "LicenseExcludedLogGroup"
}