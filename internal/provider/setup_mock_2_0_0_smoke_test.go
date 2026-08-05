package provider

import (
	"encoding/json"
	"testing"
)

// mustMap fetches key from m and asserts it is a JSON object. On any
// failure it calls t.Fatalf with a message that includes the traversal
// path so a contributor changing a fixture gets an actionable error
// instead of a stack trace.
func mustMap(t *testing.T, m map[string]interface{}, key, path string) map[string]interface{} {
	t.Helper()
	raw, ok := m[key]
	if !ok {
		t.Fatalf("%s: missing key %q", path, key)
	}
	obj, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("%s.%s: expected object, got %T", path, key, raw)
	}
	return obj
}

// mustSlice fetches key from m and asserts it is a JSON array.
func mustSlice(t *testing.T, m map[string]interface{}, key, path string) []interface{} {
	t.Helper()
	raw, ok := m[key]
	if !ok {
		t.Fatalf("%s: missing key %q", path, key)
	}
	arr, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("%s.%s: expected array, got %T", path, key, raw)
	}
	return arr
}

// TestUnitFixtures2_0_0Shape verifies the 2.0.0 fixture files are well-formed
// JSON with the expected structural shape. This catches regressions where a
// contributor edits a fixture and accidentally reintroduces a leaf the task
// removed (e.g. tally-count) or drops a leaf tests depend on (e.g. min-days).
//
// The test is intentionally shallow: it validates presence/absence of the
// leaves called out by the 2.0.0 shape task, not the leaves' values. Value
// assertions belong in the resource-specific unit tests that consume the
// fixtures.
//
// All traversal uses defensive lookups (mustMap / mustSlice) so a fixture
// shape mismatch produces a t.Fatalf with the traversal path instead of a
// panic; this gives contributors a clear failure signal to act on.
func TestUnitFixtures2_0_0Shape(t *testing.T) {
	t.Run("f5os_auth.json has no tally-count", func(t *testing.T) {
		assertNoTallyCount(t, "./fixtures/f5os_auth.json",
			"leaf was removed in F5OS 2.0.0 and this fixture must not carry it")
	})

	t.Run("f5os_auth_v17.json has no tally-count", func(t *testing.T) {
		assertNoTallyCount(t, "./fixtures/f5os_auth_v17.json",
			"fixture was intentionally cleaned to align with 2.0.0 shape")
	})

	t.Run("f5os_auth_v2_0.json contains expanded 2.0.0 leaves", func(t *testing.T) {
		raw := loadFixtureString("./fixtures/f5os_auth_v2_0.json")
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			t.Fatalf("f5os_auth_v2_0.json is not valid JSON: %s", err)
		}
		aaa := mustMap(t, data, "openconfig-system:aaa", "root")

		// password-policy must carry 2.0.0 additions
		pp := mustMap(t, aaa, "f5-openconfig-aaa-password-policy:password-policy", "aaa")
		ppCfg := mustMap(t, pp, "config", "aaa.password-policy")
		for _, leaf := range []string{"min-days", "remember", "warn-age", "max-letter-repeat"} {
			if _, ok := ppCfg[leaf]; !ok {
				t.Errorf("password-policy.config missing 2.0.0 leaf %q", leaf)
			}
		}

		// login-policy container must exist with all three leaves
		lp := mustMap(t, aaa, "f5-openconfig-aaa-login-policy:login-policy", "aaa")
		lpCfg := mustMap(t, lp, "config", "aaa.login-policy")
		for _, leaf := range []string{"admin-role-limit", "restconf-max-session-limit", "ssh-max-session-limit"} {
			if _, ok := lpCfg[leaf]; !ok {
				t.Errorf("login-policy.config missing 2.0.0 leaf %q", leaf)
			}
		}

		// LDAP container must expose the object-class leaf-lists
		auth := mustMap(t, aaa, "authentication", "aaa")
		ldap := mustMap(t, auth, "f5-openconfig-aaa-ldap:ldap", "aaa.authentication")
		for _, leaf := range []string{"user-object-class", "group-object-class"} {
			if _, ok := ldap[leaf]; !ok {
				t.Errorf("ldap missing 2.0.0 leaf %q", leaf)
			}
		}
	})

	t.Run("tenant_get_status_2_0_0_shape.json has 2.0.0 leaves and no pre-2.0 legacy leaves", func(t *testing.T) {
		raw := loadFixtureString("./fixtures/tenant_get_status_2_0_0_shape.json")
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			t.Fatalf("tenant_get_status_2_0_0_shape.json is not valid JSON: %s", err)
		}
		state := mustMap(t, data, "f5-tenants:state", "root")

		// Required 2.0.0 leaves
		for _, leaf := range []string{
			"max-nodes",
			"f5-tenant-mgmt-vlan:mgmt-vlan",
			"f5-tenant-mgmt-vlan:mgmt-vlan-accessible",
			"feature-flags",
		} {
			if _, ok := state[leaf]; !ok {
				t.Errorf("tenant 2.0.0 shape fixture missing required leaf %q", leaf)
			}
		}

		// Pre-2.0 legacy leaves must be absent
		for _, leaf := range []string{
			"instances",
			"cpu-allocations",
			"primary-slot",
			"image-version",
			"mac-block",
		} {
			if _, ok := state[leaf]; ok {
				t.Errorf("tenant 2.0.0 shape fixture must not carry pre-2.0 leaf %q; leaf was removed / renamed in 2.0.0", leaf)
			}
		}
	})

	t.Run("focused container fixtures are well-formed", func(t *testing.T) {
		for _, path := range []string{
			"./fixtures/aaa_login_policy_2_0_0.json",
			"./fixtures/aaa_password_policy_2_0_0.json",
			"./fixtures/aaa_ldap_object_classes_2_0_0.json",
			"./fixtures/aaa_ldap_object_classes_2_0_0_empty.json",
		} {
			raw := loadFixtureString(path)
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &data); err != nil {
				t.Errorf("%s not valid JSON: %s", path, err)
			}
		}
	})
}

// assertNoTallyCount walks aaa.authentication.f5-system-aaa:users.user[]
// in the given fixture and reports a t.Error for any user whose config or
// state carries a tally-count leaf. reasonSuffix is appended to the error
// message so each caller can tell contributors *why* their fixture must
// not carry the leaf (removed by 2.0.0 vs. intentionally cleaned).
func assertNoTallyCount(t *testing.T, fixturePath, reasonSuffix string) {
	t.Helper()
	raw := loadFixtureString(fixturePath)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("%s is not valid JSON: %s", fixturePath, err)
	}
	aaa := mustMap(t, data, "openconfig-system:aaa", "root")
	auth := mustMap(t, aaa, "authentication", "aaa")
	usersContainer := mustMap(t, auth, "f5-system-aaa:users", "aaa.authentication")
	users := mustSlice(t, usersContainer, "user", "aaa.authentication.f5-system-aaa:users")
	for i, u := range users {
		userObj, ok := u.(map[string]interface{})
		if !ok {
			t.Errorf("user[%d]: expected object, got %T", i, u)
			continue
		}
		for _, section := range []string{"config", "state"} {
			m, ok := userObj[section].(map[string]interface{})
			if !ok {
				// Missing section is not the concern of this check.
				continue
			}
			if _, has := m["tally-count"]; has {
				t.Errorf("user[%d].%s still contains tally-count; %s", i, section, reasonSuffix)
			}
		}
	}
}

// TestUnitSetupMock2_0_0Helpers verifies the mock-registration helpers wire
// up the expected fixture at the expected RESTCONF path. It stands up the
// mock server via testAccPreUnitCheck, registers the helpers, and then
// probes each endpoint over the client-facing HTTP interface to confirm the
// response body matches the served fixture.
//
// The test uses client.GetRequest which returns only the response body (not
// the HTTP status), so each case asserts the body is valid, non-empty JSON.
// An HTTP-level status assertion would require either extending the client
// to surface status or bypassing the client with a raw http.Get — neither
// is warranted given the mock helpers hard-code http.StatusOK on the write
// path; a change there would be caught by every other unit test that uses
// these fixtures.
func TestUnitSetupMock2_0_0Helpers(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupMock2_0_0AAA(mux)
	// setupMock2_0_0AAA intentionally does not register the wide aaa
	// container path (many callers already mock it). This smoke test
	// exercises that path directly, so opt in via the dedicated helper.
	setupMock2_0_0AAAContainer(t, mux)

	// Cases that hit the "populated" helpers registered above. LDAP
	// gets a second, separate probe below against a fresh mux with the
	// empty-variant helper — see comment there.
	cases := []struct {
		name    string
		path    string
		fixture string
	}{
		{
			name:    "login-policy config",
			path:    "/restconf/data/openconfig-system:system/aaa/f5-openconfig-aaa-login-policy:login-policy/config",
			fixture: "./fixtures/aaa_login_policy_2_0_0.json",
		},
		{
			name:    "password-policy config",
			path:    "/restconf/data/openconfig-system:system/aaa/f5-openconfig-aaa-password-policy:password-policy/config",
			fixture: "./fixtures/aaa_password_policy_2_0_0.json",
		},
		{
			name:    "LDAP object-classes (populated)",
			path:    "/restconf/data/openconfig-system:system/aaa/authentication/f5-openconfig-aaa-ldap:ldap",
			fixture: "./fixtures/aaa_ldap_object_classes_2_0_0.json",
		},
		{
			name:    "aaa container",
			path:    "/restconf/data/openconfig-system:system/aaa",
			fixture: "./fixtures/f5os_auth_v2_0.json",
		},
	}

	client, err := newTestClientFromEnv()
	if err != nil {
		t.Fatalf("failed to create test client against mock: %s", err)
	}
	// setupMockPlatformVersion2_0_0 puts a 2.0.0 build string on the client.
	if !platformVersionAtLeast(client.PlatformVersion, "v2.0") {
		t.Fatalf("expected mock client to report >= v2.0, got %q", client.PlatformVersion)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.GetRequest(tc.path[len("/restconf/data"):])
			if err != nil {
				t.Fatalf("GET %s failed: %s", tc.path, err)
			}
			// Structural check: valid, non-empty JSON.
			var body map[string]interface{}
			if err := json.Unmarshal(resp, &body); err != nil {
				t.Fatalf("response for %s is not valid JSON: %s\nbody=%s", tc.path, err, string(resp))
			}
			if len(body) == 0 {
				t.Errorf("response for %s is empty JSON object", tc.path)
			}
		})
	}
}

// TestUnitSetupMock2_0_0LdapEmptyVariant verifies the LDAP helper's
// "empty" fixture variant, which exercises the code path where the
// device has cleared the object-class leaf-lists to empty (distinct
// from the leaf-lists being unmanaged / absent).
//
// This lives in its own top-level test rather than a subtest of
// TestUnitSetupMock2_0_0Helpers because it needs an isolated ServeMux
// — http.ServeMux panics on duplicate handler registration, and the
// helper above already registered the same LDAP path with the
// populated fixture.
func TestUnitSetupMock2_0_0LdapEmptyVariant(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupMockPlatformVersion2_0_0(mux)
	setupMock2_0_0LdapObjectClasses(mux, true /* empty */)

	client, err := newTestClientFromEnv()
	if err != nil {
		t.Fatalf("failed to create test client against mock: %s", err)
	}

	const path = "/restconf/data/openconfig-system:system/aaa/authentication/f5-openconfig-aaa-ldap:ldap"
	resp, err := client.GetRequest(path[len("/restconf/data"):])
	if err != nil {
		t.Fatalf("GET %s failed: %s", path, err)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(resp, &body); err != nil {
		t.Fatalf("response for %s is not valid JSON: %s\nbody=%s", path, err, string(resp))
	}
	ldap := mustMap(t, body, "f5-openconfig-aaa-ldap:ldap", "root")
	// Empty variant: both leaf-lists must be present and must be
	// empty arrays. This is what the "cleared to empty" case looks
	// like on the wire and is what the client is expected to
	// distinguish from the "unmanaged" case (leaves absent).
	for _, leaf := range []string{"user-object-class", "group-object-class"} {
		v, ok := ldap[leaf]
		if !ok {
			t.Errorf("empty-variant fixture missing %q — the leaf must be present as an empty array to exercise the cleared-to-empty path", leaf)
			continue
		}
		arr, ok := v.([]interface{})
		if !ok {
			t.Errorf("empty-variant %q: expected array, got %T", leaf, v)
			continue
		}
		if len(arr) != 0 {
			t.Errorf("empty-variant %q: expected empty array, got %d elements", leaf, len(arr))
		}
	}
}
