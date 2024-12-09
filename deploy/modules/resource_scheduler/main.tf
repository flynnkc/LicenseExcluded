resource "oci_resource_scheduler_schedule" "function_schedule" {
    display_name = "LicenseExcludedSchedule"
    action = "START_RESOURCE"
    compartment_id = var.compartment_id
    recurrence_type = "CRON"
    recurrence_details = "15 0 * * *"

    resources {
      id = var.function_id
    }
}