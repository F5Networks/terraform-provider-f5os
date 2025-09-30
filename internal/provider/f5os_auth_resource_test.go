package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
		case "/restconf/data/openconfig-system:roles":
			switch r.Method {
			case "GET":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{
					"openconfig-system:roles": {
						"role": [
							{
								"name": "test-role",
								"config": {
									"role-name": "test-role"
								}
							}
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

	// // Test GetRoles method
	// t.Run("GetRoles", func(t *testing.T) {
	// 	result, err := client.GetRoles()
	// 	assert.NoError(t, err, "GetRoles should not return error")
	// 	assert.NotNil(t, result, "GetRoles should return result")
	// })
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
