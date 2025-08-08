package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccQkviewResource_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccQkviewResourceConfig("test_qkview", 60, 5, 2, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "test_qkview"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "timeout", "60"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "max_file_size", "5"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "max_core_size", "2"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "exclude_cores", "true"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
					resource.TestCheckResourceAttrSet("f5os_qkview.test", "id"),
					resource.TestCheckResourceAttrSet("f5os_qkview.test", "generated_file"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccQkviewResource_CustomParams(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with custom parameters
			{
				Config: testAccQkviewResourceConfig("custom_qkview", 60, 3, 2, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "custom_qkview"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "timeout", "60"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "max_file_size", "3"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "max_core_size", "2"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "exclude_cores", "true"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
					resource.TestCheckResourceAttrSet("f5os_qkview.test", "id"),
					resource.TestCheckResourceAttrSet("f5os_qkview.test", "generated_file"),
				),
			},
		},
	})
}

func TestAccQkviewResource_InvalidMaxFileSize(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccQkviewResourceConfig("invalid_qkview", 60, 1500, 2, true), // max_file_size > 1000
				ExpectError: regexp.MustCompile("max_file_size must be between 2-1000"),
			},
		},
	})
}

func TestAccQkviewResource_InvalidMaxCoreSize(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccQkviewResourceConfig("invalid_qkview", 60, 5, 1, true), // max_core_size < 2
				ExpectError: regexp.MustCompile("max_core_size must be between 2-1000"),
			},
		},
	})
}

func TestAccQkviewResource_DuplicateFilename(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create first qkview
			{
				Config: testAccQkviewResourceConfig("duplicate_qkview", 60, 2, 2, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "duplicate_qkview"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
				),
			},
			// Try to create another qkview with the same filename - should fail
			{
				Config:      testAccQkviewResourceDuplicateConfig(),
				ExpectError: regexp.MustCompile("already exists"),
			},
		},
	})
}

func TestAccQkviewResource_RequiresReplace(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create initial qkview
			{
				Config: testAccQkviewResourceConfig("replace_test", 60, 2, 2, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "replace_test"),
				),
			},
			// Change filename - should require replace
			{
				Config: testAccQkviewResourceConfig("replace_test_new", 60, 2, 2, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "replace_test_new"),
				),
			},
		},
	})
}

func TestAccQkviewResource_MinimalConfig(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with minimal config (only filename)
			{
				Config: `
					resource "f5os_qkview" "test" {
					  filename = "minimal_qkview"
					  timeout = 60
					  max_file_size = 2
					  max_core_size = 2
					  exclude_cores = true
					}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "minimal_qkview"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "timeout", "60"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "max_file_size", "2"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "max_core_size", "2"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "exclude_cores", "true"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
					resource.TestCheckResourceAttrSet("f5os_qkview.test", "id"),
					resource.TestCheckResourceAttrSet("f5os_qkview.test", "generated_file"),
				),
			},
		},
	})
}

func testAccQkviewResourceConfig(filename string, timeout, maxFileSize, maxCoreSize int, excludeCores bool) string {
	return fmt.Sprintf(`
resource "f5os_qkview" "test" {
  filename       = %[1]q
  timeout        = %[2]d
  max_file_size  = %[3]d
  max_core_size  = %[4]d
  exclude_cores  = %[5]t
}
`, filename, timeout, maxFileSize, maxCoreSize, excludeCores)
}

func testAccQkviewResourceDuplicateConfig() string {
	return `
resource "f5os_qkview" "test" {
  filename = "duplicate_qkview"
  timeout = 60
  max_file_size = 2
  max_core_size = 2
  exclude_cores = true
}

resource "f5os_qkview" "test2" {
  filename = "duplicate_qkview"
  timeout = 60
  max_file_size = 2
  max_core_size = 2
  exclude_cores = true
}
`
}
