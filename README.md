# VirtFusion Terraform Provider

![Terraform](https://img.shields.io/badge/Terraform-%235835CC.svg?style=for-the-badge&logo=terraform&logoColor=white)
![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/dark-vex/terraform-provider-virtfusion)

## Overview

This is a fork of [snowsidejon/terraform-provider-virtfusion](https://github.com/snowsidejon/terraform-provider-virtfusion),
rewritten against VirtFusion v7.0.x API.

- 🔑 Environment variable support for easy automation
- 🧩 Fork of the community provider on the [Terraform Registry](https://registry.terraform.io/providers/snowsidejon/virtfusion/latest)

---

## Installation

Build from source and use a [`dev_overrides`](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers)
block, or reference this fork's own release once published.

```hcl
terraform {
  required_providers {
    virtfusion = {
      source  = "dark-vex/virtfusion"
      version = "1.1.0"
    }
  }
}
```

---

## Provider Configuration

Both `endpoint` and `api_token` are **required** — this fork has no default
panel host, and there are no other provider-level defaults (an earlier
version of this fork carried `os_template`/`resource_package`/`public_ips`/
`private_ips`/`hypervisor_group`, inherited from the old guessed Create
payload; none of those concepts exist in the real API, so they were removed
rather than kept as dead configuration).

```hcl
provider "virtfusion" {
  endpoint  = "example.com"
  api_token = var.api_token
}
```

| Attribute   | Env Var                | Default            |
|-------------|-------------------------|--------------------|
| `endpoint`  | `VIRTFUSION_ENDPOINT`   | _none (required)_  |
| `api_token` | `VIRTFUSION_API_TOKEN`  | _none (required)_  |

---

## Resources

### `virtfusion_server`

Server creation goes through VirtFusion's resource-pack discovery flow, not
a flat parameter list:

```hcl
resource "virtfusion_server" "node1" {
  resource_pack_id = 5
  create_id        = "5b342c362c382c362c31325d"

  # Only meaningful for resource pack options marked "variable" in the
  # discovery response; ignored for fixed-size packs.
  override_memory_mb  = 2048
  override_storage_gb = 40
  override_cpu_cores  = 2
}
```

Look up `resource_pack_id` via `GET /resourcePack` and `create_id` via
`GET /resourcePack/{resourcePackId}` (its response is a nested
DC → plan → size tree; `create_id` is an opaque string on the leaf options,
not something you can guess). `resource_pack_id`/`create_id`/the
`override_*` attributes are `Optional+Computed` rather than `Required` —
deliberately, so that an **imported** server never carries a value Read
can't give back and never shows a forced replace on its first plan.
`Create` still validates `resource_pack_id`/`create_id` are set before
doing anything.

Almost everything else about a server has no update endpoint at all on the
real API and is `RequiresReplace`. The few things that are genuinely
updatable in place go through their own narrow endpoints: `name` (rename),
`boot_type`/`auto_configuration` (boot settings), `boot_order`. Deletion
goes through `DELETE /resourcePack/{serverId}` — per the API's own
documentation, this "will only work for servers created as part of a
resource pack," so a server that predates resource-pack provisioning (e.g.
one brought in via `terraform import`) may not be deletable through this
endpoint at all.

An existing server can be brought under management without going through
Create:

```hcl
resource "virtfusion_server" "example" {}
```

```bash
terraform import virtfusion_server.example 385e8f86-6cc8-4f88-8d1e-a8fece0f6b32
terraform plan   # should show 0 changes
```

### `virtfusion_build`

Triggers a server build/rebuild (`POST /server/{serverId}/build`). This is
a stateless action on the real API, not a persistent object — there's no
GET-by-id, update, or delete for a "build." Every attribute is
`RequiresReplace` (changing any of them re-runs the, destructive, build),
`id` is the target server's own UUID (build has no identity of its own),
and `Delete` only drops the resource from Terraform state — there's no
"unbuild" endpoint to call.

```hcl
resource "virtfusion_build" "node1" {
  server_id   = virtfusion_server.node1.id
  method      = "template"
  template_id = 21 # see GET /server/{serverId}/operatingSystemTemplates
  hostname    = "node1.example.com"
  ssh_keys    = [virtfusion_ssh.dummy_key.id]
}
```

### `virtfusion_ssh`

Account-scoped SSH keys — there's no `user_id` field on the real API (the
account comes from the bearer token) and no update endpoint at all, so
`name`/`public_key` are `RequiresReplace`.

```hcl
resource "virtfusion_ssh" "dummy_key" {
  name       = "dummy_key"
  public_key = "ssh-ed25519 AAAA..."
}
```

---

## Contributing

This fork is maintained for one specific deployment. Issues and PRs against
`dark-vex/terraform-provider-virtfusion` are welcome, but generic VirtFusion
compatibility is out of scope here — see the upstream project for that.

---

## License

MPL-2.0 — see [LICENSE](LICENSE).
