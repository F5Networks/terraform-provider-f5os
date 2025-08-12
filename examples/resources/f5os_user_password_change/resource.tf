# Change password for admin user
resource "f5os_user_password_change" "admin_password" {
  user_name    = "admin"
  old_password = "default_pass"
  new_password = "new_admin_pass"
}

# Change password for standard user
resource "f5os_user_password_change" "user1_password" {
  user_name    = "user1"
  old_password = "default_pass"
  new_password = "mySecurePass@123"
}

# Change password for root user
resource "f5os_user_password_change" "root_password" {
  user_name    = "root"
  old_password = "root_default"
  new_password = "root_secure_password"
}