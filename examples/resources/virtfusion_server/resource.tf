# virtfusion_server only supports bringing an existing server under
# management via `terraform import` — Create/Update/Delete are not
# implemented against this fork's real API shape (see CODE-27). All
# attributes besides name/hostname are read-only (Computed).
resource "virtfusion_server" "node1" {}

import {
  to = virtfusion_server.node1
  id = "385e8f86-6cc8-4f88-8d1e-a8fece0f6b32"
}
