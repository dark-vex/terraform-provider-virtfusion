# Server creation goes through a resource-pack discovery flow rather than a
# flat parameter list. Look up resource_pack_id via GET /resourcePack and
# create_id via GET /resourcePack/{resourcePackId} against your account.
resource "virtfusion_server" "node1" {
  resource_pack_id = 5
  create_id        = "5b342c362c382c362c31325d"

  # Only meaningful (and only used) for resource pack options marked
  # "variable" in the discovery response; ignored for fixed-size packs.
  override_memory_mb  = 2048
  override_storage_gb = 40
  override_cpu_cores  = 2
}

# An existing server can also be brought under management without going
# through Create, using its VirtFusion server UUID. resource_pack_id and
# create_id are intentionally Optional+Computed (not Required) so an
# imported server never shows a forced replace on its first plan.
#
# import {
#   to = virtfusion_server.node1
#   id = "385e8f86-6cc8-4f88-8d1e-a8fece0f6b32"
# }
