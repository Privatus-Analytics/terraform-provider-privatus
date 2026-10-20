# By name (case-insensitive) …
data "privatus_workspace" "main" {
  name = "Acme Inc"
}

# … or by id. With neither, the token must see exactly one workspace.
data "privatus_workspace" "agency" {
  id = "ws_7Q2K9XH3AB"
}

output "plan" {
  value = data.privatus_workspace.main.plan
}
