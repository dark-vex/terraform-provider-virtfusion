terraform {
  required_providers {
    virtfusion = {
      source  = "dark-vex/virtfusion"
      version = "1.1.0"
    }
  }
}

# endpoint and api_token are both required; this fork has no default panel
# host, since it targets one specific VirtFusion deployment.
provider "virtfusion" {
  endpoint  = "example.com"
  api_token = var.api_token
}

variable "api_token" {
  type      = string
  sensitive = true
}
