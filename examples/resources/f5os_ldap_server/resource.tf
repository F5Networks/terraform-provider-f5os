# Basic LDAPS example.
resource "f5os_ldap_server" "example" {
  server_group = "ldap-servers"
  address      = "192.0.2.10"
  auth_port    = 636
  type         = "ldaps"
}

# Example with default LDAP port (389).
resource "f5os_ldap_server" "example_standard" {
  server_group = "ldap-servers"
  address      = "192.0.2.11"
  type         = "ldap"
}

# Example with minimal configuration (uses device defaults for port and type).
resource "f5os_ldap_server" "example_minimal" {
  server_group = "ldap-servers"
  address      = "192.0.2.12"
}
