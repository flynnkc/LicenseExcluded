resource "oci_identity_policy" "function_policy" {
    compartment_id = var.compartment_id
    description = "For LicenseExcluded scheduled function"
    name = "LicenseExcludedPolicy"
    statements = [
        "Allow dynamic-group ${oci_identity_dynamic_group.function_dynamic_group.name} to use autonomous-databases in tenancy",
        "Allow dynamic-group ${oci_identity_dynamic_group.function_dynamic_group.name} to manage db-systems in tenancy",
        "Allow dynamic-group ${oci_identity_dynamic_group.function_dynamic_group.name} to manage analytics-instance in tenancy where all {request.permission != 'ANALYTICS_INSTANCE_CREATE', request.permission != 'ANALYTICS_INSTANCE_DELETE', request.permission != 'ANALYTICS_INSTANCE_MOVE'}",
        "Allow any-user to manage functions-family in tenancy where all {request.principal.type='resourceschedule', request.principal.id='${var.schedule_id}'}",
    ]
}

resource "oci_identity_dynamic_group" "function_dynamic_group" {
    compartment_id = var.compartment_id
    description = "Dynamic Group for LicenseExcluded function"
    name = "LicenseExcludedDG"
    matching_rule = "All {resource.type = 'fnfunc', resource.id = '${var.function_id}'}"
}