provider "f5os" {
  username = "education"
  password = "test123"
  host     = "http://localhost:19090"
}

# Example with custom HTTP headers (e.g. for HTTPS proxy authentication)
provider "f5os" {
  username = "education"
  password = "test123"
  host     = "http://localhost:19090"
  custom_headers = {
    "X-Tenant-ID" = "my-tenant"
  }
}