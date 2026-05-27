package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTenantImageDataSourceTC1Resource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}
	// Ensure DNS is configured so hostname resolution works, but do NOT
	// try to import the test image (the insecure field may be rejected
	// on some F5OS versions). Instead, discover whatever image already
	// exists on the device.
	testAccPreCheck(t)
	testAccEnsureDNSServer(t)

	imageName := testAccGetExistingImageName(t)

	// Pre-query the device to get the expected status for verification.
	client, err := newTestClientFromEnv()
	if err != nil {
		t.Skipf("Cannot create client: %v", err)
	}
	imgResp, err := client.GetImage(imageName)
	if err != nil || imgResp == nil || len(imgResp.TenantImages) == 0 {
		t.Skipf("Cannot query image %q: %v", imageName, err)
	}
	expectedStatus := imgResp.TenantImages[0].Status

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Read an image that exists on the DUT.
			{
				Config: testAccTenantImageDatasourceConfigDynamic(imageName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.f5os_tenant_image.test", "id", imageName),
					resource.TestCheckResourceAttr("data.f5os_tenant_image.test", "image_name", imageName),
					resource.TestCheckResourceAttr("data.f5os_tenant_image.test", "image_status", expectedStatus),
				),
			},
			// Step 2: Read an image that does NOT exist — expect an error.
			// The data source returns "Unable to Get Image Details" when
			// GetImage fails with a 404.
			{
				Config:      testAccTenantImageDatasourceFailConfig,
				ExpectError: regexp.MustCompile(`Unable to Get Image Details`),
			},
		},
	})
}

func testAccTenantImageDatasourceConfigDynamic(imageName string) string {
	return fmt.Sprintf(`
data "f5os_tenant_image" "test" {
  image_name = %q
}
`, imageName)
}

const testAccTenantImageDatasourceFailConfig = `
data "f5os_tenant_image" "test" {
  image_name = "BIGIP-nonexistent-datasource-test.qcow2.zip.bundle"
}
`
