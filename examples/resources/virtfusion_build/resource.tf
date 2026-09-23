# virtfusion_build triggers a server build/rebuild action (POST
# /server/{serverId}/build). It is not a persistent object on the real API
# — every attribute is RequiresReplace, and changing any of them re-runs
# the (destructive) build.
resource "virtfusion_build" "node1" {
  server_id   = virtfusion_server.node1.id
  method      = "template"
  template_id = 21 # see GET /server/{serverId}/operatingSystemTemplates
  hostname    = "node1.example.com"
  timezone    = "America/Los_Angeles"
  ipv6        = true
  ssh_keys    = [virtfusion_ssh.dummy_key.id]
}
