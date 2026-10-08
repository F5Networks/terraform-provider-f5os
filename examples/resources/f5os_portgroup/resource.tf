resource "f5os_portgroup" "example" {
  name = "5"
  mode = "MODE_10GB"
}

resource "f5os_portgroup" "with_ddm" {
  name               = "5"
  mode               = "MODE_25GB"
  ddm_poll_frequency = 60
}
