# Resource for tenant image copy
resource "f5os_tenant_image" "test" {
  image_name  = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
  remote_host = "xxxxx"
  remote_path = "v17.1.0/daily/current/VM"
  local_path  = "images" ## for velos partition/rSeries appliance this path should be `images`/`images/tenant` respectively
  timeout     = 360
}