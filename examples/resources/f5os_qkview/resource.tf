# Basic qkview generation with default settings
resource "f5os_qkview" "basic" {
  filename = "basic_diagnostics"
}

# Qkview with custom parameters
resource "f5os_qkview" "custom" {
  filename      = "custom_diagnostics"
  timeout       = 600
  max_file_size = 200
  max_core_size = 50
  exclude_cores = true
}

# Qkview for specific troubleshooting with larger size limits
resource "f5os_qkview" "troubleshooting" {
  filename      = "troubleshooting_logs"
  timeout       = 1800
  max_file_size = 1000
  max_core_size = 100
  exclude_cores = false
}