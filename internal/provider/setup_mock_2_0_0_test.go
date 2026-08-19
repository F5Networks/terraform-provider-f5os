package provider

import (
	"fmt"
	"net/http"
	"testing"
)

// -----------------------------------------------------------------------------
// F5OS 2.0.0 mock helpers
// -----------------------------------------------------------------------------
//
// These helpers consolidate the RESTCONF endpoints introduced or reshaped by
// F5OS 2.0.0 (AAA login-policy, expanded password-policy, LDAP object-class
// leaf-lists, tenant state without the pre-2.0 legacy leaves). Tests can mount
// them in one call via setupMock2_0_0AAA(mux) after calling
// setupMockPlatformVersion(mux, "2.0.0-...").
//
// The helpers are unit-test only. They never touch a real device. All response
// bodies come from fixture files so schema drift can be tracked in one place.

// setupMockPlatformVersion2_0_0 is a convenience wrapper around
// setupMockPlatformVersion(m, "2.0.0-22925"). It exists so tests that want the
// canonical 2.0.0 build string can call one helper instead of hardcoding the
// version literal, and so that a future toolchain bump to a different canonical
// build only has to be updated here.
func setupMockPlatformVersion2_0_0(m *http.ServeMux) {
	setupMockPlatformVersion(m, "2.0.0-22925")
}

// setupMock2_0_0LoginPolicy registers a GET handler that returns the
// aaa_login_policy_2_0_0.json fixture for the login-policy config endpoint.
// Callers that need to intercept PATCH (e.g., to assert the payload) should
// register their own handler on the same path AFTER this helper — Go's
// http.ServeMux panics on duplicate registrations, so callers should either
// use this helper OR register a full handler on the path, not both.
func setupMock2_0_0LoginPolicy(m *http.ServeMux) {
	m.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-openconfig-aaa-login-policy:login-policy/config",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/aaa_login_policy_2_0_0.json"))
		})
}

// setupMock2_0_0PasswordPolicy registers a GET handler that returns the
// aaa_password_policy_2_0_0.json fixture (includes v1.7 fields plus 2.0.0
// additions: min-days, remember, warn-age).
func setupMock2_0_0PasswordPolicy(m *http.ServeMux) {
	m.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-openconfig-aaa-password-policy:password-policy/config",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/aaa_password_policy_2_0_0.json"))
		})
}

// setupMock2_0_0LdapObjectClasses registers a GET handler that returns the
// LDAP container with the object-class leaf-lists. Pass empty=true to return
// the empty-list variant (aaa_ldap_object_classes_2_0_0_empty.json) that
// exercises the "cleared to empty" case the client code specifically
// distinguishes from "unmanaged" (absent leaves). Pass empty=false for the
// populated variant.
//
// The parameter is a bool rather than a string so a caller typo (e.g.,
// "emty") can no longer silently select the populated fixture.
func setupMock2_0_0LdapObjectClasses(m *http.ServeMux, empty bool) {
	fixture := "./fixtures/aaa_ldap_object_classes_2_0_0.json"
	if empty {
		fixture = "./fixtures/aaa_ldap_object_classes_2_0_0_empty.json"
	}
	m.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-openconfig-aaa-ldap:ldap",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString(fixture))
		})
}

// setupMock2_0_0AAAContainer registers a GET handler on the wide
// /restconf/data/openconfig-system:system/aaa path that serves the 2.0.0
// aaa container (f5os_auth_v2_0.json). This path is registered by many
// tests as their default auth-endpoint mock; http.ServeMux panics on
// duplicate registration, so this helper is intentionally NOT bundled
// into setupMock2_0_0AAA. Call it explicitly only when your test does
// not already register a handler on the aaa container path.
//
// The helper accepts *testing.T and turns the ServeMux's duplicate-
// registration panic into a targeted t.Fatalf so a mistaken second
// registration fails the offending test with a clear message instead
// of crashing the entire test binary.
func setupMock2_0_0AAAContainer(t *testing.T, m *http.ServeMux) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("setupMock2_0_0AAAContainer: failed to register /restconf/data/openconfig-system:system/aaa handler — this is almost always a duplicate registration; ensure no earlier setup helper already mounted this path: %v", r)
		}
	}()
	m.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth_v2_0.json"))
	})
}

// setupMock2_0_0AAA is the "everything but the aaa container" bundle for
// AAA-focused tests: platform version, login-policy config, password-policy
// config (with 2.0.0 fields), and LDAP object-class leaf-lists (populated
// variant). Call this after testAccPreUnitCheck(t) when a test needs a
// working 2.0.0-shaped AAA surface at the focused endpoints. Tests that
// need the empty LDAP variant should not use this bundle; they should call
// setupMockPlatformVersion2_0_0 and setupMock2_0_0LdapObjectClasses(m, true)
// directly.
//
// This helper deliberately does NOT register a handler on the wide
// /restconf/data/openconfig-system:system/aaa path — many callers already
// mock that endpoint themselves (e.g. via loadFixtureString("./fixtures/
// f5os_auth.json")) and http.ServeMux panics on duplicate registration.
// Tests that want the 2.0.0 aaa container served automatically should call
// setupMock2_0_0AAAContainer(t, m) explicitly, ONLY when they have not
// registered another handler on that path.
func setupMock2_0_0AAA(m *http.ServeMux) {
	setupMockPlatformVersion2_0_0(m)
	setupMock2_0_0LoginPolicy(m)
	setupMock2_0_0PasswordPolicy(m)
	setupMock2_0_0LdapObjectClasses(m, false /* populated */)
}
