# Import a tenant image from a remote HTTPS server
resource "f5os_tenant_image" "test" {
  image_name  = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
  remote_host = "xxxxx"
  remote_path = "v17.1.0/daily/current/VM"
  local_path  = "images" ## for velos partition/rSeries appliance this path should be `images`/`images/tenant` respectively
  protocol    = "https"  ## supported values: scp, sftp, https
  insecure    = true     ## skip TLS certificate verification on the remote host
  timeout     = 360
}

# Import a tenant image via SCP with credentials
resource "f5os_tenant_image" "scp_example" {
  image_name      = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
  remote_host     = "xxxxx"
  remote_path     = "v17.1.0/daily/current/VM"
  local_path      = "images/tenant"
  protocol        = "scp"
  remote_user     = "imageuser"
  remote_password = "imagepass"
  remote_port     = 22
  timeout         = 600
}