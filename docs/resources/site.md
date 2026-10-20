---
page_title: "privatus_site Resource"
---

# privatus_site (Resource)

A tracked website. **Destroying it deletes the site and all its data**
(the provider sends the domain as the API's delete confirmation).

## Example Usage

```terraform
resource "privatus_site" "blog" {
  workspace_id = data.privatus_workspace.main.id
  domain       = "blog.example.com"
  name         = "Company blog"
}
```

## Schema

### Required

- `workspace_id` (String) Owning workspace. Forces replacement.
- `domain` (String) Primary domain.

### Optional (computed from the server when omitted)

- `name`, `timezone`, `currency`,
  `extra_domains`, `labels`, `honor_dnt`, `honor_gpc`, `pii_scrubber`,
  `bot_filtering`, `allowed_params`, `retention_months`, `traffic_rule_modes` (map list → `block`/`allow`),
  `public_dashboard`.

### Read-Only

- `id` (String) Site id (`pa_…`), the `data-site` value.
- `snippet` (String) HTML tracking snippet.
- `created_at` (String)

## Import

```shell
terraform import privatus_site.blog pa_ABC123
```
