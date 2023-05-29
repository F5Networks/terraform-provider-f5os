# Manages Changing F5os Partition password
resource "f5os_partition_change_password" "changepass" {
  user_name    = "xxxxx"
  old_password = "xxxxxxxx"
  new_password = "xxxxxx"
}