# VirtFusion Terraform Provider

![Terraform](https://img.shields.io/badge/Terraform-%235835CC.svg?style=for-the-badge&logo=terraform&logoColor=white)
![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/snowsidejon/terraform-provider-virtfusion)

## Overview

This is a fork of [snowsidejon/terraform-provider-virtfusion](https://github.com/snowsidejon/terraform-provider-virtfusion),
patched to work against one specific real VirtFusion deployment
(`vps.hostbrr.com`). It is **not** intended to be upstreamed — the fixes here
are specific to that deployment's actual API shape, not generic VirtFusion
behavior.

`virtfusion_server` only supports bringing an existing server under
management via `terraform import`. Create/Update/Delete are deliberately not
implemented for any resource in this fork — the real mutating request/response
shapes are unconfirmed, and guessing them risks firing an unverified request
at a live production server. Every attribute besides `name`/`hostname` on
`virtfusion_server` is Computed (read-only), so an import followed by
`terraform plan` should show zero changes.

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
  endpoint  = "vps.hostbrr.com"
  api_token = var.api_token
}
```

### Environment variables
| Attribute   | Env Var                | Default            |
|-------------|-------------------------|--------------------|
| `endpoint`  | `VIRTFUSION_ENDPOINT`   | _none (required)_  |
| `api_token` | `VIRTFUSION_API_TOKEN`  | _none (required)_  |

---

## Example: Importing an existing server

```hcl
provider "virtfusion" {
  endpoint  = "vps.hostbrr.com"
  api_token = var.api_token
}

resource "virtfusion_server" "example" {}

import {
  to = virtfusion_server.example
  id = "385e8f86-6cc8-4f88-8d1e-a8fece0f6b32" # server UUID
}
```

```bash
export VIRTFUSION_API_TOKEN="your_api_token"
terraform init
terraform plan   # should show 0 changes once state matches the import
```

---

## Resources

- `virtfusion_server` → Import and read an existing server (Create/Update/Delete unverified — not implemented)
- `virtfusion_build` → Schema only; endpoint existence unconfirmed against this deployment (not implemented)
- `virtfusion_ssh` → Schema only; endpoint existence unconfirmed against this deployment (not implemented)

---

## Contributing

This fork is maintained for one specific deployment. Issues and PRs against
`dark-vex/terraform-provider-virtfusion` are welcome, but generic VirtFusion
compatibility is out of scope here — see the upstream project for that.

---

## License

MPL-2.0 — see [LICENSE](LICENSE).
