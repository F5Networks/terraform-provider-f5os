# Manages System settings on F5OS platforms
resource "f5os_system" "system_settings" {
  hostname          = "system.example.net"
  motd              = "Todays weather is great!"
  login_banner      = "Welcome to the system."
  timezone          = "UTC"
  cli_timeout       = 3600
  token_lifetime    = 15
  sshd_idle_timeout = 1800
  httpd_ciphersuite = "ECDHE-RSA-AES256-GCM-SHA384"
  sshd_ciphers      = ["aes256-ctr", "aes256-gcm@openssh.com"]
  sshd_kex_alg      = ["ecdh-sha2-nistp384"]
  sshd_mac_alg      = ["hmac-sha1-96"]
  sshd_hkey_alg     = ["ssh-rsa"]
}