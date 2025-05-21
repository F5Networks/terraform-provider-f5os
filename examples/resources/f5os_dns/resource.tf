# Manages DNS servers and search domains on F5OS platforms
resource "f5os_dns" "test" {
  dns_servers = ["8.8.8.8"]
  dns_domains = ["internal.domain"]
}