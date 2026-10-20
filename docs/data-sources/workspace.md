---
page_title: "privatus_workspace Data Source"
---

# privatus_workspace (Data Source)

Looks up a workspace by `id` or `name`. With neither, the token must have
access to exactly one workspace.

## Example Usage

```terraform
data "privatus_workspace" "main" {
  name = "Acme Inc"
}
```

## Schema

### Optional

- `id` (String) Workspace id (`ws_…`).
- `name` (String) Workspace name (case-insensitive exact match).

### Read-Only

- `timezone`, `currency`, `plan`, `plan_events`, `subscription_status`, `role`, `created_at`.
