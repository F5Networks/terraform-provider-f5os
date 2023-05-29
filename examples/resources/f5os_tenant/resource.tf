# Manage F5OS Tenant
resource "f5os_tenant" "test3" {
  name              = "testtenant-ecosys3"
  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
  mgmt_ip           = "10.100.100.26"
  mgmt_gateway      = "10.100.100.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  nodes             = [1]
  vlans             = [1, 2]
  running_state     = "deployed"
  virtual_disk_size = 82
}