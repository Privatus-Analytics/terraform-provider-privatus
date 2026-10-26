# terraform-provider-privatus

> **Not published.** `privatus-analytics/privatus` is not available on the Terraform Registry (the `privatus-analytics` namespace is not set up).
> Do not install anything with this name from a public registry: it is not ours, whoever published it.
> Build and install it from this folder until we announce the published name.

**Set up your Privatus Analytics sites and goals as code.**

You run many sites, and you set each one up by hand: the site, its goals, the
traffic rules. It is slow, and it is easy to get one wrong.

This provider lets Terraform (a tool that sets up services from config files)
create and update your Privatus Analytics sites, goals and traffic rules for
you.

So every site is set up the same way, and changes are checked like any other
code change. The numbers are honest too. Privatus Analytics counts visits
without cookies, so there is no cookie pop-up for your analytics, and it leaves
bots out.

**Free plan, no card:** 10,000 events a month (an event is a page view or an action you track) on up to 3 sites. [Start free at privatusanalytics.com](https://privatusanalytics.com).

Terraform provider for [Privatus Analytics](https://privatusanalytics.com),
the privacy-first, cookieless web analytics. Built with the
[terraform-plugin-framework](https://github.com/hashicorp/terraform-plugin-framework).

| Kind        | Name                    |
|-------------|-------------------------|
| Resource    | `privatus_site`         |
| Resource    | `privatus_goal`         |
| Resource    | `privatus_traffic_rule` |
| Data source | `privatus_workspace`    |

## Usage

```terraform
terraform {
  required_providers {
    privatus = { source = "privatus-analytics/privatus" }
  }
}

provider "privatus" {
  # api_token = "pat_…"   # or PRIVATUS_API_TOKEN
  # endpoint  = "https://privatusanalytics.com"  # or PRIVATUS_ENDPOINT
}

data "privatus_workspace" "main" { name = "Acme Inc" }

resource "privatus_site" "shop" {
  workspace_id = data.privatus_workspace.main.id
  domain       = "shop.example.com"
  name         = "Shop"
}

resource "privatus_goal" "purchase" {
  site_id    = privatus_site.shop.id
  name       = "Purchase"
  kind       = "event"
  event_name = "Purchase"
  value_mode = "revenue"
}

resource "privatus_traffic_rule" "admin" {
  site_id = privatus_site.shop.id
  list    = "paths"
  pattern = "/admin/*"
}
```

More in [`examples/`](examples) and [`docs/`](docs).

Create an API token in the app under **Settings → API tokens**. A token can
only narrow its member's permissions: managing sites and traffic rules
needs `sites.manage`, goals need `content.write`.

## API mapping

All calls go to the management API (`{endpoint}/<path>.json`, `Authorization:
Bearer pat_…`). Success bodies are `{data, meta}`; errors are
`{error: {code, message, details}}` and surface verbatim as Terraform
diagnostics.

| Resource | Endpoints | Status |
|---|---|---|
| `privatus_workspace` | `GET /workspaces.json` (paged) | in openapi.json |
| `privatus_site` | `POST /workspaces/{ws}/sites.json`, `GET/PATCH/DELETE /sites/{id}.json` | in openapi.json |
| `privatus_goal` | `POST /sites/{site}/goals.json`, `GET/PATCH/DELETE /sites/{site}/goals/{id}.json`, `POST …/{id}/archive.json`, `…/unarchive.json` | **assumed** (routes exist; not yet in openapi.json) |
| `privatus_traffic_rule` | `GET/POST /sites/{site}/traffic_rules.json`, `PATCH/DELETE /sites/{site}/traffic_rules/{id}.json` | **assumed** (routes exist; not yet in openapi.json) |

Notes:

- Site create only accepts `domain`, `name`, `timezone` and `currency`.
  The provider PATCHes the remaining settings right after creating the site.
- Destroying a site sends `{"confirm": "<domain>"}`, as the API requires.
- Traffic rules have no single-rule GET route, so reads scan the list and
  ids are numeric (stored as strings). Country codes are upper-cased by the
  server; the provider keeps your spelling if only the case differs.

## Development

Everything runs in Docker; no local Go needed.

```sh
docker run --rm -v "$PWD":/src -w /src golang:1.26 \
  sh -c 'go build ./... && go vet ./... && go test ./...'
```

Tests use `net/http/httptest` with an in-memory fake of the API and call
the resources' CRUD methods directly (no Terraform CLI needed).

To try a local build, add a `dev_overrides` block for
`privatus-analytics/privatus` pointing at your `$GOPATH/bin` in
`~/.terraformrc`, then `go install .`.

## License

MIT, see [LICENSE](LICENSE).
