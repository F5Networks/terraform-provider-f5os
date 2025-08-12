resource "f5os_user" "test" {
  username = "testuser"
  password = "MyStrongP@ss123"
  role     = "operator"
}

# Create a user with admin role and secondary role
resource "f5os_user" "admin_user" {
  username       = "adminuser"
  password       = "AdminPass789"
  role           = "admin"
  secondary_role = "operator"
  expiry_status  = "enabled"
}

# Create a user with SSH authorized keys
resource "f5os_user" "ssh_user" {
  username = "sshuser"
  password = "SSHPass456"
  role     = "operator"
  authorized_keys = [
    "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7...",
    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI..."
  ]
}

# Create a user with specific expiry date
resource "f5os_user" "temp_user" {
  username      = "tempuser"
  password      = "TempPass321"
  role          = "operator"
  expiry_status = "2024-12-31"
}