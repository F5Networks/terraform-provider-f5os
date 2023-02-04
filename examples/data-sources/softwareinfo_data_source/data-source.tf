terraform {
  required_providers {
    f5os = {
      source  = "f5networks/bigip"
    }
  }
}
provider "f5os" {
  username = "admin"
  password = "ess-pwe-f5site02"
  host     = "https://10.144.140.70"
}

data "f5os_softwareinfo" "test" {}