---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "f5os_tenant_image Data Source - terraform-provider-f5os"
subcategory: ""
description: |-
  Get information about the tenant Image on f5os platform.
  Use this data source to get information, whether image available on platform or not
---

# f5os_tenant_image (Data Source)

Get information about the tenant Image on f5os platform.

Use this data source to get information, whether image available on platform or not

## Example Usage

```terraform
data "f5os_tenant_image" "test" {
  image_name = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `image_name` (String) Name of the tenant image to check

### Read-Only

- `id` (String) Unique identifier of this data source
- `image_status` (String) Status of Image on the F5OS Platforms


