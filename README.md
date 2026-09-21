# VirtFusion Terraform Provider

![Terraform](https://img.shields.io/badge/Terraform-%235835CC.svg?style=for-the-badge&logo=terraform&logoColor=white)
![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/snowsidejon/terraform-provider-virtfusion)

## Overview

This is a fork of [snowsidejon/terraform-provider-virtfusion](https://github.com/snowsidejon/terraform-provider-virtfusion),
patched to fix request handling that meant this provider had never
successfully issued a request against any real VirtFusion API (see below),
and to add `terraform import` support for `virtfusion_server`. It is **not**
intended to be upstreamed — the fixes here are specific to one deployment's
actual API shape, not generic VirtFusion behavior.

All three resources (`virtfusion_server`, `virtfusion_build`,
`virtfusion_ssh`) keep their original Create/Update/Delete behavior — same
attributes, same request payloads — now with the URL/auth bug fixed, so
those requests can actually reach the API for the first time.
`virtfusion_server` additionally supports `terraform import`, so an existing
server can be brought under management without going through Create.

**Import caveat:** `virtfusion_server`'s schema is still the original
create-time schema (`user_id`, `package_id`, `storage`, `memory`, `cores`,
etc.) — `Read` does not populate these from the real API after an import,
because the real API doesn't return them in that shape. That means a
`terraform plan` right after import will typically show a diff for any
`Required` attribute (e.g. `user_id`) that isn't already set to match reality
in your config, and if you `apply` that plan, `Update` will send a real
request to the live server with those values. Review any post-import plan
carefully — don't `apply` on an imported resource until you're sure the
config matches the real server, since Update/Delete requests are no longer
silently broken.

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
      source  = "snowsidejon/virtfusion"
      version = "1.1.0"
    }
  }
}
```

---

## Provider Configuration

The provider can be configured using HCL attributes or environment variables.
Both `endpoint` and `api_token` are **required** — this fork has no default
panel host.

### Attributes
```hcl
provider "virtfusion" {
  endpoint         = "example.com"
  api_token        = var.api_token
  os_template      = "Ubuntu Server 22.04"
  resource_package = 11
  public_ips       = 1
  private_ips      = 0
  hypervisor_group = 14
}
```

### Environment variables
| Attribute          | Env Var                       | Default                 |
|--------------------|--------------------------------|--------------------------|
| `endpoint`         | `VIRTFUSION_ENDPOINT`         | _none (required)_       |
| `api_token`        | `VIRTFUSION_API_TOKEN`        | _none (required)_       |
| `os_template`      | `VIRTFUSION_OS_TEMPLATE`      | `Ubuntu Server 22.04`   |
| `resource_package` | `VIRTFUSION_RESOURCE_PACKAGE` | n/a                      |
| `public_ips`       | `VIRTFUSION_PUBLIC_IPS`       | `1`                      |
| `private_ips`      | `VIRTFUSION_PRIVATE_IPS`      | `0`                      |
| `hypervisor_group` | `VIRTFUSION_HYPERVISOR_GROUP` | n/a                      |

---

## Example: Basic

```hcl
provider "virtfusion" {}

resource "virtfusion_server" "demo" {
  user_id = 1
}

resource "virtfusion_build" "demo" {
  server_id = virtfusion_server.demo.id
  name      = "tf-basic"
  hostname  = "tf-basic.example.com"
}
```

Just set your API token and endpoint:

```bash
export VIRTFUSION_API_TOKEN="your_api_token"
export VIRTFUSION_ENDPOINT="example.com"
terraform init
terraform apply
```

---

## Example: Advanced

```hcl
provider "virtfusion" {
  api_token        = var.api_token
  os_template      = "Debian 12"
  resource_package = 15
  public_ips       = 2
  private_ips      = 1
  hypervisor_group = 14
}

variable "api_token" {
  type      = string
  sensitive = true
}

resource "virtfusion_ssh" "my_key" {
  user_id    = 1
  name       = "terraform-key"
  public_key = "ssh-ed25519 AAAAC3NzExampleKeyGeneratedLocally"
}

resource "virtfusion_server" "vm" {
  user_id = 1
}

resource "virtfusion_build" "vm" {
  server_id = virtfusion_server.vm.id
  name      = "adv-vm"
  hostname  = "adv.example.com"
  ssh_keys  = [virtfusion_ssh.my_key.id]
  vnc       = true
  ipv6      = true
  email     = true
}
```

---

## Example: Importing an existing server

```hcl
provider "virtfusion" {
  endpoint  = "example.com"
  api_token = var.api_token
}

resource "virtfusion_server" "example" {
  user_id = 1 # set to match the real server so plan doesn't propose a change
}

import {
  to = virtfusion_server.example
  id = "385e8f86-6cc8-4f88-8d1e-a8fece0f6b32" # server UUID
}
```

```bash
export VIRTFUSION_API_TOKEN="your_api_token"
terraform init
terraform plan   # review carefully before ever applying against a real server
```

---

## Resources

- `virtfusion_server` → Create, read, update, delete, and import a VM
- `virtfusion_build` → Provision and configure servers
- `virtfusion_ssh` → Manage SSH keys

---

## Contributing

This fork is maintained for one specific deployment. Issues and PRs against
`dark-vex/terraform-provider-virtfusion` are welcome, but generic VirtFusion
compatibility is out of scope here — see the upstream project for that.

---

## License

MPL-2.0 — see [LICENSE](LICENSE).
