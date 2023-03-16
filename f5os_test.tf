terraform {
  required_providers {
    f5os = {
      source  = "f5networks/f5os"
    }
  }
}

provider "f5os" {
  username = "admin"
  password = "ess-pwe-f5site02"
  //  host     = "https://10.144.140.70"
//  host     = "https://10.144.140.50"
  host     = "https://10.144.140.190"
}
//
//data "f5os_softwareinfo" "test" {}

resource "f5os_tenant_image" "test" {
  image_name= "BIGIP-15.1.8-0.0.7.ALL-F5OS.qcow2.zip.bundle"
  remote_host="spkapexsrvc01.olympus.f5net.com"
  remote_path="v15.1.8/daily/current/VM"
  local_path="images/tenant"
  timeout = 360
}