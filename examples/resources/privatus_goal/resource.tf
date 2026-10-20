resource "privatus_goal" "signup" {
  site_id    = privatus_site.blog.id
  name       = "Signed up"
  kind       = "page"
  match_type = "prefix"
  pattern    = "/welcome"
}

resource "privatus_goal" "purchase" {
  site_id    = privatus_site.blog.id
  name       = "Purchase"
  kind       = "event"
  event_name = "Purchase"
  value_mode = "revenue" # uses the revenue/currency props sent with the event
  count_mode = "every"

  conditions = [
    { key = "plan", operator = "is", value = "pro" },
  ]
}
