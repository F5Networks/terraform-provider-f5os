resource "f5os_ntp_server" "test" {
  server             = "10.20.30.40"
  key_id             = 123
  prefer             = true
  iburst             = true
  ntp_service        = true
  ntp_authentication = true

  # association_type, version, and port are only supported on F5OS
  # 2.0.0 and later. Omit them on older devices.
  association_type = "SERVER"
  version          = 4
  port             = 123
}