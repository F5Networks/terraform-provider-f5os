# Restore a backup file that already exists under configs/ on the
# device (for example, one created previously by f5os_config_backup).
resource "f5os_config_restore" "test" {
  name = "test_cfg_backup"
}

# Restore a backup file that first needs to be fetched from a remote
# server before the restore action is invoked.
resource "f5os_config_restore" "test_remote" {
  name            = "test_cfg_backup"
  remote_host     = "1.2.3.4"
  remote_user     = "corpuser"
  remote_password = "password"
  remote_path     = "/upload/test_cfg_backup"
  protocol        = "https"
}
