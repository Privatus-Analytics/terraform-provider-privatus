resource "privatus_site" "blog" {
  workspace_id  = data.privatus_workspace.main.id
  domain        = "blog.example.com"
  name          = "Company blog"
  timezone      = "Europe/Berlin"
  currency      = "EUR"
  honor_gpc     = true
  extra_domains = ["www.blog.example.com"]
  labels        = ["marketing"]

  # Only count traffic on the hostnames listed in the "hostnames" rules;
  # every other list keeps blocking (the default).
  traffic_rule_modes = {
    hostnames = "allow"
  }
}

output "snippet" {
  value = privatus_site.blog.snippet
}
