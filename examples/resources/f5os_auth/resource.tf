# Example: Configure authentication order and role GID mappings
resource "f5os_auth" "aaa" {
  auth_order = ["local", "ldap", "radius"]

  remote_roles = [
    { rolename = "admin", remote_gid = 9000 },
    { rolename = "operator", remote_gid = 9001 },
  ]

  password_policy = {
    min_length         = 8
    required_numeric   = 1
    required_uppercase = 1
    required_lowercase = 1
    required_special   = 1
    reject_username    = true
    max_login_failures = 5
    unlock_time        = 300
    max_age            = 90

    # min_days, remember, and warn_age are only supported on F5OS 2.0.0 and later.
    min_days = 1
    remember = 5
    warn_age = 14
  }

  # login_policy is only supported on F5OS 2.0.0 and later.
  login_policy = {
    admin_role_limit           = true
    restconf_max_session_limit = 10
    ssh_max_session_limit      = 10
  }

  # ldap object classes are only supported on F5OS 2.0.0 and later.
  ldap = {
    user_object_class  = ["posixAccount"]
    group_object_class = ["posixGroup"]
  }
}
