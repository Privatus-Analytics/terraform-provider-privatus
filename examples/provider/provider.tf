terraform {
  required_providers {
    privatus = {
      source = "privatus-analytics/privatus"
    }
  }
}

# The token can also come from PRIVATUS_API_TOKEN, and the endpoint from
# PRIVATUS_ENDPOINT.
provider "privatus" {
  api_token = var.privatus_api_token # pat_…
  # endpoint = "https://privatusanalytics.com"
}

variable "privatus_api_token" {
  type      = string
  sensitive = true
}
