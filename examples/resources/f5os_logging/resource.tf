resource "f5os_logging" "logging" {
  provider         = f5os.f5osr5600
  include_hostname = false
  servers = [
    {
      address        = "192.168.100.1"
      port           = 514
      protocol       = "tcp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "debug"
        }
      ]
    },
    {
      address        = "192.168.100.2"
      port           = 514
      protocol       = "tcp"
      authentication = false
      logs = [
        {
          facility = "authpriv"
          severity = "emergency"
        }
      ]
    }
  ]
  remote_forwarding = {
    enabled = true

    logs = [
      {
        facility = "local0"
        severity = "error"
      },
      {
        facility = "authpriv"
        severity = "critical"
      }
    ]

    files = [
      {
        name = "rseries_debug.log"
      },
      {
        name = "rseries_audit.log"
      }
    ]
  }
}
