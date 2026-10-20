---
page_title: "privatus_traffic_rule Resource"
---

# privatus_traffic_rule (Resource)

One entry in a site's block/allow lists, checked at ingest. Set whether a
list blocks or allows with `privatus_site.traffic_rule_modes`.

> Uses `/sites/{site_id}/traffic_rules(.json)` and
> `/sites/{site_id}/traffic_rules/{id}.json` (not yet in the published
> OpenAPI document). There is no single-rule GET, so reads scan the list.

## Example Usage

```terraform
resource "privatus_traffic_rule" "admin" {
  site_id = privatus_site.blog.id
  list    = "paths"
  pattern = "/wp-admin/*"
}
```

## Schema

### Required

- `site_id` (String) Forces replacement.
- `list` (String) `hostnames`, `paths`, `referrers`, `countries`, `ip_ranges`, `user_agents` or `events`. Forces replacement.
- `pattern` (String) `*` wildcards; IP/CIDR for `ip_ranges`; ISO code for `countries`.

### Optional

- `label` (String)

### Read-Only

- `id` (String) Numeric rule id, as a string.
- `mode` (String) `block` or `allow`.

## Import

```shell
terraform import privatus_traffic_rule.admin pa_ABC123/42
```
