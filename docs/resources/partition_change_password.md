---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "f5os_partition_change_password Resource - terraform-provider-f5os"
subcategory: ""
description: |-
  Resource used to manage password of a specific user on a velos chassis partition.
---

# f5os_partition_change_password (Resource)

Resource used to manage password of a specific user on a velos chassis partition.

## Example Usage

```terraform
# Manages Changing F5os Partition password
resource "f5os_partition_change_password" "changepass" {
  user_name    = "xxxxx"
  old_password = "xxxxxxxx"
  new_password = "xxxxxx"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `new_password` (String, Sensitive) New password for the specified user account.
- `old_password` (String, Sensitive) Current password for the specified user account.
- `user_name` (String) Name of the chassis partition user account.

### Read-Only

- `id` (String) Unique identifier for resource.


