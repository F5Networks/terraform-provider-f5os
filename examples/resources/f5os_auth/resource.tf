# Example: Configure authentication order and role GID mappings
resource "f5os_auth" "aaa" {
  auth_order = ["local", "ldap", "radius"]

  remote_roles = [
    { rolename = "admin", remote_gid = 9000 },
    { rolename = "operator", remote_gid = 9001 },
  ]

  password_policy = {
    min_length         = 8
    required_numeric   = 1
    required_uppercase = 1
    required_lowercase = 1
    required_special   = 1
    reject_username    = true
    max_login_failures = 5
    unlock_time        = 300
    max_age            = 90
  }
}
