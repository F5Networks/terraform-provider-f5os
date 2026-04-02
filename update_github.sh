# !/bin/bash
rm -rf /tmp/terraform-provider-f5os
git clone --depth 1 https://github.com/F5Networks/terraform-provider-f5os.git /tmp/terraform-provider-f5os
cp -rf docs examples internal vendor templates go.mod go.sum .gitignore main.go CHANGELOG.md /tmp/terraform-provider-f5os