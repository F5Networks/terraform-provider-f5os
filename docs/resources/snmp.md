# f5os_snmp Resource

Resource used to manage SNMP configuration (Communities, Users, Targets, and MIB settings) on F5OS systems (VELOS or rSeries).

-> **NOTE:** The `f5os_snmp` resource manages SNMP settings on F5OS platforms using Open API. Due to API restrictions, passwords cannot be retrieved which may lead to Terraform detecting changes on every plan.

## Example Usage

### Basic SNMP Community Configuration

```terraform
resource "f5os_snmp" "example" {
  snmp_community = [
    {
      name           = "public"
      security_model = ["v1", "v2c"]
    }
  ]
}
```

### SNMP v3 User Configuration

```terraform
resource "f5os_snmp" "snmpv3_example" {
  snmp_user = [
    {
      name           = "snmpv3user"
      auth_proto     = "sha"
      auth_passwd    = "authentication_password"
      privacy_proto  = "aes"
      privacy_passwd = "privacy_password"
    }
  ]
}
```

### SNMP Targets Configuration

```terraform
resource "f5os_snmp" "targets_example" {
  snmp_community = [
    {
      name           = "monitoring"
      security_model = ["v2c"]
    }
  ]
  
  snmp_target = [
    {
      name           = "monitoring_server"
      security_model = "v2c"
      community      = "monitoring"
      ipv4_address   = "192.168.1.100"
      port           = 162
    },
    {
      name         = "v3_target"
      user         = "snmpv3user"
      ipv4_address = "192.168.1.101"
      port         = 162
    }
  ]
  
  snmp_user = [
    {
      name           = "snmpv3user"
      auth_proto     = "sha"
      auth_passwd    = "auth_password"
      privacy_proto  = "aes"
      privacy_passwd = "privacy_password"
    }
  ]
}
```

### Complete SNMP Configuration

```terraform
resource "f5os_snmp" "complete_example" {
  state = "present"
  
  snmp_community = [
    {
      name           = "public"
      security_model = ["v1", "v2c"]
    },
    {
      name           = "private"
      security_model = ["v2c"]
    }
  ]
  
  snmp_target = [
    {
      name           = "monitoring_v2c"
      security_model = "v2c"
      community      = "public"
      ipv4_address   = "10.1.1.100"
      port           = 162
    },
    {
      name         = "monitoring_v3"
      user         = "admin_user"
      ipv6_address = "2001:db8::100"
      port         = 162
    }
  ]
  
  snmp_user = [
    {
      name           = "admin_user"
      auth_proto     = "sha"
      auth_passwd    = "strong_auth_password"
      privacy_proto  = "aes"
      privacy_passwd = "strong_privacy_password"
    },
    {
      name        = "read_user"
      auth_proto  = "md5"
      auth_passwd = "read_password"
    }
  ]
  
  snmp_mib = {
    sysname     = "F5OS-Production-System"
    syscontact  = "network-admin@company.com"
    syslocation = "DataCenter-1/Rack-42/Slot-1"
  }
}
```

### Remove SNMP Configuration

```terraform
resource "f5os_snmp" "remove_example" {
  state = "absent"
  
  snmp_community = [
    {
      name           = "public"
      security_model = ["v1", "v2c"]
    }
  ]
  
  snmp_target = [
    {
      name           = "monitoring_server"
      security_model = "v2c"
      community      = "public"
      ipv4_address   = "192.168.1.100"
      port           = 162
    }
  ]
}
```

## Argument Reference

The following arguments are supported:

### Top-level Arguments

* `snmp_community` - (Optional) List of SNMP Community configurations. Each community represents a group with specific security models.
* `snmp_target` - (Optional) List of SNMP Target configurations. Targets define where SNMP notifications are sent.
* `snmp_user` - (Optional) List of SNMP User configurations for SNMPv3. Due to API restrictions, passwords cannot be retrieved which leads to Terraform always detecting changes.
* `snmp_mib` - (Optional) Custom SNMP MIB entries for system information.
* `state` - (Optional) State of the SNMP configuration. Valid options: `present`, `absent`. Default is `present`.

### SNMP Community Block

The `snmp_community` block supports:

* `name` - (Required) Unique name for the SNMP community.
* `security_model` - (Optional) List of security models for the community. Valid options: `v1`, `v2c`. Default is `["v1"]`.

### SNMP Target Block

The `snmp_target` block supports:

* `name` - (Required) Unique name for the SNMP target.
* `security_model` - (Optional) Security model for the SNMP target. Valid options: `v1`, `v2c`. Note: `v3` is applied automatically when `user` is specified.
* `community` - (Optional) SNMP community name to use for this target. Cannot be used with `user`.
* `user` - (Optional) SNMP user for SNMPv3 targets. Cannot be used with `community` or `security_model`.
* `ipv4_address` - (Optional) IPv4 address for the SNMP target. Cannot be used with `ipv6_address`.
* `ipv6_address` - (Optional) IPv6 address for the SNMP target. Cannot be used with `ipv4_address`.
* `port` - (Required) Port number for the SNMP target.

### SNMP User Block

The `snmp_user` block supports:

* `name` - (Required) Unique name for the SNMP user.
* `auth_proto` - (Optional) Authentication protocol. Valid options: `sha`, `md5`.
* `auth_passwd` - (Optional, Sensitive) Password for authentication.
* `privacy_proto` - (Optional) Privacy protocol. Valid options: `aes`, `des`. Requires authentication to be configured.
* `privacy_passwd` - (Optional, Sensitive) Password for encryption.

### SNMP MIB Block

The `snmp_mib` block supports:

* `sysname` - (Optional) SNMPv2 sysName.
* `syscontact` - (Optional) SNMPv2 sysContact.
* `syslocation` - (Optional) SNMPv2 sysLocation.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `id` - Unique identifier for the resource, computed from the SNMP configuration.

## Import

SNMP configurations can be imported using the resource ID:

```bash
terraform import f5os_snmp.example <id>
```

Note: Due to the nature of SNMP configuration and password sensitivity, importing may require manual adjustment of the Terraform configuration to match the actual state.

## Notes

* A PATCH call updates only the specified targets, communities, or users mentioned in the request leaving all other unmentioned things unchanged.
* A target cannot be mapped to a community if it is already mapped to a user.
* By default, the security model will be set to v3 when a target is mapped to a user.
* Due to API restrictions, passwords can not be retrieved which leads to Terraform always detecting changes for user configurations with passwords.
* Privacy protocol requires authentication to be configured as well.
