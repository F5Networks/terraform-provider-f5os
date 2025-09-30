# Configure SNMP with communities, targets, users and MIB settings
resource "f5os_snmp" "example" {
  state = "present"

  # SNMP Communities
  snmp_community = [
    {
      name           = "test_public"
      security_model = ["v1", "v2c"]
    },
    {
      name           = "test_private"
      security_model = ["v2c"]
    }
  ]

  # SNMP Targets
  snmp_target = [
    {
      name           = "monitoring_v2c"
      security_model = "v2c"
      community      = "test_public"
      ipv4_address   = "192.168.1.100"
      port           = 162
    },
    {
      name         = "monitoring_v3"
      user         = "admin_user"
      ipv4_address = "192.168.1.101"
      port         = 162
    }
  ]

  # SNMP Users (v3)
  snmp_user = [
    {
      name           = "admin_user"
      auth_proto     = "sha"
      auth_passwd    = "strong_authentication_password"
      privacy_proto  = "aes"
      privacy_passwd = "strong_privacy_password"
    },
    {
      name        = "read_user"
      auth_proto  = "md5"
      auth_passwd = "read_only_password"
    }
  ]

  # SNMP MIB Settings
  snmp_mib = {
    sysname     = "F5OS-Production-System"
    syscontact  = "network-admin@company.com"
    syslocation = "DataCenter-1/Rack-42/Slot-1"
  }
}
