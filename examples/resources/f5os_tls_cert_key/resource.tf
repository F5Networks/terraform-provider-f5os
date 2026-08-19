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

# F5OS 2.0.0+ import path: supply an existing certificate/key pair
# instead of generating a self-signed cert. Setting `certificate`
# (and optionally `key`) switches the resource into import mode; the
# `create-self-signed-cert` RPC is skipped. Both attributes are only
# supported on F5OS 2.0.0 or later.
resource "f5os_tls_cert_key" "imported" {
  name        = "imported-cert"
  certificate = file("cert.pem")
  key         = file("key.pem")
}