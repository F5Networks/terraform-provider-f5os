# Manages Primary key settings on F5OS platforms
resource "f5os_primarykey" "default" {
  passphrase   = "test-pass"
  salt         = "test-salt"
  force_update = true
}