---
page_title: "privatus_goal Resource"
---

# privatus_goal (Resource)

A conversion goal. Goals are query-time definitions, so they apply to
historical data as soon as they exist.

> Uses `/sites/{site_id}/goals(.json)` and `/sites/{site_id}/goals/{id}.json`
> (plus `…/archive` and `…/unarchive`), which are not yet in the published
> OpenAPI document.

## Example Usage

```terraform
resource "privatus_goal" "purchase" {
  site_id    = privatus_site.blog.id
  name       = "Purchase"
  kind       = "event"
  event_name = "Purchase"
  value_mode = "revenue"
}
```

## Schema

### Required

- `site_id` (String) Forces replacement.
- `name` (String)
- `kind` (String) `page`, `event`, `engagement`, `outbound` or `download`.

### Optional

- `match_type` (`exact`, `prefix`, `glob`, `regex`), `pattern`, `hostname`, `event_name`,
  `conditions` (list of `{key, operator, value}`; operators `is`, `is_not`, `contains`, `gt`, `lt`),
  `scroll_threshold`, `time_threshold_seconds`, `value_mode` (`none`, `fixed`, `revenue`),
  `fixed_value`, `count_mode` (`once_per_visit`, `every`), `archived`.

### Read-Only

- `id` (String) Goal id (`goal_…`).

## Import

```shell
terraform import privatus_goal.purchase pa_ABC123/goal_9XK2
```
