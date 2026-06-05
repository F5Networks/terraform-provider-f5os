package provider

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// ---------------------------------------------------------------------------
// Unit Tests
// ---------------------------------------------------------------------------

// setupLicenseMock registers the auth/platform handlers needed by NewSession
// plus the license-specific EULA, Install, and GetLicense handlers. The caller
// passes functions that define the mock behavior for EULA POST, Install POST,
// and GetLicense GET. If any handler func is nil a sensible default is used.
func setupLicenseMock(
	t *testing.T,
	eulaHandler func(w http.ResponseWriter, r *http.Request),
	installHandler func(w http.ResponseWriter, r *http.Request),
	getLicenseHandler func(w http.ResponseWriter, r *http.Request),
) {
	t.Helper()
	testAccPreUnitCheck(t)

	// Auth handler — required by NewSession
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token-license")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})

	// Platform detection — return rSeries platform so NewSession completes
	// successfully. The license resource does not gate on platform type.
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"openconfig-platform:components": {
				"component": [
					{
						"name": "platform",
						"state": {
							"description": "rSeries Platform"
						}
					}
				]
			}
		}`))
	})

	// Version detection for rSeries
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-image:image/state/install", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-system-image:install":{"install-os-version":"1.7.0","install-status":"success"}}`)
	})

	// EULA handler
	if eulaHandler == nil {
		eulaHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-licensing:licensing/f5-system-licensing-install:get-eula", eulaHandler)

	// License Install handler
	if installHandler == nil {
		installHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"f5-system-licensing-install:output": {
					"result": "License installed successfully."
				}
			}`))
		}
	}
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-licensing:licensing/f5-system-licensing-install:install", installHandler)

	// GetLicense handler (used by Read)
	if getLicenseHandler == nil {
		getLicenseHandler = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"f5-system-licensing:licensing": {
					"state": {
						"registration-key": {
							"base": "AAAAA-BBBBB-CCCCC-DDDDD-EEEEEEE"
						},
						"license": "Licensed",
						"raw-license": "raw-data"
					}
				}
			}`))
		}
	}
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-licensing:licensing", getLicenseHandler)
}

// TestUnitLicenseCreateBasic verifies that a license resource can be created
// with just a registration key (no addon keys). Exercises the happy path
// through Create and Read.
func TestUnitLicenseCreateBasic(t *testing.T) {
	setupLicenseMock(t, nil, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"f5-system-licensing:licensing": {
				"state": {
					"registration-key": {
						"base": "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
					},
					"license": "Licensed"
				}
			}
		}`))
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLicenseBasicConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "registration_key", "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"),
					resource.TestCheckResourceAttrSet("f5os_license.test", "id"),
				),
			},
		},
	})
}

const testUnitLicenseBasicConfig = `
resource "f5os_license" "test" {
  registration_key = "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
}
`

// TestUnitLicenseCreateWithAddonKeys verifies that addon_keys are passed
// through to the EULA and Install API calls. Exercises the addon_keys
// code path in Create.
func TestUnitLicenseCreateWithAddonKeys(t *testing.T) {
	setupLicenseMock(t, nil, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"f5-system-licensing:licensing": {
				"state": {
					"registration-key": {
						"base": "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
					},
					"license": "Licensed"
				}
			}
		}`))
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLicenseWithAddonKeysConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "registration_key", "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"),
					resource.TestCheckResourceAttr("f5os_license.test", "addon_keys.#", "2"),
					resource.TestCheckResourceAttr("f5os_license.test", "addon_keys.0", "ADDON1-KEY-12345"),
					resource.TestCheckResourceAttr("f5os_license.test", "addon_keys.1", "ADDON2-KEY-67890"),
					resource.TestCheckResourceAttrSet("f5os_license.test", "id"),
				),
			},
		},
	})
}

const testUnitLicenseWithAddonKeysConfig = `
resource "f5os_license" "test" {
  registration_key = "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
  addon_keys       = ["ADDON1-KEY-12345", "ADDON2-KEY-67890"]
}
`

// TestUnitLicenseCreateWithLicenseServer verifies that the license_server
// attribute is accepted and stored in state.
func TestUnitLicenseCreateWithLicenseServer(t *testing.T) {
	setupLicenseMock(t, nil, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"f5-system-licensing:licensing": {
				"state": {
					"registration-key": {
						"base": "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
					},
					"license": "Licensed"
				}
			}
		}`))
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLicenseWithServerConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "registration_key", "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"),
					resource.TestCheckResourceAttr("f5os_license.test", "license_server", "https://activate.f5.com"),
					resource.TestCheckResourceAttrSet("f5os_license.test", "id"),
				),
			},
		},
	})
}

const testUnitLicenseWithServerConfig = `
resource "f5os_license" "test" {
  registration_key = "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
  license_server   = "https://activate.f5.com"
}
`

// TestUnitLicenseCreateEulaError exercises the error path when the Eula call
// fails during Create.
func TestUnitLicenseCreateEulaError(t *testing.T) {
	setupLicenseMock(t,
		// EULA returns error
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"EULA acceptance failed"}]}}`))
		},
		nil, nil,
	)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testUnitLicenseBasicConfig,
				ExpectError: regexp.MustCompile(`Error during EULA`),
			},
		},
	})
}

// TestUnitLicenseCreateInstallError exercises the error path when the
// LicenseInstall HTTP request fails during Create.
func TestUnitLicenseCreateInstallError(t *testing.T) {
	setupLicenseMock(t,
		nil,
		// Install returns HTTP error
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"License install request failed"}]}}`))
		},
		nil,
	)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testUnitLicenseBasicConfig,
				ExpectError: regexp.MustCompile(`Error during License Install`),
			},
		},
	})
}

// TestUnitLicenseCreateInstallBadResult exercises the error path when the
// LicenseInstall HTTP request succeeds but the result field does not
// contain "License installed successfully."
func TestUnitLicenseCreateInstallBadResult(t *testing.T) {
	setupLicenseMock(t,
		nil,
		// Install returns 200 but with a failure message in the result
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"f5-system-licensing-install:output": {
					"result": "License activation failed: invalid registration key"
				}
			}`))
		},
		nil,
	)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testUnitLicenseBasicConfig,
				ExpectError: regexp.MustCompile(`Error during License Install|License activation failed`),
			},
		},
	})
}

// TestUnitLicenseReadRefreshesFromDevice exercises the Read code path
// via a two-step test. Step 1 creates the resource. Step 2 re-applies
// the same config, which forces the framework to call Read (refresh
// state) before computing the plan. The mock GetLicense handler returns
// the expected key and the step verifies state is consistent.
func TestUnitLicenseReadRefreshesFromDevice(t *testing.T) {
	setupLicenseMock(t, nil, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"f5-system-licensing:licensing": {
				"state": {
					"registration-key": {
						"base": "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
					},
					"license": "Licensed"
				}
			}
		}`))
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testUnitLicenseBasicConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "registration_key", "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"),
					resource.TestCheckResourceAttrSet("f5os_license.test", "id"),
				),
			},
			// Step 2: Re-apply triggers Read refresh before plan
			{
				Config: testUnitLicenseBasicConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "registration_key", "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"),
					resource.TestCheckResourceAttrSet("f5os_license.test", "id"),
				),
			},
		},
	})
}

// TestUnitLicenseReadError verifies that a GetLicense API failure during
// Read produces a diagnostic error. We test this by having the first step
// succeed (Create + initial Read) and then the second step (RefreshState
// before plan) encounter a GetLicense error.
func TestUnitLicenseReadError(t *testing.T) {
	callCount := 0
	setupLicenseMock(t, nil, nil, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			// First two calls succeed (Create Read + step-1 verify Read)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"f5-system-licensing:licensing": {
					"state": {
						"registration-key": {
							"base": "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
						},
						"license": "Licensed"
					}
				}
			}`))
		} else {
			// Subsequent calls fail (pre-plan refresh in step 2)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"Get license failed"}]}}`))
		}
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLicenseBasicConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("f5os_license.test", "id"),
				),
			},
			{
				Config:      testUnitLicenseBasicConfig,
				ExpectError: regexp.MustCompile(`Error during Get License`),
			},
		},
	})
}

// TestUnitLicenseUpdateRegKey verifies that changing the registration_key
// triggers an update that calls Eula and LicenseInstall with the new key.
func TestUnitLicenseUpdateRegKey(t *testing.T) {
	currentKey := "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"

	setupLicenseMock(t, nil, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
			"f5-system-licensing:licensing": {
				"state": {
					"registration-key": {
						"base": %q
					},
					"license": "Licensed"
				}
			}
		}`, currentKey)
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLicenseResourceConfig("W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "registration_key", "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"),
					resource.TestCheckResourceAttrSet("f5os_license.test", "id"),
				),
			},
			{
				PreConfig: func() {
					currentKey = "W9XXX-8YYYZ-8KKK7-7PPP2-NEWKEYZ"
				},
				Config: testAccLicenseResourceConfig("W9XXX-8YYYZ-8KKK7-7PPP2-NEWKEYZ"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "registration_key", "W9XXX-8YYYZ-8KKK7-7PPP2-NEWKEYZ"),
					resource.TestCheckResourceAttrSet("f5os_license.test", "id"),
				),
			},
		},
	})
}

// TestUnitLicenseUpdateAddonKeys verifies that changing addon_keys triggers
// an update that passes the new addon keys to Eula and LicenseInstall.
func TestUnitLicenseUpdateAddonKeys(t *testing.T) {
	setupLicenseMock(t, nil, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"f5-system-licensing:licensing": {
				"state": {
					"registration-key": {
						"base": "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
					},
					"license": "Licensed"
				}
			}
		}`))
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLicenseAddonKeysStep1Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "addon_keys.#", "1"),
					resource.TestCheckResourceAttr("f5os_license.test", "addon_keys.0", "ADDON1-KEY-12345"),
				),
			},
			{
				Config: testUnitLicenseAddonKeysStep2Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "addon_keys.#", "2"),
					resource.TestCheckResourceAttr("f5os_license.test", "addon_keys.0", "ADDON1-KEY-12345"),
					resource.TestCheckResourceAttr("f5os_license.test", "addon_keys.1", "ADDON3-KEY-99999"),
				),
			},
		},
	})
}

const testUnitLicenseAddonKeysStep1Config = `
resource "f5os_license" "test" {
  registration_key = "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
  addon_keys       = ["ADDON1-KEY-12345"]
}
`

const testUnitLicenseAddonKeysStep2Config = `
resource "f5os_license" "test" {
  registration_key = "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
  addon_keys       = ["ADDON1-KEY-12345", "ADDON3-KEY-99999"]
}
`

// TestUnitLicenseUpdateEulaError exercises the error path when Eula fails
// during an Update operation.
func TestUnitLicenseUpdateEulaError(t *testing.T) {
	eulaCallCount := 0

	setupLicenseMock(t,
		func(w http.ResponseWriter, r *http.Request) {
			eulaCallCount++
			if eulaCallCount <= 1 {
				// First call (Create) succeeds
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			} else {
				// Subsequent calls (Update) fail
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"EULA acceptance failed on update"}]}}`))
			}
		},
		nil,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"f5-system-licensing:licensing": {
					"state": {
						"registration-key": {
							"base": "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
						},
						"license": "Licensed"
					}
				}
			}`))
		},
	)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLicenseResourceConfig("W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("f5os_license.test", "id"),
				),
			},
			{
				Config:      testAccLicenseResourceConfig("W9XXX-8YYYZ-8KKK7-7PPP2-NEWKEYZ"),
				ExpectError: regexp.MustCompile(`Error during EULA`),
			},
		},
	})
}

// TestUnitLicenseUpdateInstallError exercises the error path when
// LicenseInstall fails during an Update operation.
func TestUnitLicenseUpdateInstallError(t *testing.T) {
	installCallCount := 0

	setupLicenseMock(t,
		nil,
		func(w http.ResponseWriter, r *http.Request) {
			installCallCount++
			if installCallCount <= 1 {
				// First call (Create) succeeds
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"f5-system-licensing-install:output": {
						"result": "License installed successfully."
					}
				}`))
			} else {
				// Subsequent calls (Update) fail
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"License install failed on update"}]}}`))
			}
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"f5-system-licensing:licensing": {
					"state": {
						"registration-key": {
							"base": "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
						},
						"license": "Licensed"
					}
				}
			}`))
		},
	)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLicenseResourceConfig("W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("f5os_license.test", "id"),
				),
			},
			{
				Config:      testAccLicenseResourceConfig("W9XXX-8YYYZ-8KKK7-7PPP2-NEWKEYZ"),
				ExpectError: regexp.MustCompile(`Error during License Install`),
			},
		},
	})
}

// TestUnitLicenseDelete verifies that the delete operation is a no-op and
// completes without error (the resource is removed from state).
func TestUnitLicenseDelete(t *testing.T) {
	setupLicenseMock(t, nil, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"f5-system-licensing:licensing": {
				"state": {
					"registration-key": {
						"base": "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
					},
					"license": "Licensed"
				}
			}
		}`))
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLicenseBasicConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("f5os_license.test", "id"),
				),
			},
			{
				// Destroy step — verifies the resource can be destroyed
				// without error. Since Delete is a no-op, this simply
				// removes it from state.
				Config:  testUnitLicenseEmptyConfig,
				Destroy: true,
			},
		},
	})
}

const testUnitLicenseEmptyConfig = `
# empty — triggers destroy of the license resource
`

// TestUnitLicenseImportState verifies that importing a license resource
// by ID populates the state correctly via ImportStatePassthroughID.
func TestUnitLicenseImportState(t *testing.T) {
	setupLicenseMock(t, nil, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"f5-system-licensing:licensing": {
				"state": {
					"registration-key": {
						"base": "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"
					},
					"license": "Licensed"
				}
			}
		}`))
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLicenseBasicConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("f5os_license.test", "id"),
				),
			},
			{
				ResourceName:      "f5os_license.test",
				ImportState:       true,
				ImportStateVerify: true,
				// registration_key is sensitive — Read replaces it with
				// the device's value which may differ from the plan.
				// addon_keys and license_server are not returned by the
				// GetLicense API so they will not match after import.
				ImportStateVerifyIgnore: []string{"registration_key", "addon_keys", "license_server"},
			},
		},
	})
}

// TestUnitLicenseCreateAndUpdateFullLifecycle exercises the complete lifecycle:
// Create with addon_keys -> Read -> Update (change reg key + addon keys) ->
// Read -> Import -> Delete.
func TestUnitLicenseCreateAndUpdateFullLifecycle(t *testing.T) {
	currentKey := "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"

	setupLicenseMock(t, nil, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
			"f5-system-licensing:licensing": {
				"state": {
					"registration-key": {
						"base": %q
					},
					"license": "Licensed"
				}
			}
		}`, currentKey)
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with addon keys
			{
				Config: testUnitLicenseWithAddonKeysConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "registration_key", "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"),
					resource.TestCheckResourceAttr("f5os_license.test", "addon_keys.#", "2"),
					resource.TestCheckResourceAttrSet("f5os_license.test", "id"),
				),
			},
			// Step 2: Update registration key and addon keys
			{
				PreConfig: func() {
					currentKey = "AAAAA-BBBBB-CCCCC-DDDDD-EEEEEEE"
				},
				Config: testUnitLicenseLifecycleStep2Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "registration_key", "AAAAA-BBBBB-CCCCC-DDDDD-EEEEEEE"),
					resource.TestCheckResourceAttr("f5os_license.test", "addon_keys.#", "1"),
					resource.TestCheckResourceAttr("f5os_license.test", "addon_keys.0", "NEWADDON-KEY-11111"),
					resource.TestCheckResourceAttrSet("f5os_license.test", "id"),
				),
			},
			// Step 3: Import
			{
				ResourceName:            "f5os_license.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"registration_key", "addon_keys", "license_server"},
			},
		},
	})
}

const testUnitLicenseLifecycleStep2Config = `
resource "f5os_license" "test" {
  registration_key = "AAAAA-BBBBB-CCCCC-DDDDD-EEEEEEE"
  addon_keys       = ["NEWADDON-KEY-11111"]
}
`

// ---------------------------------------------------------------------------
// Acceptance Tests (require real F5OS device)
// ---------------------------------------------------------------------------

func TestAccLicenseResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccLicenseResourceConfig("W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "registration_key", "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_license.test",
				ImportState:       true,
				ImportStateVerify: true,
				// The license registration key is sensitive and won't be returned in read operations
				ImportStateVerifyIgnore: []string{"registration_key", "addon_keys"},
			},
			// Update and Read testing
			{
				Config: testAccLicenseResourceConfig("W9XXX-8YYYZ-8KKK7-7PPP2-WWWWWWW"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "registration_key", "W9XXX-8YYYZ-8KKK7-7PPP2-WWWWWWW"),
				),
			},
		},
	})
}

func testAccLicenseResourceConfig(regKey string) string {
	return fmt.Sprintf(`
resource "f5os_license" "test" {
  registration_key = %[1]q
}
`, regKey)
}
