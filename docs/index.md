---
page_title: "Privatus Analytics Provider"
description: |-
  Manage Privatus Analytics sites, goals and traffic rules.
---

# Privatus Analytics Provider

Manage [Privatus Analytics](https://privatusanalytics.com) (privacy-first,
cookieless web analytics) with Terraform.

## Example Usage

```terraform
provider "privatus" {
  api_token = var.privatus_api_token # or PRIVATUS_API_TOKEN
}
```

## Schema

### Optional

- `api_token` (String, Sensitive) Personal API token (`pat_…`). Defaults to `PRIVATUS_API_TOKEN`.
- `endpoint` (String) API base URL. Defaults to `PRIVATUS_ENDPOINT`, then `https://privatusanalytics.com`.
