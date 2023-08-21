resource "f5os_config_backup" "test" {
  name            = "test_cfg_backup"
  remote_host     = "1.2.3.4"
  remote_user     = "corpuser"
  remote_password = "password"
  remote_path     = "/upload/test_cfg_backup"
  protocol        = "https"
}