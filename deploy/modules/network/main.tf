resource "oci_core_vcn" "function_vcn" {
    compartment_id = var.compartment_id
    display_name = "LicenseExcludedVCN"
    cidr_blocks = ["10.0.0.0/24"]
}

resource "oci_core_subnet" "function_subnet" {
    compartment_id = var.compartment_id
    display_name = "LicenseExcludedSubnet"
    cidr_block = "10.0.0.0/26"
    vcn_id = oci_core_vcn.function_vcn.id
    prohibit_internet_ingress = true # Private Subnet
    route_table_id = oci_core_route_table.function_route_table.id
    security_list_ids = [ oci_core_security_list.function_security_list.id ]
}

resource "oci_core_route_table" "function_route_table" {
    compartment_id = var.compartment_id
    vcn_id = oci_core_vcn.function_vcn.id
    display_name = "LicenseExcludedRouteTable"
    depends_on = [ oci_core_nat_gateway.function_nat_gateway, oci_core_service_gateway.function_service_gateway ]
    route_rules {
      network_entity_id = oci_core_service_gateway.function_service_gateway.id
      destination = data.oci_core_services.all_oci_services.services[0].cidr_block
      destination_type = "SERVICE_CIDR_BLOCK"
    }
    route_rules {
      network_entity_id = oci_core_nat_gateway.function_nat_gateway.id
      destination = "0.0.0.0/0"
      destination_type = "CIDR_BLOCK"
    }
}

resource "oci_core_security_list" "function_security_list" {
    compartment_id = var.compartment_id
    display_name = "LicenseExcludedSecurityList"
    vcn_id = oci_core_vcn.function_vcn.id
    egress_security_rules {
      destination = "0.0.0.0/0"
      protocol = "all"
      stateless = false
    }
}

resource "oci_core_service_gateway" "function_service_gateway" {
    compartment_id = var.compartment_id
    display_name = "LicenseExcludedSGW"
    vcn_id = oci_core_vcn.function_vcn.id
    #route_table_id = oci_core_route_table.function_route_table.id
    services {
      service_id = data.oci_core_services.all_oci_services.services[0].id
    }
}

resource "oci_core_nat_gateway" "function_nat_gateway" {
    compartment_id = var.compartment_id
    display_name = "LicenseExcludedNatGW"
    vcn_id = oci_core_vcn.function_vcn.id
    #route_table_id = oci_core_route_table.function_route_table.id
}