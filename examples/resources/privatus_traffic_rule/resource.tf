resource "privatus_traffic_rule" "admin" {
  site_id = privatus_site.blog.id
  list    = "paths"
  pattern = "/wp-admin/*"
  label   = "WordPress admin"
}

# IP rules are matched in memory at ingest; IPs are never stored.
resource "privatus_traffic_rule" "office" {
  site_id = privatus_site.blog.id
  list    = "ip_ranges"
  pattern = "203.0.113.0/24"
  label   = "Office"
}
