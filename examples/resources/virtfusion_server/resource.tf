resource "virtfusion_server" "node1" {
  package_id             = 1
  user_id                = 1
  hypervisor_id          = 1
  ipv4                   = 1
  storage                = 30
  memory                 = 1024
  cores                  = 1
  traffic                = 1000
  inbound_network_speed  = 100
  outbound_network_speed = 100
  storage_profile        = 1
  network_profile        = 1
}

# An existing server can also be brought under management without going
# through Create, using its VirtFusion server UUID:
#
# import {
#   to = virtfusion_server.node1
#   id = "385e8f86-6cc8-4f88-8d1e-a8fece0f6b32"
# }
