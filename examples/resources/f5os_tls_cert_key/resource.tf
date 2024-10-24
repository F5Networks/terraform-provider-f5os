resource "f5os_tls_cert_key" "testcert" {
  name                   = "testcert"
  days_valid             = 40
  email                  = "user@org.com"
  city                   = "Hyd"
  province               = "Telangana"
  country                = "IN"
  organization           = "F7"
  unit                   = "IT"
  key_type               = "encrypted-rsa"
  key_size               = 2048
  key_passphrase         = "test123"
  confirm_key_passphrase = "test123"
}