---
page_title: "f5os_tenant Resource - terraform-provider-f5os"
subcategory: ""
description: |-
  Resource used for Manage F5OS tenant
---

# f5os_tenant (Resource)

Resource used for Manage F5OS tenant

## Example Usage

```terraform
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `image_name` (String) Name of the tenant image to be used.
Required for create operations
- `mgmt_gateway` (String) Tenant management gateway.
- `mgmt_ip` (String) IP address used to connect to the deployed tenant.
Required for create operations.
- `mgmt_prefix` (Number) Tenant management CIDR prefix.
- `name` (String) Name of the tenant.
The first character must be a letter.
Only lowercase alphanumeric characters are allowed.
No special or extended characters are allowed except for hyphens.
The name cannot exceed 50 characters.

### Optional

- `cpu_cores` (Number) The number of vCPUs that should be added to the tenant.
Required for create operations.
- `cryptos` (String) Whether crypto and compression hardware offload should be enabled on the tenant.
We recommend it is enabled, otherwise crypto and compression may be processed in CPU.
- `deployment_file` (String) Deployment file used for BIG-IP-Next .
Required for if `type` is `BIG-IP-Next`.
- `nodes` (List of Number) List of integers. Specifies on which blades nodes the tenants are deployed.
Required for create operations.
For single blade platforms like rSeries only the value of 1 should be provided.
- `running_state` (String) Desired running_state of the tenant.
- `timeout` (Number) The number of seconds to wait for image import to finish.
- `type` (String) Name of the tenant image to be used.
Required for create operations
- `virtual_disk_size` (Number) Minimum virtual disk size required for Tenant deployment
- `vlans` (List of Number) The existing VLAN IDs in the chassis partition that should be added to the tenant.
The order of these VLANs is ignored.
This module orders the VLANs automatically, if you deliberately re-order them in subsequent tasks, this module will not register a change.
Required for create operations

### Read-Only

- `id` (String) Tenant identifier
- `status` (String) Tenant status

