locals {
  instance_name = ["test-ecosys-1", "test-ecosys-2", "test-ecosys-3"]
  instance_ip   = ["10.14.10.11", "10.14.10.12", "10.14.10.13"]
}
resource "f5os_tenant" "test3" {
  count             = length(local.instance_name)
  name              = element(local.instance_name, count.index)
  cpu_cores         = 8
  cryptos           = "enabled"
  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
  mgmt_gateway      = "10.14.10.1"
  mgmt_ip           = element(local.instance_ip, count.index)
  mgmt_prefix       = 24
  running_state     = "configured"
  virtual_disk_size = 82
  nodes             = [1]
  vlans             = [1, 2, 3]
}