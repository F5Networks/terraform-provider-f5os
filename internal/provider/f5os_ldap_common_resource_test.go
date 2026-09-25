package provider

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestUnitLdapCommonResource(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-openconfig-aaa-ldap:ldap", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"f5-openconfig-aaa-ldap:ldap": {"base": "dc=example,dc=com"}}`))
		case http.MethodPatch:
			w.WriteHeader(http.StatusNoContent)
		}
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"f5os": providerserver.NewProtocol6WithError(New("test-version")()),
		},
		Steps: []resource.TestStep{
			{
				Config: `resource "f5os_ldap_common" "test" {
					base_dn = "dc=example,dc=com"
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ldap_common.test", "base_dn", "dc=example,dc=com"),
				),
			},
		},
	})
}
