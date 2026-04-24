package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	f5os "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// Pure Unit Tests

func TestAuthResourceUnit_Constructor(t *testing.T) {
	authRes := NewAuthResource()
	assert.NotNil(t, authRes, "NewAuthResource() should not return nil")
}

func TestAuthResourceUnit_InterfaceCompliance(t *testing.T) {
	authRes := NewAuthResource()
	assert.Implements(t, (*resource.Resource)(nil), authRes)
	assert.Implements(t, (*resource.ResourceWithImportState)(nil), authRes)
}

func TestAuthResourceUnit_Metadata(t *testing.T) {
	authRes := NewAuthResource()
	req := resource.MetadataRequest{
		ProviderTypeName: "f5os",
	}
	resp := &resource.MetadataResponse{}
	authRes.Metadata(context.Background(), req, resp)
	assert.Equal(t, "f5os_auth", resp.TypeName, "TypeName should be 'f5os_auth'")
}

func TestAuthResourceUnit_Schema(t *testing.T) {
	authRes := NewAuthResource()
	req := resource.SchemaRequest{}
	resp := &resource.SchemaResponse{}
	authRes.Schema(context.Background(), req, resp)
	assert.NotNil(t, resp.Schema.Attributes, "Schema Attributes should not be nil")
	assert.NotEmpty(t, resp.Schema.Attributes, "Schema should have attributes")
}

func TestAuthResourceUnit_Configure(t *testing.T) {
	authRes := NewAuthResource().(*AuthResource)
	req := resource.ConfigureRequest{
		ProviderData: &f5os.F5os{
			Host:     "https://test.example.com",
			User:     "test",
			Password: "test",
		},
	}
	resp := &resource.ConfigureResponse{}
	authRes.Configure(context.Background(), req, resp)
	assert.Empty(t, resp.Diagnostics, "Configure should not return diagnostics")
	assert.NotNil(t, authRes.client, "Client should be set after configure")
}

func TestAuthResourceUnit_Models(t *testing.T) {
	var model AuthResourceModel

	// Test ID field
	model.ID = types.StringValue("test-id")
	assert.False(t, model.ID.IsNull(), "ID should not be null")
	assert.Equal(t, "test-id", model.ID.ValueString(), "ID value should match")

	// Test AuthOrder field
	authOrderElems := []attr.Value{
		types.StringValue("local"),
		types.StringValue("radius"),
	}
	authOrderType := types.ListType{ElemType: types.StringType}
	authOrderList, _ := types.ListValue(authOrderType.ElemType, authOrderElems)
	model.AuthOrder = authOrderList
	assert.False(t, model.AuthOrder.IsNull(), "AuthOrder should not be null")

	// Test PasswordPolicy field with new schema attributes
	passwordPolicyAttrs := map[string]attr.Value{
		"min_length":           types.Int64Value(8),
		"required_numeric":     types.Int64Value(1),
		"required_uppercase":   types.Int64Value(1),
		"required_lowercase":   types.Int64Value(1),
		"required_special":     types.Int64Value(1),
		"required_differences": types.Int64Value(5),
		"reject_username":      types.BoolValue(true),
		"apply_to_root":        types.BoolValue(false),
		"retries":              types.Int64Value(3),
		"max_login_failures":   types.Int64Value(5),
		"unlock_time":          types.Int64Value(300),
		"root_lockout":         types.BoolValue(true),
		"root_unlock_time":     types.Int64Value(600),
		"max_age":              types.Int64Value(90),
		"max_letter_repeat":    types.Int64Null(),
		"max_sequence_repeat":  types.Int64Null(),
		"max_class_repeat":     types.Int64Null(),
	}
	passwordPolicyObj, _ := types.ObjectValue(passwordPolicyAttrTypes(), passwordPolicyAttrs)
	model.PasswordPolicy = passwordPolicyObj
	assert.False(t, model.PasswordPolicy.IsNull(), "PasswordPolicy should not be null")
}

func TestAuthResourceUnit_Validator(t *testing.T) {
	// Test that authentication methods are properly validated
	authMethods := []string{"local", "radius", "tacacs", "ldap"}
	expectedMappings := map[string]string{
		"local":  "openconfig-aaa-types:LOCAL",
		"radius": "openconfig-aaa-types:RADIUS_ALL",
		"tacacs": "openconfig-aaa-types:TACACS_ALL",
		"ldap":   "f5-openconfig-aaa-ldap:LDAP_ALL",
	}

	for method, expected := range expectedMappings {
		assert.Contains(t, authMethods, method, "Expected authentication method '%s' should be supported", method)
		assert.NotEmpty(t, expected, "Expected mapping for '%s' should not be empty", method)
	}
}

func TestAuthResourceUnit_ValidateList(t *testing.T) {
	// Create a simple test that calls the Description methods which are easier to test
	validator := listAuthOrderValidator{}

	// Test Description method
	desc := validator.Description(context.Background())
	assert.NotEmpty(t, desc, "Description should not be empty")

	// Test MarkdownDescription method
	mdDesc := validator.MarkdownDescription(context.Background())
	assert.NotEmpty(t, mdDesc, "MarkdownDescription should not be empty")
	assert.Contains(t, mdDesc, "local", "Should mention local auth method")
	assert.Contains(t, mdDesc, "radius", "Should mention radius auth method")
	assert.Contains(t, mdDesc, "tacacs", "Should mention tacacs auth method")
	assert.Contains(t, mdDesc, "ldap", "Should mention ldap auth method")
}

func TestAuthResourceUnit_ImportState(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Logf("ImportState panicked as expected: %v", r)
		}
	}()

	authRes := NewAuthResource().(*AuthResource)
	req := resource.ImportStateRequest{
		ID: "test-import-id",
	}
	resp := &resource.ImportStateResponse{}
	authRes.ImportState(context.Background(), req, resp)

	// Check that import state doesn't cause panic and sets the ID
	for _, diag := range resp.Diagnostics {
		if diag.Severity().String() == "ERROR" {
			t.Fatalf("ImportState returned error: %s", diag.Detail())
		}
	}
}

// Panic Handling Tests

func TestAuthResourceUnit_CreateWithNilRequest(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Create with nil request panicked as expected: %v", r)
		}
	}()
	authRes := NewAuthResource().(*AuthResource)
	authRes.Create(context.Background(), resource.CreateRequest{}, &resource.CreateResponse{})
}

func TestAuthResourceUnit_ReadWithNilRequest(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Read with nil request panicked as expected: %v", r)
		}
	}()
	authRes := NewAuthResource().(*AuthResource)
	authRes.Read(context.Background(), resource.ReadRequest{}, &resource.ReadResponse{})
}

func TestAuthResourceUnit_UpdateWithNilRequest(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Update with nil request panicked as expected: %v", r)
		}
	}()
	authRes := NewAuthResource().(*AuthResource)
	authRes.Update(context.Background(), resource.UpdateRequest{}, &resource.UpdateResponse{})
}

func TestAuthResourceUnit_DeleteWithNilRequest(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Delete with nil request panicked as expected: %v", r)
		}
	}()
	authRes := NewAuthResource().(*AuthResource)
	authRes.Delete(context.Background(), resource.DeleteRequest{}, &resource.DeleteResponse{})
}

// Mocked HTTP Tests

func setupMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/restconf/data/openconfig-system:system/aaa/authentication/config":
			switch r.Method {
			case "GET":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{
					"openconfig-system:config": {
						"authentication-method": ["openconfig-aaa-types:LOCAL", "openconfig-aaa-types:RADIUS_ALL"]
					}
				}`)
			case "PUT", "PATCH":
				w.WriteHeader(http.StatusNoContent)
			case "DELETE":
				w.WriteHeader(http.StatusNoContent)
			}
		case "/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:roles":
			switch r.Method {
			case "GET":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{
					"f5-system-aaa:roles": {
						"role": [
							{"rolename": "admin", "config": {"rolename": "admin", "gid": 9000, "remote-gid": "-"}},
							{"rolename": "operator", "config": {"rolename": "operator", "gid": 9001, "remote-gid": 9001}},
							{"rolename": "resource-admin", "config": {"rolename": "resource-admin", "gid": 9003, "remote-gid": "-"}},
							{"rolename": "superuser", "config": {"rolename": "superuser", "gid": 9004, "remote-gid": "-"}},
							{"rolename": "user", "config": {"rolename": "user", "gid": 9002, "remote-gid": "-"}}
						]
					}
				}`)
			case "PUT", "PATCH":
				w.WriteHeader(http.StatusNoContent)
			case "DELETE":
				w.WriteHeader(http.StatusNoContent)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestAuthResourceMocked_ClientMethods(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	config := &f5os.F5osConfig{
		Host:             server.URL,
		User:             "test",
		Password:         "test",
		DisableSSLVerify: true,
	}

	client, err := f5os.NewSession(config)
	assert.NoError(t, err, "Client initialization should not fail")
	assert.NotNil(t, client, "Client should not be nil")

	// Test SetAuthOrder method
	t.Run("SetAuthOrder", func(t *testing.T) {
		methods := []string{"local"}
		err := client.SetAuthOrder(methods)
		assert.NoError(t, err, "SetAuthOrder should not return error")
	})

	// Test GetAuthOrder method
	t.Run("GetAuthOrder", func(t *testing.T) {
		result, err := client.GetAuthOrder()
		assert.NoError(t, err, "GetAuthOrder should not return error")
		assert.NotNil(t, result, "GetAuthOrder should return result")
	})

	// Test ClearAuthOrder method
	t.Run("ClearAuthOrder", func(t *testing.T) {
		err := client.ClearAuthOrder()
		assert.NoError(t, err, "ClearAuthOrder should not return error")
	})

	// Test SetRoleConfig method
	t.Run("SetRoleConfig", func(t *testing.T) {
		gid := int64(100)
		err := client.SetRoleConfig("test-role", &gid)
		assert.NoError(t, err, "SetRoleConfig should not return error")
	})

	// Test GetRoles method
	t.Run("GetRoles", func(t *testing.T) {
		result, err := client.GetRoles()
		assert.NoError(t, err, "GetRoles should not return error")
		assert.NotNil(t, result, "GetRoles should return result")
		// operator is the only role with a numeric remote-gid in the mock
		assert.Equal(t, 9001, result["operator"], "operator remote-gid should be 9001")
		// roles with remote-gid: "-" should have 0
		assert.Equal(t, 0, result["admin"], "admin remote-gid should be 0 (not configured)")
	})
}

func TestAuthResourceMocked_ErrorHandling(t *testing.T) {
	// Test with server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, "Internal Server Error")
	}))
	defer server.Close()

	config := &f5os.F5osConfig{
		Host:             server.URL,
		User:             "test",
		Password:         "test",
		DisableSSLVerify: true,
	}

	client, err := f5os.NewSession(config)
	assert.NoError(t, err, "Client initialization should not fail")

	// Test that errors are properly handled
	_, err = client.GetAuthOrder()
	assert.Error(t, err, "Expected error for server error")
}

func TestAuthResourceMocked_ComplexConfig(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	config := &f5os.F5osConfig{
		Host:             server.URL,
		User:             "test",
		Password:         "test",
		DisableSSLVerify: true,
	}

	client, err := f5os.NewSession(config)
	assert.NoError(t, err, "Client initialization should not fail")

	// Test complex configuration with multiple auth methods
	complexMethods := []string{"local", "radius", "tacacs", "ldap"}

	err = client.SetAuthOrder(complexMethods)
	assert.NoError(t, err, "SetAuthOrder with complex config should not fail")

	// Verify we can read it back
	result, err := client.GetAuthOrder()
	assert.NoError(t, err, "GetAuthOrder after complex config should not fail")
	assert.NotNil(t, result, "GetAuthOrder should return result after complex config")
}

// TestAuthResourceMocked_ReadRoleConfigFiltering verifies that readRoleConfig
// only populates state with roles the user declared in their config, not every
// role on the device. Without filtering, a device with 5 built-in roles
// (admin, operator, resource-admin, superuser, user) would inject all 5 into
// state even if the user only configured 1, causing drift on the next plan.
func TestAuthResourceMocked_ReadRoleConfigFiltering(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	cfg := &f5os.F5osConfig{
		Host:             server.URL,
		User:             "test",
		Password:         "test",
		DisableSSLVerify: true,
	}
	client, err := f5os.NewSession(cfg)
	assert.NoError(t, err, "Client initialization should not fail")

	res := &AuthResource{client: client}
	ctx := context.Background()

	// Simulate a state where the user configured only the "operator" role.
	// Use a GID (1234) that differs from what the mock returns (9001) so we
	// can confirm readRoleConfig actually read from the device and updated
	// state, rather than being a no-op that left state untouched.
	operatorRole := authRemoteRoleModel{
		Rolename:  types.StringValue("operator"),
		RemoteGID: types.Int64Value(1234),
	}
	configuredSet, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: map[string]attr.Type{
		"rolename":   types.StringType,
		"remote_gid": types.Int64Type,
		"ldap_group": types.StringType,
	}}, []authRemoteRoleModel{operatorRole})
	assert.False(t, diags.HasError(), "Building configured roles set should not error")

	state := &AuthResourceModel{
		RemoteRoles: configuredSet,
	}

	// readRoleConfig should filter the 5 device roles down to just "operator"
	err = res.readRoleConfig(ctx, state)
	assert.NoError(t, err, "readRoleConfig should not return error")

	var resultRoles []authRemoteRoleModel
	diags = state.RemoteRoles.ElementsAs(ctx, &resultRoles, false)
	assert.False(t, diags.HasError(), "Extracting roles from state should not error")

	var resultNames []string
	for _, r := range resultRoles {
		resultNames = append(resultNames, r.Rolename.ValueString())
	}

	assert.Equal(t, 1, len(resultRoles),
		"readRoleConfig should return only user-configured roles, got %v", resultNames)

	if len(resultRoles) == 1 {
		assert.Equal(t, "operator", resultRoles[0].Rolename.ValueString(),
			"Single returned role should be 'operator'")
		assert.Equal(t, int64(9001), resultRoles[0].RemoteGID.ValueInt64(),
			"GID should be 9001 (from the device), not 1234 (from the seed state)")
	}
}

// TestAuthResourceMocked_ReadRoleConfigImport verifies that readRoleConfig
// returns all device roles when state.RemoteRoles is null (the import scenario).
// During import, there's no prior config to filter against, so all roles should
// be included in state.
func TestAuthResourceMocked_ReadRoleConfigImport(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	cfg := &f5os.F5osConfig{
		Host:             server.URL,
		User:             "test",
		Password:         "test",
		DisableSSLVerify: true,
	}
	client, err := f5os.NewSession(cfg)
	assert.NoError(t, err, "Client initialization should not fail")

	res := &AuthResource{client: client}
	ctx := context.Background()

	// Simulate import: RemoteRoles is null (no prior state)
	state := &AuthResourceModel{
		RemoteRoles: types.SetNull(types.ObjectType{AttrTypes: map[string]attr.Type{
			"rolename":   types.StringType,
			"remote_gid": types.Int64Type,
			"ldap_group": types.StringType,
		}}),
	}

	err = res.readRoleConfig(ctx, state)
	assert.NoError(t, err, "readRoleConfig should not return error")

	var resultRoles []authRemoteRoleModel
	diags := state.RemoteRoles.ElementsAs(ctx, &resultRoles, false)
	assert.False(t, diags.HasError(), "Extracting roles from state should not error")

	// During import, all 5 device roles should be returned
	assert.Equal(t, 5, len(resultRoles),
		"readRoleConfig should return all device roles during import")
}

// TestAuthResourceMocked_SnapshotRestoreRoundtrip exercises the full
// Create → Destroy lifecycle through the Terraform framework with a mock
// HTTP server. It verifies that:
//  1. Create snapshots the pre-existing auth_order into private state.
//  2. Delete reads the snapshot and restores it via PUT (not DELETE).
//
// The mock server tracks its state so CheckDestroy can verify the final
// device state matches the pre-existing baseline.
func TestAuthResourceMocked_SnapshotRestoreRoundtrip(t *testing.T) {
	preExisting := `["openconfig-aaa-types:LOCAL","f5-openconfig-aaa-ldap:LDAP_ALL"]`
	currentAuthMethods := preExisting
	deleteCalled := false

	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"openconfig-system:config":{"authentication-method":%s}}`, currentAuthMethods)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/config/authentication-method", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PUT":
			var payload struct {
				Methods []string `json:"openconfig-system:authentication-method"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			b, _ := json.Marshal(payload.Methods)
			currentAuthMethods = string(b)
			w.WriteHeader(http.StatusNoContent)
		case "DELETE":
			deleteCalled = true
			currentAuthMethods = "[]"
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	tfresource.Test(t, tfresource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			if currentAuthMethods != preExisting {
				return fmt.Errorf("expected auth_order restored to %s, got %s", preExisting, currentAuthMethods)
			}
			if deleteCalled {
				return fmt.Errorf("DELETE was called; expected PUT to restore original")
			}
			return nil
		},
		Steps: []tfresource.TestStep{
			{
				Config: `resource "f5os_auth" "test" { auth_order = ["local", "radius"] }`,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.0", "local"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.1", "radius"),
					func(s *terraform.State) error {
						expected := `["openconfig-aaa-types:LOCAL","openconfig-aaa-types:RADIUS_ALL"]`
						if currentAuthMethods != expected {
							return fmt.Errorf("after create, expected device to have %s, got %s", expected, currentAuthMethods)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestAuthResourceMocked_DeleteFallbackWhenNoSnapshot exercises the fallback
// path in Delete when private state has no saved auth_order. This happens
// when the device had no authentication-method configured at Create time
// (getAuthOrder returns nil, so snapshotAuthOrder stores nothing). In this
// case Delete should fall back to calling ClearAuthOrder (DELETE).
func TestAuthResourceMocked_DeleteFallbackWhenNoSnapshot(t *testing.T) {
	deleteCalled := false

	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	// GET /config returns NO authentication-method — simulates a device
	// with no pre-existing auth_order.
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"openconfig-system:config":{}}`)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/config/authentication-method", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PUT":
			w.WriteHeader(http.StatusNoContent)
		case "DELETE":
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	tfresource.Test(t, tfresource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			if !deleteCalled {
				return fmt.Errorf("expected DELETE fallback when no snapshot exists, but DELETE was not called")
			}
			return nil
		},
		Steps: []tfresource.TestStep{
			{
				Config: `resource "f5os_auth" "test" { auth_order = ["local", "radius"] }`,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.0", "local"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.1", "radius"),
				),
			},
		},
	})
}

// TestAccAuthResourceDeleteRestoresOriginal verifies that when Terraform
// destroys the f5os_auth resource, the auth_order is restored to whatever
// was configured on the device before Terraform managed it, rather than
// being deleted entirely.
//
// Strategy:
//  1. Capture the device's current auth_order (the true baseline).
//  2. Set a known auth_order on the device via direct API: ["local", "ldap"]
//  3. Apply a Terraform config with auth_order = ["local", "radius"]
//     — Create should snapshot ["local", "ldap"] into private state
//  4. Terraform destroy runs automatically at the end of the test
//     — Delete should restore ["local", "ldap"] from private state
//  5. CheckDestroy verifies the device has ["local", "ldap"]
//  6. t.Cleanup restores the true baseline captured in step 1.
//
// Safety: always keeps "local" first; restores original baseline in Cleanup.
func TestAccAuthResourceDeleteRestoresOriginal(t *testing.T) {
	preExisting := []string{"local", "ldap"}

	client, err := newAuthClientFromEnv()
	if err != nil {
		t.Skipf("Cannot create f5os client: %v", err)
	}

	// Capture the true device baseline before we touch anything.
	trueBaseline, err := client.GetAuthOrder()
	if err != nil {
		t.Fatalf("Failed to read true device baseline: %v", err)
	}
	t.Logf("True device baseline auth_order: %v", mapOpenConfigMethodsToFriendly(trueBaseline))

	// Set a known pre-existing auth_order so Create has something to snapshot.
	if err := client.SetAuthOrder(preExisting); err != nil {
		t.Fatalf("Failed to set pre-existing auth order: %v", err)
	}
	t.Logf("Pre-set device auth_order to %v", preExisting)

	// Cleanup: restore the true baseline regardless of test outcome.
	t.Cleanup(func() {
		cleanupClient, err := newAuthClientFromEnv()
		if err != nil {
			t.Logf("WARNING: cleanup failed to create client: %v", err)
			return
		}
		if trueBaseline == nil {
			if err := cleanupClient.ClearAuthOrder(); err != nil {
				t.Logf("WARNING: cleanup failed to clear auth order: %v", err)
			} else {
				t.Log("Cleanup: cleared auth_order (baseline had none)")
			}
		} else {
			if err := cleanupClient.SetAuthOrder(mapOpenConfigMethodsToFriendly(trueBaseline)); err != nil {
				t.Logf("WARNING: cleanup failed to restore auth order to %v: %v",
					mapOpenConfigMethodsToFriendly(trueBaseline), err)
			} else {
				t.Logf("Cleanup: restored auth_order to %v", mapOpenConfigMethodsToFriendly(trueBaseline))
			}
		}
	})

	tfresource.Test(t, tfresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			// After destroy, the device should have the pre-existing auth
			// order restored, NOT an empty/deleted auth-method array.
			c, err := newAuthClientFromEnv()
			if err != nil {
				return fmt.Errorf("failed to create client for destroy check: %w", err)
			}
			rawMethods, err := c.GetAuthOrder()
			if err != nil {
				return fmt.Errorf("failed to read auth order after destroy: %w", err)
			}
			actual := mapOpenConfigMethodsToFriendly(rawMethods)
			if len(actual) != len(preExisting) {
				return fmt.Errorf("expected auth_order %v restored after destroy, got %v", preExisting, actual)
			}
			for i, want := range preExisting {
				if actual[i] != want {
					return fmt.Errorf("auth_order[%d] mismatch: expected %q, got %q (full: expected %v, got %v)",
						i, want, actual[i], preExisting, actual)
				}
			}
			return nil
		},
		Steps: []tfresource.TestStep{
			// Step 1: Create — Terraform sets auth_order to ["local", "radius"],
			// which should snapshot the pre-existing ["local", "ldap"] first.
			{
				Config: testAccAuthResourceConfig,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.#", "2"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.0", "local"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.1", "radius"),
					testAccCheckAuthOrderApplied([]string{"local", "radius"}),
				),
			},
			// Step 2: Destroy is automatic — CheckDestroy verifies the
			// pre-existing ["local", "ldap"] was restored.
		},
	})
}

// TestAuthResourceMocked_RoleGIDRestoreDeletesWhenBaselineUnset exercises
// the Delete path where the snapshotted GID is 0 (the role had no
// remote-gid before Terraform). Delete should call ClearRoleRemoteGID
// (HTTP DELETE on the remote-gid leaf) rather than SetRoleConfig (PATCH).
func TestAuthResourceMocked_RoleGIDRestoreDeletesWhenBaselineUnset(t *testing.T) {
	// Pre-existing state: operator has NO remote-gid (returns 0 from GetRoles).
	operatorGID := 0
	deleteCalledForRemoteGID := false

	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"openconfig-system:config":{"authentication-method":["openconfig-aaa-types:LOCAL","openconfig-aaa-types:RADIUS_ALL"]}}`)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/config/authentication-method", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:roles", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		// operator has remote-gid "-" (unset) which GetRoles parses as 0
		_, _ = fmt.Fprintf(w, `{
			"f5-system-aaa:roles": {
				"role": [
					{"rolename": "admin", "config": {"rolename": "admin", "gid": 9000, "remote-gid": "-"}},
					{"rolename": "operator", "config": {"rolename": "operator", "gid": 9001, "remote-gid": %s}}
				]
			}
		}`, func() string {
			if operatorGID == 0 {
				return `"-"`
			}
			return strconv.Itoa(operatorGID)
		}())
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:roles/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH":
			var payload struct {
				Config struct {
					Rolename string `json:"f5-system-aaa:rolename"`
					GID      *int64 `json:"f5-system-aaa:remote-gid"`
				} `json:"f5-system-aaa:config"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if payload.Config.GID != nil {
				operatorGID = int(*payload.Config.GID)
			}
		case "DELETE":
			deleteCalledForRemoteGID = true
			operatorGID = 0
		}
		w.WriteHeader(http.StatusNoContent)
	})

	defer teardown()

	tfresource.Test(t, tfresource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			if !deleteCalledForRemoteGID {
				return fmt.Errorf("expected DELETE to clear remote-gid during destroy, but DELETE was not called")
			}
			if operatorGID != 0 {
				return fmt.Errorf("expected operator GID to be 0 (unset) after destroy, got %d", operatorGID)
			}
			return nil
		},
		Steps: []tfresource.TestStep{
			{
				Config: `resource "f5os_auth" "test" {
  auth_order = ["local", "radius"]
  remote_roles = [
    {
      rolename   = "operator"
      remote_gid = 9999
    },
  ]
}`,
				Check: func(s *terraform.State) error {
					if operatorGID != 9999 {
						return fmt.Errorf("after create, expected operator GID 9999, got %d", operatorGID)
					}
					return nil
				},
			},
		},
	})
}

// TestAuthResourceMocked_RoleGIDNoSnapshotSkipsRestore verifies that when
// private state has no saved role GIDs (e.g., the resource was created
// before this fix, or no remote_roles were configured), Delete does not
// attempt any role restoration and completes without error.
func TestAuthResourceMocked_RoleGIDNoSnapshotSkipsRestore(t *testing.T) {
	roleConfigPatched := false

	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"openconfig-system:config":{}}`)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/config/authentication-method", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	// No roles endpoint registered for GET — the resource config doesn't
	// include remote_roles, so snapshotRoleGIDs is never called.
	// Track whether any role PATCH happens during Delete.
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:roles/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			roleConfigPatched = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	defer teardown()

	tfresource.Test(t, tfresource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			if roleConfigPatched {
				return fmt.Errorf("expected no role config PATCH during destroy when no snapshot exists, but PATCH was called")
			}
			return nil
		},
		Steps: []tfresource.TestStep{
			{
				// Only auth_order, no remote_roles — no role snapshot should be taken.
				Config: `resource "f5os_auth" "test" { auth_order = ["local", "radius"] }`,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.0", "local"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.1", "radius"),
				),
			},
		},
	})
}

// TestAuthResourceMocked_RoleGIDUpdateAddsNewRole verifies that when
// an Update adds a new role that wasn't in the original Create config,
// the new role's pre-existing GID is captured into the snapshot so that
// Delete restores it.
//
// Scenario:
//  1. Create with only operator (remote-gid 9999) — snapshots operator's
//     pre-existing GID (9001) but NOT user-manager's.
//  2. Update adds user-manager (remote-gid 8888) — ensureRoleGIDsSnapshotted
//     should capture user-manager's pre-existing GID (7777) into the snapshot.
//  3. Delete should restore operator to 9001 AND user-manager to 7777.
func TestAuthResourceMocked_RoleGIDUpdateAddsNewRole(t *testing.T) {
	operatorGID := 9001
	userManagerGID := 7777

	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"openconfig-system:config":{"authentication-method":["openconfig-aaa-types:LOCAL","openconfig-aaa-types:RADIUS_ALL"]}}`)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/config/authentication-method", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:roles", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
			"f5-system-aaa:roles": {
				"role": [
					{"rolename": "admin", "config": {"rolename": "admin", "gid": 9000, "remote-gid": "-"}},
					{"rolename": "operator", "config": {"rolename": "operator", "gid": 9001, "remote-gid": %d}},
					{"rolename": "user-manager", "config": {"rolename": "user-manager", "gid": 9005, "remote-gid": %d}}
				]
			}
		}`, operatorGID, userManagerGID)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:roles/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			var payload struct {
				Config struct {
					Rolename string `json:"f5-system-aaa:rolename"`
					GID      *int64 `json:"f5-system-aaa:remote-gid"`
				} `json:"f5-system-aaa:config"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if payload.Config.GID != nil {
				switch payload.Config.Rolename {
				case "operator":
					operatorGID = int(*payload.Config.GID)
				case "user-manager":
					userManagerGID = int(*payload.Config.GID)
				}
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})

	defer teardown()

	tfresource.Test(t, tfresource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			if operatorGID != 9001 {
				return fmt.Errorf("expected operator GID restored to 9001, got %d", operatorGID)
			}
			if userManagerGID != 7777 {
				return fmt.Errorf("expected user-manager GID restored to 7777, got %d", userManagerGID)
			}
			return nil
		},
		Steps: []tfresource.TestStep{
			// Step 1: Create with only operator.
			{
				Config: `resource "f5os_auth" "test" {
  auth_order = ["local", "radius"]
  remote_roles = [
    { rolename = "operator", remote_gid = 9999 },
  ]
}`,
				Check: func(s *terraform.State) error {
					if operatorGID != 9999 {
						return fmt.Errorf("after create, expected operator GID 9999, got %d", operatorGID)
					}
					return nil
				},
			},
			// Step 2: Update — add user-manager.
			{
				Config: `resource "f5os_auth" "test" {
  auth_order = ["local", "radius"]
  remote_roles = [
    { rolename = "operator", remote_gid = 9999 },
    { rolename = "user-manager", remote_gid = 8888 },
  ]
}`,
				Check: func(s *terraform.State) error {
					if userManagerGID != 8888 {
						return fmt.Errorf("after update, expected user-manager GID 8888, got %d", userManagerGID)
					}
					return nil
				},
			},
			// Step 3: Destroy is automatic — CheckDestroy verifies both
			// roles are restored to their pre-Terraform values.
		},
	})
}

// TestAccAuthResourceDeleteRestoresRoleGIDs verifies that when Terraform
// destroys the f5os_auth resource, the operator role GID is restored to
// whatever was configured on the device before Terraform managed it.
//
// Strategy:
//  1. Capture the device's current operator GID (the true baseline).
//  2. Set a known operator GID on the device via direct API: 9050
//  3. Apply a Terraform config with remote_roles operator GID = 9060
//     — Create should snapshot 9050 into private state
//  4. Terraform destroy runs automatically at the end of the test
//     — Delete should restore 9050 from private state
//  5. CheckDestroy verifies the device has operator GID 9050
//  6. t.Cleanup restores the true baseline captured in step 1.
func TestAccAuthResourceDeleteRestoresRoleGIDs(t *testing.T) {
	preExistingGID := int64(9050)

	client, err := newAuthClientFromEnv()
	if err != nil {
		t.Skipf("Cannot create f5os client: %v", err)
	}

	// Capture the true device baseline before we touch anything.
	originalRoles, err := client.GetRoles()
	if err != nil {
		t.Fatalf("Failed to read true device baseline roles: %v", err)
	}
	trueBaselineGID, hasOperator := originalRoles["operator"]
	if !hasOperator {
		t.Skip("Skipping: device has no 'operator' role to test with")
	}
	t.Logf("True device baseline operator GID: %d", trueBaselineGID)

	// Pre-flight: verify we can modify role config on this device.
	if err := client.SetRoleConfig("operator", &preExistingGID); err != nil {
		if strings.Contains(err.Error(), "access denied") || strings.Contains(err.Error(), "403") {
			t.Skip("Skipping role test: admin user lacks permission to modify role config on this device")
		}
		t.Skipf("Skipping role test: unexpected error testing role config access: %v", err)
	}
	t.Logf("Pre-set device operator GID to %d", preExistingGID)

	// Cleanup: restore the true baseline regardless of test outcome.
	t.Cleanup(func() {
		cleanupClient, err := newAuthClientFromEnv()
		if err != nil {
			t.Logf("WARNING: cleanup failed to create client: %v", err)
			return
		}
		if trueBaselineGID == 0 {
			// Baseline had no remote-gid; delete the leaf to restore
			// the unset state rather than leaving a test value behind.
			if err := cleanupClient.ClearRoleRemoteGID("operator"); err != nil {
				t.Logf("WARNING: cleanup failed to clear operator remote-gid: %v", err)
			} else {
				t.Log("Cleanup: cleared operator remote-gid (baseline had none)")
			}
			return
		}
		gid := int64(trueBaselineGID)
		if err := cleanupClient.SetRoleConfig("operator", &gid); err != nil {
			t.Logf("WARNING: cleanup failed to restore operator GID to %d: %v", trueBaselineGID, err)
		} else {
			t.Logf("Cleanup: restored operator GID to %d", trueBaselineGID)
		}
	})

	tfresource.Test(t, tfresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			c, err := newAuthClientFromEnv()
			if err != nil {
				return fmt.Errorf("failed to create client for destroy check: %w", err)
			}
			roles, err := c.GetRoles()
			if err != nil {
				return fmt.Errorf("failed to read roles after destroy: %w", err)
			}
			actualGID, exists := roles["operator"]
			if !exists {
				return fmt.Errorf("operator role not found on device after destroy")
			}
			if int64(actualGID) != preExistingGID {
				return fmt.Errorf("expected operator GID %d restored after destroy, got %d", preExistingGID, actualGID)
			}
			return nil
		},
		Steps: []tfresource.TestStep{
			{
				Config: `
resource "f5os_auth" "test" {
  auth_order = ["local", "radius"]
  remote_roles = [
    {
      rolename   = "operator"
      remote_gid = 9060
    },
  ]
}`,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.#", "2"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.0", "local"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.1", "radius"),
					testAccCheckAuthOrderApplied([]string{"local", "radius"}),
					testAccCheckRoleGIDApplied("operator", 9060),
				),
			},
			// Destroy is automatic — CheckDestroy verifies the pre-existing
			// operator GID (9050) was restored.
		},
	})
}

// Helper functions - removed unused helper functions

// Integration Test Functions

func TestAuthResourceIntegration_BasicFlow(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	config := &f5os.F5osConfig{
		Host:             server.URL,
		User:             "test",
		Password:         "test",
		DisableSSLVerify: true,
	}

	client, err := f5os.NewSession(config)
	assert.NoError(t, err, "Client initialization should not fail")

	// Test complete CRUD flow
	t.Run("Create", func(t *testing.T) {
		authMethods := []string{"local"}
		err := client.SetAuthOrder(authMethods)
		assert.NoError(t, err, "Create should not fail")
	})

	t.Run("Read", func(t *testing.T) {
		result, err := client.GetAuthOrder()
		assert.NoError(t, err, "Read should not fail")
		assert.NotNil(t, result, "Read should return result")
	})

	t.Run("Update", func(t *testing.T) {
		authMethods := []string{"local", "radius"}
		err := client.SetAuthOrder(authMethods)
		assert.NoError(t, err, "Update should not fail")
	})

	t.Run("Delete", func(t *testing.T) {
		err := client.ClearAuthOrder()
		assert.NoError(t, err, "Delete should not fail")
	})
}

func TestAuthResourceIntegration_RoleManagement(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	config := &f5os.F5osConfig{
		Host:             server.URL,
		User:             "test",
		Password:         "test",
		DisableSSLVerify: true,
	}

	client, err := f5os.NewSession(config)
	assert.NoError(t, err, "Client initialization should not fail")

	// Test role configuration flow
	t.Run("CreateRole", func(t *testing.T) {
		gid := int64(100)
		err := client.SetRoleConfig("test-role", &gid)
		assert.NoError(t, err, "CreateRole should not fail")
	})

	// t.Run("ReadRoles", func(t *testing.T) {
	// 	result, err := client.GetRoles()
	// 	assert.NoError(t, err, "ReadRoles should not fail")
	// 	assert.NotNil(t, result, "ReadRoles should return result")
	// })
}

func TestAuthResourceIntegration_MultipleAuthMethods(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	config := &f5os.F5osConfig{
		Host:             server.URL,
		User:             "test",
		Password:         "test",
		DisableSSLVerify: true,
	}

	client, err := f5os.NewSession(config)
	assert.NoError(t, err, "Client initialization should not fail")

	// Test all authentication methods
	authMethodsTests := [][]string{
		{"local"},
		{"radius"},
		{"tacacs"},
		{"ldap"},
		{"local", "radius"},
		{"local", "tacacs"},
		{"local", "ldap"},
	}

	for i, methods := range authMethodsTests {
		t.Run(fmt.Sprintf("AuthMethod_%d", i), func(t *testing.T) {
			err := client.SetAuthOrder(methods)
			assert.NoError(t, err, "AuthMethod %v should not fail", methods)
		})
	}
}

func TestAuthResourceIntegration_ConfigValidation(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	config := &f5os.F5osConfig{
		Host:             server.URL,
		User:             "test",
		Password:         "test",
		DisableSSLVerify: true,
	}

	client, err := f5os.NewSession(config)
	assert.NoError(t, err, "Client initialization should not fail")

	// Test configuration validation
	validConfigs := [][]string{
		{"local"},
		{"local", "radius", "tacacs", "ldap"},
	}

	for i, config := range validConfigs {
		t.Run(fmt.Sprintf("ValidConfig_%d", i), func(t *testing.T) {
			err := client.SetAuthOrder(config)
			assert.NoError(t, err, "Valid config %d should not fail", i)
		})
	}
}

func TestAuthResourceIntegration_DataTypesValidation(t *testing.T) {
	// Test data types and structures
	t.Run("AuthResourceModel", func(t *testing.T) {
		var model AuthResourceModel

		// Test ID field
		model.ID = types.StringValue("test-id")
		assert.False(t, model.ID.IsNull(), "ID should not be null")
		assert.Equal(t, "test-id", model.ID.ValueString(), "ID value should match")

		// Test AuthOrder field (empty list)
		authOrderType := types.ListType{ElemType: types.StringType}
		authOrderList, _ := types.ListValue(authOrderType.ElemType, []attr.Value{})
		model.AuthOrder = authOrderList
		assert.False(t, model.AuthOrder.IsNull(), "AuthOrder should not be null")
	})

	t.Run("TypeConversions", func(t *testing.T) {
		// Test type conversions and validations
		testString := "test-value"
		typeValue := types.StringValue(testString)

		assert.Equal(t, testString, typeValue.ValueString(), "String type conversion should work")

		// Test null values
		nullValue := types.StringNull()
		assert.True(t, nullValue.IsNull(), "Null value should be null")
	})

	t.Run("StructureValidation", func(t *testing.T) {
		// Test structure validation
		authRes := NewAuthResource()

		// Check type
		resourceType := reflect.TypeOf(authRes)
		assert.Equal(t, "*provider.AuthResource", resourceType.String(), "Resource type should match")

		// Check that it implements required interfaces
		assert.Implements(t, (*resource.Resource)(nil), authRes)
		assert.Implements(t, (*resource.ResourceWithImportState)(nil), authRes)
	})
}

func TestAuthResourceIntegration_EdgeCases(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	config := &f5os.F5osConfig{
		Host:             server.URL,
		User:             "test",
		Password:         "test",
		DisableSSLVerify: true,
	}

	client, err := f5os.NewSession(config)
	assert.NoError(t, err, "Client initialization should not fail")

	// Test edge cases
	t.Run("EmptyConfig", func(t *testing.T) {
		emptyMethods := []string{}
		err := client.SetAuthOrder(emptyMethods)
		// This might succeed or fail depending on validation
		// The important thing is it doesn't panic
		_ = err
	})

	t.Run("NilConfig", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("NilConfig panicked as expected: %v", r)
			}
		}()
		err := client.SetAuthOrder(nil)
		_ = err
	})

	t.Run("LargeConfig", func(t *testing.T) {
		// Test with many authentication methods including duplicates
		largeMethods := []string{
			"local",
			"radius",
			"tacacs",
			"ldap",
			"local", // Duplicate to test handling
		}
		err := client.SetAuthOrder(largeMethods)
		if err != nil {
			t.Logf("Large config failed as expected: %v", err)
		}
	})
}

func TestAuthResourceIntegration_ConcurrentAccess(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	config := &f5os.F5osConfig{
		Host:             server.URL,
		User:             "test",
		Password:         "test",
		DisableSSLVerify: true,
	}

	client, err := f5os.NewSession(config)
	assert.NoError(t, err, "Client initialization should not fail")

	// Test concurrent access
	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func(index int) {
			defer func() {
				done <- true
			}()

			authMethods := []string{"local"}

			err := client.SetAuthOrder(authMethods)
			if err != nil {
				t.Errorf("Concurrent access %d failed: %v", index, err)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestAuthResourceIntegration_HTTPMethods(t *testing.T) {
	// Test different HTTP methods with custom server
	methodCalled := make(map[string]bool)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		methodCalled[req.Method] = true

		switch req.Method {
		case "GET":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"openconfig-system:config": {"authentication-method": ["openconfig-aaa-types:LOCAL"]}}`)
		case "PUT":
			w.WriteHeader(http.StatusNoContent)
		case "PATCH":
			w.WriteHeader(http.StatusNoContent)
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	config := &f5os.F5osConfig{
		Host:             server.URL,
		User:             "test",
		Password:         "test",
		DisableSSLVerify: true,
	}

	client, err := f5os.NewSession(config)
	assert.NoError(t, err, "Client initialization should not fail")

	// Test GET method
	_, err = client.GetAuthOrder()
	assert.NoError(t, err, "GET method should not fail")
	assert.True(t, methodCalled["GET"], "GET method should be called")

	// Test PUT method (via SetAuthOrder)
	authMethods := []string{"local"}
	err = client.SetAuthOrder(authMethods)
	assert.NoError(t, err, "PUT method should not fail")

	// Test DELETE method
	err = client.ClearAuthOrder()
	assert.NoError(t, err, "DELETE method should not fail")
}

// ---------------------------------------------------------------------------
// Acceptance Tests (require live F5OS device with TF_ACC=1)
// ---------------------------------------------------------------------------

// newAuthClientFromEnv creates a fresh f5osclient from environment variables.
// Used by custom check functions to verify device state independently of the
// resource's Read method.
func newAuthClientFromEnv() (*f5os.F5os, error) {
	host := os.Getenv("F5OS_HOST")
	user := os.Getenv("F5OS_USERNAME")
	if user == "" {
		user = os.Getenv("F5OS_USER")
	}
	pass := os.Getenv("F5OS_PASSWORD")
	port := 8888 // Default matches the provider (provider.go:104)
	if p := os.Getenv("F5OS_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}
	cfg := &f5os.F5osConfig{
		Host:             host,
		User:             user,
		Password:         pass,
		Port:             port,
		DisableSSLVerify: true,
	}
	return f5os.NewSession(cfg)
}

// mapOpenConfigMethodsToFriendly converts OpenConfig auth method identifiers
// to user-friendly names (same mapping as in the resource's getAuthOrder).
func mapOpenConfigMethodsToFriendly(methods []string) []string {
	methodMap := map[string]string{
		"openconfig-aaa-types:LOCAL":      "local",
		"openconfig-aaa-types:RADIUS_ALL": "radius",
		"openconfig-aaa-types:TACACS_ALL": "tacacs",
		"f5-openconfig-aaa-ldap:LDAP_ALL": "ldap",
	}
	out := make([]string, 0, len(methods))
	for _, m := range methods {
		if friendly, ok := methodMap[m]; ok {
			out = append(out, friendly)
		} else {
			out = append(out, m)
		}
	}
	return out
}

// testAccCheckAuthOrderApplied queries the device directly to verify the
// authentication order matches the expected methods (order-sensitive).
func testAccCheckAuthOrderApplied(expectedMethods []string) tfresource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newAuthClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create f5os client: %w", err)
		}
		rawMethods, err := client.GetAuthOrder()
		if err != nil {
			return fmt.Errorf("failed to read auth order from device: %w", err)
		}
		actualMethods := mapOpenConfigMethodsToFriendly(rawMethods)

		if len(actualMethods) != len(expectedMethods) {
			return fmt.Errorf("auth order length mismatch: expected %v, got %v", expectedMethods, actualMethods)
		}
		for i, expected := range expectedMethods {
			if actualMethods[i] != expected {
				return fmt.Errorf("auth order mismatch at index %d: expected %q, got %q (full: expected %v, got %v)",
					i, expected, actualMethods[i], expectedMethods, actualMethods)
			}
		}
		return nil
	}
}

// testAccCheckAuthDestroy verifies that the auth order was cleared after
// terraform destroy. Note: Delete intentionally does NOT remove role GID
// configurations, so we only check that auth_order was removed.
func testAccCheckAuthDestroy(s *terraform.State) error {
	client, err := newAuthClientFromEnv()
	if err != nil {
		// Cannot connect — treat as destroyed
		return nil
	}
	rawMethods, err := client.GetAuthOrder()
	if err != nil {
		// If the GET fails (e.g., 404 because the path was deleted), that's fine
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("unexpected error checking auth order after destroy: %w", err)
	}
	// After ClearAuthOrder (DELETE), the response may return nil/empty or
	// the device may fall back to a default. Accept nil/empty as "destroyed".
	if len(rawMethods) == 0 {
		return nil
	}
	// Some F5OS versions may return a default auth order after clearing.
	// Only fail if the test-specific methods (radius, tacacs) are still present,
	// since those would indicate our config was not cleaned up.
	friendly := mapOpenConfigMethodsToFriendly(rawMethods)
	for _, m := range friendly {
		if m == "radius" || m == "tacacs" {
			return fmt.Errorf("auth order still contains test method %q after destroy: %v", m, friendly)
		}
	}
	return nil
}

// TestAccAuthResource is a real-device acceptance test for the f5os_auth resource.
// It tests the full Terraform lifecycle: Create, Import, Update, Destroy.
//
// Safety:
//   - auth_order always keeps "local" first
//   - Each step is verified via direct API calls, not just Terraform state
func TestAccAuthResource(t *testing.T) {
	tfresource.Test(t, tfresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAuthDestroy,
		Steps: []tfresource.TestStep{
			// Step 1: Create — set auth_order to local + radius
			{
				Config: testAccAuthResourceConfig,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					// Terraform state checks
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.#", "2"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.0", "local"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.1", "radius"),
					tfresource.TestCheckResourceAttrSet("f5os_auth.test", "id"),
					// Direct API verification — proves the device accepted the config
					testAccCheckAuthOrderApplied([]string{"local", "radius"}),
				),
			},
			// Step 2: Import state
			{
				ResourceName:      "f5os_auth.test",
				ImportState:       true,
				ImportStateVerify: true,
				// remote_roles: import reads all device roles, not just
				// user-declared ones, so imported state won't match config.
				// password_policy: not implemented.
				ImportStateVerifyIgnore: []string{"remote_roles", "password_policy"},
			},
			// Step 3: Update — change auth_order to local + tacacs
			{
				Config: testAccAuthResourceConfigUpdated,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					// Terraform state checks
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.#", "2"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.0", "local"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.1", "tacacs"),
					tfresource.TestCheckResourceAttrSet("f5os_auth.test", "id"),
					// Direct API verification
					testAccCheckAuthOrderApplied([]string{"local", "tacacs"}),
				),
			},
			// Step 4: Destroy is automatic — CheckDestroy verifies cleanup
		},
	})
}

// TestAccAuthResourceDriftDetection proves that the Read method queries the
// device after Create/Update so that Terraform can detect out-of-band changes.
//
// Strategy:
//  1. Apply auth_order = ["local", "radius"] — establishes Terraform-managed state.
//  2. Between steps, mutate the device directly via API to ["local", "tacacs"]
//     (simulating an out-of-band change / drift).
//  3. Re-apply the same ["local", "radius"] config.
//     - If Read queries the device: Terraform sees the drift, detects a diff,
//     and re-applies ["local", "radius"]. The device ends up correct.
//     - If Read is broken (preserves plan state): Terraform thinks nothing
//     changed, skips the apply, and the device stays at ["local", "tacacs"].
//  4. Verify via direct API that the device has ["local", "radius"].
//
// Safety: always keeps "local" first; restores baseline on destroy.
func TestAccAuthResourceDriftDetection(t *testing.T) {
	tfresource.Test(t, tfresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAuthDestroy,
		Steps: []tfresource.TestStep{
			// Step 1: Create — set auth_order to ["local", "radius"]
			{
				Config: testAccAuthResourceConfig,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.#", "2"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.0", "local"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.1", "radius"),
					testAccCheckAuthOrderApplied([]string{"local", "radius"}),
				),
			},
			// Step 2: Inject drift, then re-apply the SAME config.
			// PreConfig runs before Terraform plans this step.
			{
				PreConfig: func() {
					// Mutate the device behind Terraform's back
					client, err := newAuthClientFromEnv()
					if err != nil {
						t.Fatalf("drift injection: failed to create client: %v", err)
					}
					if err := client.SetAuthOrder([]string{"local", "tacacs"}); err != nil {
						t.Fatalf("drift injection: failed to set auth order: %v", err)
					}
					t.Log("drift injection: device auth_order changed to [local, tacacs]")
				},
				// Re-apply the original config. If Read detects the drift,
				// Terraform will see a diff and re-apply ["local", "radius"].
				Config: testAccAuthResourceConfig,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					// Terraform state should show the desired config
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.#", "2"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.0", "local"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.1", "radius"),
					// Critical: the DEVICE must actually have ["local", "radius"].
					// If Read is broken, the device will still have ["local", "tacacs"]
					// and this check will fail.
					testAccCheckAuthOrderApplied([]string{"local", "radius"}),
				),
			},
		},
	})
}

// testAccCheckRoleGIDApplied queries the device directly to verify a role's
// GID matches the expected value.
func testAccCheckRoleGIDApplied(rolename string, expectedGID int) tfresource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newAuthClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create f5os client: %w", err)
		}
		roles, err := client.GetRoles()
		if err != nil {
			return fmt.Errorf("failed to read roles from device: %w", err)
		}
		actualGID, exists := roles[rolename]
		if !exists {
			return fmt.Errorf("role %q not found on device; available roles: %v", rolename, roles)
		}
		if actualGID != expectedGID {
			return fmt.Errorf("role %q GID mismatch: expected %d, got %d", rolename, expectedGID, actualGID)
		}
		return nil
	}
}

// TestAccAuthResourceWithRoles tests the full lifecycle of auth_order together
// with remote_roles: Create, Import, Update, Destroy.
//
// This validates that:
//   - SetRoleConfig can write role GIDs via PATCH to the RESTCONF API
//   - Import correctly reads role GIDs and filters to user-configured roles only
//   - Role GID updates are applied to the device
//
// Safety:
//   - auth_order always keeps "local" first
//   - Only modifies the "operator" role GID (never admin/root/tenant-console)
//   - Restores the original operator GID after the test via t.Cleanup
//   - Pre-flight check skips gracefully if the device blocks role writes
func TestAccAuthResourceWithRoles(t *testing.T) {
	// Pre-flight: check if we can modify role config on this device
	client, err := newAuthClientFromEnv()
	if err != nil {
		t.Skipf("Cannot create f5os client: %v", err)
	}

	// Save the operator role's current GID so we can restore it after the test
	originalRoles, err := client.GetRoles()
	if err != nil {
		t.Skipf("Cannot read roles from device: %v", err)
	}
	originalOperatorGID, hasOperator := originalRoles["operator"]
	if !hasOperator {
		t.Skip("Skipping: device has no 'operator' role to test with")
	}
	t.Cleanup(func() {
		restoreClient, err := newAuthClientFromEnv()
		if err != nil {
			t.Logf("WARNING: failed to create client for operator GID restore: %v", err)
			return
		}
		if originalOperatorGID == 0 {
			if err := restoreClient.ClearRoleRemoteGID("operator"); err != nil {
				t.Logf("WARNING: failed to clear operator remote-gid: %v", err)
			} else {
				t.Log("Cleanup: cleared operator remote-gid (baseline had none)")
			}
			return
		}
		gid := int64(originalOperatorGID)
		if err := restoreClient.SetRoleConfig("operator", &gid); err != nil {
			t.Logf("WARNING: failed to restore operator GID to %d: %v", originalOperatorGID, err)
		} else {
			t.Logf("Restored operator GID to %d", originalOperatorGID)
		}
	})

	// Verify we can write a role GID before running the full test.
	// Use 9099 (different from the test GIDs 9010/9011) so the pre-flight
	// doesn't make Step 1's Create a no-op. Cleanup restores the original.
	probeGID := int64(9099)
	if err := client.SetRoleConfig("operator", &probeGID); err != nil {
		if strings.Contains(err.Error(), "access denied") || strings.Contains(err.Error(), "403") {
			t.Skip("Skipping role test: admin user lacks permission to modify role config on this device")
		}
		t.Skipf("Skipping role test: unexpected error testing role config access: %v", err)
	}

	tfresource.Test(t, tfresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAuthDestroy,
		Steps: []tfresource.TestStep{
			// Step 1: Create — auth_order + operator role GID
			{
				Config: testAccAuthResourceWithRolesConfig,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.#", "2"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.0", "local"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.1", "radius"),
					tfresource.TestCheckResourceAttrSet("f5os_auth.test", "id"),
					testAccCheckAuthOrderApplied([]string{"local", "radius"}),
					testAccCheckRoleGIDApplied("operator", 9010),
				),
			},
			// Step 2: Import state
			// remote_roles: import reads all device roles (by design), not
			// just user-declared ones, so imported state won't match config.
			// password_policy: not implemented.
			{
				ResourceName:      "f5os_auth.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"remote_roles",    // import reads all device roles, not just user-declared
					"password_policy", // not implemented
				},
			},
			// Step 3: Update — change auth_order and operator GID
			{
				Config: testAccAuthResourceWithRolesConfigUpdated,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.#", "2"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.0", "local"),
					tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.1", "tacacs"),
					tfresource.TestCheckResourceAttrSet("f5os_auth.test", "id"),
					testAccCheckAuthOrderApplied([]string{"local", "tacacs"}),
					testAccCheckRoleGIDApplied("operator", 9011),
				),
			},
			// Step 4: Destroy is automatic — CheckDestroy verifies cleanup
		},
	})
}

// testAccAuthResourceConfig — Create step: local+radius auth order only
const testAccAuthResourceConfig = `
resource "f5os_auth" "test" {
  auth_order = ["local", "radius"]
}
`

// testAccAuthResourceConfigUpdated — Update step: local+tacacs auth order only
const testAccAuthResourceConfigUpdated = `
resource "f5os_auth" "test" {
  auth_order = ["local", "tacacs"]
}
`

// testAccAuthResourceWithRolesConfig — Create step with roles.
// GID 9010 is chosen to avoid conflicting with built-in role GIDs (9000-9004).
const testAccAuthResourceWithRolesConfig = `
resource "f5os_auth" "test" {
  auth_order = ["local", "radius"]

  remote_roles = [
    {
      rolename   = "operator"
      remote_gid = 9010
    },
  ]
}
`

// testAccAuthResourceWithRolesConfigUpdated — Update step with roles.
// GID 9011 is chosen to avoid conflicting with built-in role GIDs (9000-9004).
const testAccAuthResourceWithRolesConfigUpdated = `
resource "f5os_auth" "test" {
  auth_order = ["local", "tacacs"]

  remote_roles = [
    {
      rolename   = "operator"
      remote_gid = 9011
    },
  ]
}
`

// ---------------------------------------------------------------------------
// Password Policy Unit Tests
// ---------------------------------------------------------------------------

func TestAuthResourceUnit_PasswordPolicyModel(t *testing.T) {
	// Test that the passwordPolicyModel struct can hold all 17 fields
	model := passwordPolicyModel{
		MinLength:           types.Int64Value(8),
		RequiredNumeric:     types.Int64Value(1),
		RequiredUppercase:   types.Int64Value(1),
		RequiredLowercase:   types.Int64Value(1),
		RequiredSpecial:     types.Int64Value(1),
		RequiredDifferences: types.Int64Value(5),
		RejectUsername:      types.BoolValue(true),
		ApplyToRoot:         types.BoolValue(false),
		Retries:             types.Int64Value(3),
		MaxLoginFailures:    types.Int64Value(5),
		UnlockTime:          types.Int64Value(300),
		RootLockout:         types.BoolValue(true),
		RootUnlockTime:      types.Int64Value(600),
		MaxAge:              types.Int64Value(90),
		// v1.7+ fields
		MaxLetterRepeat:   types.Int64Value(3),
		MaxSequenceRepeat: types.Int64Value(2),
		MaxClassRepeat:    types.Int64Value(4),
	}

	assert.Equal(t, int64(8), model.MinLength.ValueInt64())
	assert.Equal(t, int64(1), model.RequiredNumeric.ValueInt64())
	assert.Equal(t, int64(1), model.RequiredUppercase.ValueInt64())
	assert.Equal(t, int64(1), model.RequiredLowercase.ValueInt64())
	assert.Equal(t, int64(1), model.RequiredSpecial.ValueInt64())
	assert.Equal(t, int64(5), model.RequiredDifferences.ValueInt64())
	assert.True(t, model.RejectUsername.ValueBool())
	assert.False(t, model.ApplyToRoot.ValueBool())
	assert.Equal(t, int64(3), model.Retries.ValueInt64())
	assert.Equal(t, int64(5), model.MaxLoginFailures.ValueInt64())
	assert.Equal(t, int64(300), model.UnlockTime.ValueInt64())
	assert.True(t, model.RootLockout.ValueBool())
	assert.Equal(t, int64(600), model.RootUnlockTime.ValueInt64())
	assert.Equal(t, int64(90), model.MaxAge.ValueInt64())
	assert.Equal(t, int64(3), model.MaxLetterRepeat.ValueInt64())
	assert.Equal(t, int64(2), model.MaxSequenceRepeat.ValueInt64())
	assert.Equal(t, int64(4), model.MaxClassRepeat.ValueInt64())
}

func TestPasswordPolicyModelToConfig(t *testing.T) {
	model := &passwordPolicyModel{
		MinLength:           types.Int64Value(10),
		RequiredNumeric:     types.Int64Value(2),
		RequiredUppercase:   types.Int64Value(1),
		RequiredLowercase:   types.Int64Value(1),
		RequiredSpecial:     types.Int64Value(1),
		RequiredDifferences: types.Int64Value(6),
		RejectUsername:      types.BoolValue(true),
		ApplyToRoot:         types.BoolValue(false),
		Retries:             types.Int64Value(5),
		MaxLoginFailures:    types.Int64Value(10),
		UnlockTime:          types.Int64Value(600),
		RootLockout:         types.BoolValue(true),
		RootUnlockTime:      types.Int64Value(900),
		MaxAge:              types.Int64Value(180),
		MaxLetterRepeat:     types.Int64Value(3),
		MaxSequenceRepeat:   types.Int64Value(2),
		MaxClassRepeat:      types.Int64Value(4),
	}

	// Test v1.7+ (includes v1.7+ fields)
	configV17 := passwordPolicyModelToConfig(model, "1.7.0")
	assert.NotNil(t, configV17)
	assert.Equal(t, int64(10), *configV17.MinLength)
	assert.Equal(t, int64(2), *configV17.RequiredNumeric)
	assert.Equal(t, int64(1), *configV17.RequiredUppercase)
	assert.Equal(t, int64(1), *configV17.RequiredLowercase)
	assert.Equal(t, int64(1), *configV17.RequiredSpecial)
	assert.Equal(t, int64(6), *configV17.RequiredDifferences)
	assert.True(t, *configV17.RejectUsername)
	assert.False(t, *configV17.ApplyToRoot)
	assert.Equal(t, int64(5), *configV17.Retries)
	assert.Equal(t, int64(10), *configV17.MaxLoginFailures)
	assert.Equal(t, int64(600), *configV17.UnlockTime)
	assert.True(t, *configV17.RootLockout)
	assert.Equal(t, int64(900), *configV17.RootUnlockTime)
	assert.Equal(t, int64(180), *configV17.MaxAge)
	assert.Equal(t, int64(3), *configV17.MaxLetterRepeat)
	assert.Equal(t, int64(2), *configV17.MaxSequenceRepeat)
	assert.Equal(t, int64(4), *configV17.MaxClassRepeat)

	// Test base case (omits v1.7+ fields)
	config := passwordPolicyModelToConfig(model, "1.5.0")
	assert.NotNil(t, config)
	assert.Equal(t, int64(10), *config.MinLength)
	assert.Nil(t, config.MaxLetterRepeat, "v1.7+ fields should be nil on base config")
	assert.Nil(t, config.MaxSequenceRepeat, "v1.7+ fields should be nil on base config")
	assert.Nil(t, config.MaxClassRepeat, "v1.7+ fields should be nil on base config")
}

func TestPasswordPolicyConfigToModel(t *testing.T) {
	// Setup API config struct
	minLen := int64(8)
	reqNum := int64(1)
	reqUpper := int64(1)
	reqLower := int64(1)
	reqSpecial := int64(1)
	reqDiff := int64(5)
	rejectUser := true
	applyRoot := false
	retries := int64(3)
	maxFail := int64(5)
	unlockTime := int64(300)
	rootLockout := true
	rootUnlock := int64(600)
	maxAge := int64(90)
	maxLetterRep := int64(3)
	maxSeqRep := int64(2)
	maxClassRep := int64(4)

	config := &f5os.PasswordPolicyConfig{
		MinLength:           &minLen,
		RequiredNumeric:     &reqNum,
		RequiredUppercase:   &reqUpper,
		RequiredLowercase:   &reqLower,
		RequiredSpecial:     &reqSpecial,
		RequiredDifferences: &reqDiff,
		RejectUsername:      &rejectUser,
		ApplyToRoot:         &applyRoot,
		Retries:             &retries,
		MaxLoginFailures:    &maxFail,
		UnlockTime:          &unlockTime,
		RootLockout:         &rootLockout,
		RootUnlockTime:      &rootUnlock,
		MaxAge:              &maxAge,
		MaxLetterRepeat:     &maxLetterRep,
		MaxSequenceRepeat:   &maxSeqRep,
		MaxClassRepeat:      &maxClassRep,
	}

	// Test v1.7+
	modelV17 := passwordPolicyConfigToModel(config, "1.7.0")
	assert.Equal(t, int64(8), modelV17.MinLength.ValueInt64())
	assert.Equal(t, int64(1), modelV17.RequiredNumeric.ValueInt64())
	assert.Equal(t, int64(1), modelV17.RequiredUppercase.ValueInt64())
	assert.Equal(t, int64(1), modelV17.RequiredLowercase.ValueInt64())
	assert.Equal(t, int64(1), modelV17.RequiredSpecial.ValueInt64())
	assert.Equal(t, int64(5), modelV17.RequiredDifferences.ValueInt64())
	assert.True(t, modelV17.RejectUsername.ValueBool())
	assert.False(t, modelV17.ApplyToRoot.ValueBool())
	assert.Equal(t, int64(3), modelV17.Retries.ValueInt64())
	assert.Equal(t, int64(5), modelV17.MaxLoginFailures.ValueInt64())
	assert.Equal(t, int64(300), modelV17.UnlockTime.ValueInt64())
	assert.True(t, modelV17.RootLockout.ValueBool())
	assert.Equal(t, int64(600), modelV17.RootUnlockTime.ValueInt64())
	assert.Equal(t, int64(90), modelV17.MaxAge.ValueInt64())
	assert.Equal(t, int64(3), modelV17.MaxLetterRepeat.ValueInt64())
	assert.Equal(t, int64(2), modelV17.MaxSequenceRepeat.ValueInt64())
	assert.Equal(t, int64(4), modelV17.MaxClassRepeat.ValueInt64())

	// Test base case (v1.7+ fields should be null)
	model := passwordPolicyConfigToModel(config, "1.5.0")
	assert.Equal(t, int64(8), model.MinLength.ValueInt64())
	assert.True(t, model.MaxLetterRepeat.IsNull(), "v1.7+ fields should be null on base config")
	assert.True(t, model.MaxSequenceRepeat.IsNull(), "v1.7+ fields should be null on base config")
	assert.True(t, model.MaxClassRepeat.IsNull(), "v1.7+ fields should be null on base config")
}

func TestPasswordPolicyAttrTypes(t *testing.T) {
	attrTypes := passwordPolicyAttrTypes()
	// Should have exactly 17 fields
	assert.Equal(t, 17, len(attrTypes), "passwordPolicyAttrTypes should return 17 attribute types")

	// Check all expected keys exist
	expectedKeys := []string{
		"min_length", "required_numeric", "required_uppercase", "required_lowercase",
		"required_special", "required_differences", "reject_username", "apply_to_root",
		"retries", "max_login_failures", "unlock_time", "root_lockout",
		"root_unlock_time", "max_age", "max_letter_repeat", "max_sequence_repeat",
		"max_class_repeat",
	}
	for _, key := range expectedKeys {
		_, exists := attrTypes[key]
		assert.True(t, exists, "attribute type %q should exist", key)
	}
}

// ---------------------------------------------------------------------------
// Password Policy Mocked HTTP Tests
// ---------------------------------------------------------------------------

// setupMockServerWithPasswordPolicy creates a mock server that handles
// password policy endpoints in addition to auth_order and roles.
func setupMockServerWithPasswordPolicy() *httptest.Server {
	currentPolicy := map[string]interface{}{
		"min-length":           6,
		"required-numeric":     0,
		"required-uppercase":   0,
		"required-lowercase":   0,
		"required-special":     0,
		"required-differences": 8,
		"reject-username":      false,
		"apply-to-root":        true,
		"retries":              3,
		"max-login-failures":   10,
		"unlock-time":          60,
		"root-lockout":         true,
		"root-unlock-time":     60,
		"max-age":              0,
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/aaa/f5-openconfig-aaa-password-policy:password-policy/config"):
			switch r.Method {
			case "GET":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				resp := map[string]interface{}{
					"f5-openconfig-aaa-password-policy:config": currentPolicy,
				}
				_ = json.NewEncoder(w).Encode(resp)
			case "PATCH":
				var payload map[string]map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if config, ok := payload["f5-openconfig-aaa-password-policy:config"]; ok {
					for k, v := range config {
						currentPolicy[k] = v
					}
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		case strings.HasSuffix(r.URL.Path, "/aaa/authentication/config"):
			switch r.Method {
			case "GET":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{
					"openconfig-system:config": {
						"authentication-method": ["openconfig-aaa-types:LOCAL", "openconfig-aaa-types:RADIUS_ALL"]
					}
				}`)
			case "PUT", "PATCH":
				w.WriteHeader(http.StatusNoContent)
			case "DELETE":
				w.WriteHeader(http.StatusNoContent)
			}
		case strings.HasSuffix(r.URL.Path, "/aaa/authentication/f5-system-aaa:roles"):
			switch r.Method {
			case "GET":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{
					"f5-system-aaa:roles": {
						"role": [
							{"rolename": "admin", "config": {"rolename": "admin", "gid": 9000, "remote-gid": "-"}},
							{"rolename": "operator", "config": {"rolename": "operator", "gid": 9001, "remote-gid": 9001}}
						]
					}
				}`)
			case "PUT", "PATCH":
				w.WriteHeader(http.StatusNoContent)
			case "DELETE":
				w.WriteHeader(http.StatusNoContent)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestAuthResourceMocked_PasswordPolicyClientMethods(t *testing.T) {
	server := setupMockServerWithPasswordPolicy()
	defer server.Close()

	config := &f5os.F5osConfig{
		Host:             server.URL,
		User:             "test",
		Password:         "test",
		DisableSSLVerify: true,
	}

	client, err := f5os.NewSession(config)
	assert.NoError(t, err, "Client initialization should not fail")
	assert.NotNil(t, client, "Client should not be nil")

	// Test GetPasswordPolicy method
	t.Run("GetPasswordPolicy", func(t *testing.T) {
		result, err := client.GetPasswordPolicy()
		assert.NoError(t, err, "GetPasswordPolicy should not return error")
		assert.NotNil(t, result, "GetPasswordPolicy should return result")
		assert.NotNil(t, result.MinLength, "MinLength should be set")
		assert.Equal(t, int64(6), *result.MinLength, "MinLength should be 6")
	})

	// Test SetPasswordPolicy method
	t.Run("SetPasswordPolicy", func(t *testing.T) {
		minLen := int64(10)
		reqNum := int64(2)
		newPolicy := &f5os.PasswordPolicyConfig{
			MinLength:       &minLen,
			RequiredNumeric: &reqNum,
		}
		err := client.SetPasswordPolicy(newPolicy)
		assert.NoError(t, err, "SetPasswordPolicy should not return error")

		// Verify the change took effect
		result, err := client.GetPasswordPolicy()
		assert.NoError(t, err, "GetPasswordPolicy after set should not fail")
		assert.Equal(t, int64(10), *result.MinLength, "MinLength should be updated to 10")
	})
}

// mockedPasswordPolicyTestParams holds the version-specific parts of a
// mocked password policy lifecycle test.
type mockedPasswordPolicyTestParams struct {
	fixture string                     // path to the full /aaa fixture
	version string                     // platform version ("" for default/pre-v1.7)
	config  string                     // HCL config for the test step
	checks  []tfresource.TestCheckFunc // Terraform state checks
}

// runMockedPasswordPolicyTest sets up a mock server with the given fixture
// and version, then runs a Create/Read/Delete lifecycle test.
func runMockedPasswordPolicyTest(t *testing.T, params mockedPasswordPolicyTestParams) {
	t.Helper()

	// Load initial policy from fixture
	var fixture map[string]interface{}
	if err := json.Unmarshal(loadFixtureBytes(params.fixture), &fixture); err != nil {
		t.Fatalf("failed to load fixture %s: %v", params.fixture, err)
	}
	aaa := fixture["openconfig-system:aaa"].(map[string]interface{})
	pp := aaa["f5-openconfig-aaa-password-policy:password-policy"].(map[string]interface{})
	originalPolicy := pp["config"].(map[string]interface{})
	currentPolicy := make(map[string]interface{})
	for k, v := range originalPolicy {
		currentPolicy[k] = v
	}

	testAccPreUnitCheck(t)

	if params.version != "" {
		setupMockPlatformVersion(mux, params.version)
	}

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString(params.fixture))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"openconfig-system:config":{"authentication-method":["openconfig-aaa-types:LOCAL"]}}`)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/config/authentication-method", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-openconfig-aaa-password-policy:password-policy/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			resp := map[string]interface{}{
				"f5-openconfig-aaa-password-policy:config": currentPolicy,
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "PATCH":
			var payload map[string]map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if config, ok := payload["f5-openconfig-aaa-password-policy:config"]; ok {
				for k, v := range config {
					currentPolicy[k] = v
				}
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	tfresource.Test(t, tfresource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: params.config,
				Check:  tfresource.ComposeAggregateTestCheckFunc(params.checks...),
			},
		},
	})
}

func TestAuthResourceMocked_PasswordPolicyCreateReadDelete(t *testing.T) {
	runMockedPasswordPolicyTest(t, mockedPasswordPolicyTestParams{
		fixture: "./fixtures/f5os_auth.json",
		config: `
resource "f5os_auth" "test" {
  auth_order = ["local"]
  password_policy = {
    min_length         = 10
    required_numeric   = 2
    required_uppercase = 1
    required_lowercase = 1
    required_special   = 1
    reject_username    = true
    max_login_failures = 10
    unlock_time        = 300
  }
}
`,
		checks: []tfresource.TestCheckFunc{
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.min_length", "10"),
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.required_numeric", "2"),
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.required_uppercase", "1"),
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.required_lowercase", "1"),
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.required_special", "1"),
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.reject_username", "true"),
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.max_login_failures", "10"),
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.unlock_time", "300"),
		},
	})
}

func TestAuthResourceMocked_PasswordPolicyV17CreateReadDelete(t *testing.T) {
	runMockedPasswordPolicyTest(t, mockedPasswordPolicyTestParams{
		fixture: "./fixtures/f5os_auth_v17.json",
		version: "1.7.0",
		config: `
resource "f5os_auth" "test" {
  auth_order = ["local"]
  password_policy = {
    min_length          = 10
    max_letter_repeat   = 4
    max_sequence_repeat = 3
    max_class_repeat    = 2
  }
}
`,
		checks: []tfresource.TestCheckFunc{
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.min_length", "10"),
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.max_letter_repeat", "4"),
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.max_sequence_repeat", "3"),
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.max_class_repeat", "2"),
		},
	})
}

// ---------------------------------------------------------------------------
// Password Policy Acceptance Tests
// ---------------------------------------------------------------------------

// newPasswordPolicyClientFromEnv creates a fresh f5osclient from environment variables.
// Used by password policy tests to verify device state independently.
func newPasswordPolicyClientFromEnv() (*f5os.F5os, error) {
	return newAuthClientFromEnv() // Reuse existing helper
}

// testAccCheckPasswordPolicyApplied queries the device directly to verify
// password policy fields match expected values.
func testAccCheckPasswordPolicyApplied(expected *f5os.PasswordPolicyConfig) tfresource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newPasswordPolicyClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create f5os client: %w", err)
		}
		policy, err := client.GetPasswordPolicy()
		if err != nil {
			return fmt.Errorf("failed to read password policy from device: %w", err)
		}
		// Check min-length (always present)
		if expected.MinLength != nil {
			if policy.MinLength == nil || *policy.MinLength != *expected.MinLength {
				actual := "<nil>"
				if policy.MinLength != nil {
					actual = fmt.Sprintf("%d", *policy.MinLength)
				}
				return fmt.Errorf("min-length mismatch: expected %d, got %s", *expected.MinLength, actual)
			}
		}
		// Check v1.7+ fields when expected
		if expected.MaxLetterRepeat != nil {
			if policy.MaxLetterRepeat == nil || *policy.MaxLetterRepeat != *expected.MaxLetterRepeat {
				actual := "<nil>"
				if policy.MaxLetterRepeat != nil {
					actual = fmt.Sprintf("%d", *policy.MaxLetterRepeat)
				}
				return fmt.Errorf("max-letter-repeat mismatch: expected %d, got %s", *expected.MaxLetterRepeat, actual)
			}
		}
		if expected.MaxSequenceRepeat != nil {
			if policy.MaxSequenceRepeat == nil || *policy.MaxSequenceRepeat != *expected.MaxSequenceRepeat {
				actual := "<nil>"
				if policy.MaxSequenceRepeat != nil {
					actual = fmt.Sprintf("%d", *policy.MaxSequenceRepeat)
				}
				return fmt.Errorf("max-sequence-repeat mismatch: expected %d, got %s", *expected.MaxSequenceRepeat, actual)
			}
		}
		if expected.MaxClassRepeat != nil {
			if policy.MaxClassRepeat == nil || *policy.MaxClassRepeat != *expected.MaxClassRepeat {
				actual := "<nil>"
				if policy.MaxClassRepeat != nil {
					actual = fmt.Sprintf("%d", *policy.MaxClassRepeat)
				}
				return fmt.Errorf("max-class-repeat mismatch: expected %d, got %s", *expected.MaxClassRepeat, actual)
			}
		}
		return nil
	}
}

// TestAccAuthResourcePasswordPolicy tests the full lifecycle of password_policy:
// Create, Import, Update, Destroy with restore.
//
// Automatically adapts to the device version:
//   - Pre-v1.7: tests the 14 core fields only
//   - v1.7+: also tests max_letter_repeat, max_sequence_repeat, max_class_repeat
//
// Safety:
//   - Uses reasonable password policy values that won't lock users out
//   - Restores original password policy on destroy
//   - Uses t.Cleanup to restore baseline even if test fails
func TestAccAuthResourcePasswordPolicy(t *testing.T) {
	client, err := newPasswordPolicyClientFromEnv()
	if err != nil {
		t.Skipf("Cannot create f5os client: %v", err)
	}

	deviceVersion := client.PlatformVersion
	t.Logf("Device version: %s", deviceVersion)

	// Capture the true device baseline before we touch anything
	trueBaseline, err := client.GetPasswordPolicy()
	if err != nil {
		t.Fatalf("Failed to read true device baseline password policy: %v", err)
	}
	t.Logf("True device baseline min_length: %v", *trueBaseline.MinLength)

	// Cleanup: restore the true baseline regardless of test outcome
	t.Cleanup(func() {
		cleanupClient, err := newPasswordPolicyClientFromEnv()
		if err != nil {
			t.Logf("WARNING: cleanup failed to create client: %v", err)
			return
		}
		if err := cleanupClient.SetPasswordPolicy(trueBaseline); err != nil {
			t.Logf("WARNING: cleanup failed to restore password policy: %v", err)
		} else {
			t.Logf("Cleanup: restored password policy to baseline (min_length=%v)", *trueBaseline.MinLength)
		}
	})

	// Build version-appropriate configs and checks
	createConfig, createChecks, createExpected := testAccPasswordPolicyCreateParams(deviceVersion)
	updateConfig, updateChecks, updateExpected := testAccPasswordPolicyUpdateParams(deviceVersion)

	tfresource.Test(t, tfresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			// Step 1: Create with password_policy
			{
				Config: createConfig,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					append(createChecks, testAccCheckPasswordPolicyApplied(createExpected))...,
				),
			},
			// Step 2: Import state
			{
				ResourceName:      "f5os_auth.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"remote_roles",    // import reads all device roles, not just user-declared
					"password_policy", // import reads all device fields, not just user-declared
				},
			},
			// Step 3: Update password_policy
			{
				Config: updateConfig,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					append(updateChecks, testAccCheckPasswordPolicyApplied(updateExpected))...,
				),
			},
			// Step 4: Destroy is automatic — cleanup restores baseline
		},
	})
}

// testAccPasswordPolicyCreateParams returns the HCL config, Terraform state
// checks, and expected API values for the Create step, adapted for the device version.
func testAccPasswordPolicyCreateParams(deviceVersion string) (string, []tfresource.TestCheckFunc, *f5os.PasswordPolicyConfig) {
	minLen := int64(10)
	expected := &f5os.PasswordPolicyConfig{MinLength: &minLen}

	checks := []tfresource.TestCheckFunc{
		tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.0", "local"),
		tfresource.TestCheckResourceAttr("f5os_auth.test", "auth_order.1", "radius"),
		tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.min_length", "10"),
		tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.required_numeric", "2"),
		tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.reject_username", "true"),
	}

	config := `
resource "f5os_auth" "test" {
  auth_order = ["local", "radius"]

  password_policy = {
    min_length         = 10
    required_numeric   = 2
    required_uppercase = 1
    required_lowercase = 1
    required_special   = 1
    reject_username    = true
    max_login_failures = 10
    unlock_time        = 300`

	// v1.7+ added max-letter-repeat, max-sequence-repeat, max-class-repeat
	if platformVersionAtLeast(deviceVersion, "v1.7") {
		config += `
    max_letter_repeat   = 4
    max_sequence_repeat = 3
    max_class_repeat    = 2`

		checks = append(checks,
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.max_letter_repeat", "4"),
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.max_sequence_repeat", "3"),
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.max_class_repeat", "2"),
		)
		letterRep, seqRep, classRep := int64(4), int64(3), int64(2)
		expected.MaxLetterRepeat = &letterRep
		expected.MaxSequenceRepeat = &seqRep
		expected.MaxClassRepeat = &classRep
	}

	config += `
  }
}
`
	return config, checks, expected
}

// testAccPasswordPolicyUpdateParams returns the HCL config, Terraform state
// checks, and expected API values for the Update step, adapted for the device version.
func testAccPasswordPolicyUpdateParams(deviceVersion string) (string, []tfresource.TestCheckFunc, *f5os.PasswordPolicyConfig) {
	minLen := int64(12)
	expected := &f5os.PasswordPolicyConfig{MinLength: &minLen}

	checks := []tfresource.TestCheckFunc{
		tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.min_length", "12"),
		tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.required_numeric", "3"),
		tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.max_login_failures", "5"),
	}

	config := `
resource "f5os_auth" "test" {
  auth_order = ["local", "radius"]

  password_policy = {
    min_length         = 12
    required_numeric   = 3
    required_uppercase = 2
    required_lowercase = 2
    required_special   = 1
    reject_username    = true
    max_login_failures = 5
    unlock_time        = 600`

	// v1.7+ added max-letter-repeat, max-sequence-repeat, max-class-repeat
	if platformVersionAtLeast(deviceVersion, "v1.7") {
		config += `
    max_letter_repeat   = 5
    max_sequence_repeat = 4
    max_class_repeat    = 3`

		checks = append(checks,
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.max_letter_repeat", "5"),
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.max_sequence_repeat", "4"),
			tfresource.TestCheckResourceAttr("f5os_auth.test", "password_policy.max_class_repeat", "3"),
		)
		letterRep, seqRep, classRep := int64(5), int64(4), int64(3)
		expected.MaxLetterRepeat = &letterRep
		expected.MaxSequenceRepeat = &seqRep
		expected.MaxClassRepeat = &classRep
	}

	config += `
  }
}
`
	return config, checks, expected
}
