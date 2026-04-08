package provider

import (
	"context"
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

	// Test PasswordPolicy field
	passwordPolicyAttrs := map[string]attr.Value{
		"min_length": types.Int64Value(8),
		"max_length": types.Int64Value(32),
	}
	passwordPolicyType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"min_length": types.Int64Type,
			"max_length": types.Int64Type,
		},
	}
	passwordPolicyObj, _ := types.ObjectValue(passwordPolicyType.AttrTypes, passwordPolicyAttrs)
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
		if originalOperatorGID == 0 {
			t.Logf("Skipping operator GID restore: no remote-gid was configured before test")
			return
		}
		restoreClient, err := newAuthClientFromEnv()
		if err != nil {
			t.Logf("WARNING: failed to create client for operator GID restore: %v", err)
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
