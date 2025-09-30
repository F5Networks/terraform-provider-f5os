# Example: Configure authentication order and role GID mappings
resource "f5os_auth" "aaa" {
  auth_order = ["local", "ldap", "radius"]

  remote_roles = [
    { rolename = "admin", remote_gid = 9000 },
    { rolename = "operator", remote_gid = 9001 },
  ]
}
