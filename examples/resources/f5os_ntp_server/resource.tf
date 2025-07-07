resource "f5os_ntp_server" "test" {
  server             = "10.20.30.40"
  key_id             = 123
  prefer             = true
  iburst             = true
  ntp_service        = true
  ntp_authentication = true
}