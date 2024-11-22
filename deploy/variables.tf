variable "tenancy_ocid" {
  type = string
}

variable "user_ocid" {
  type = string
}

variable "private_key_path" {
  type = string
}

variable "fingerprint" {
  type = string
}

variable "private_key_password" {
  type    = string
  default = ""
}

variable "compartment_id" {
  type = string
}

variable "home_region" {
  type = string
}
