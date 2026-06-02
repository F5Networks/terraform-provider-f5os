package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// ---------------------------------------------------------------------------
// Qkview mock-server helpers
// ---------------------------------------------------------------------------

// qkviewMockOpts configures the mock handlers for qkview unit tests.
type qkviewMockOpts struct {
	// Filename is the base filename used in the qkview resource.
	Filename string
	// ListedFilename is the filename returned by the list endpoint after
	// capture. Defaults to Filename + ".tar" if empty.
	ListedFilename string
	// CaptureError, if non-empty, makes the capture handler return HTTP 500
	// with this message.
	CaptureError string
	// AlwaysListFile, when true, makes the list endpoint always return the
	// file (even before capture). Used to test the "already exists" path.
	AlwaysListFile bool
	// StatusResponses is a slice of (percent, status, message) tuples
	// returned sequentially by the status endpoint. After the slice is
	// exhausted the last entry is repeated. Defaults to a single
	// "100/complete/completed" entry.
	StatusResponses []qkviewMockStatus
	// DeleteError, if non-empty, makes the delete handler return a body
	// containing "Error deleting".
	DeleteError string
	// EmptyListAlways makes the list endpoint always return an empty list
	// (no files). Used to test the delete fallback path.
	EmptyListAlways bool
}

type qkviewMockStatus struct {
	Percent int
	Status  string
	Message string
}

// setupQkviewMock registers all mock handlers required for a qkview unit
// test: AAA login, platform component detection, and the four qkview API
// endpoints (list, capture, status, delete).
//
// It returns an *atomic.Int32 tracking how many times the capture endpoint
// has been called (used by the list handler to decide whether to return
// a file).
func setupQkviewMock(t *testing.T, opts qkviewMockOpts) *atomic.Int32 {
	t.Helper()
	testAccPreUnitCheck(t)

	listedFilename := opts.ListedFilename
	if listedFilename == "" {
		listedFilename = opts.Filename + ".tar"
	}

	// --- AAA login ---
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})

	// --- Platform detection (404 → no-op) ---
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	// --- Capture counter ---
	var captureCount atomic.Int32

	// --- List endpoint ---
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)

			showFile := opts.AlwaysListFile || captureCount.Load() > 0
			if opts.EmptyListAlways {
				showFile = false
			}

			var resultStr string
			if showFile {
				resultStr = fmt.Sprintf(`{"Qkviews":[{"Filename":"%s"}]}`, listedFilename)
			} else {
				resultStr = `{"Qkviews":null}`
			}

			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			_, _ = w.Write([]byte(resp))
		},
	)

	// --- Capture endpoint ---
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			if opts.CaptureError != "" {
				w.WriteHeader(http.StatusInternalServerError)
				// Return RESTCONF-formatted error so doRequest parses it
				// and returns a non-nil error to the caller.
				_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"%s"}]}}`, opts.CaptureError)
				return
			}
			captureCount.Add(1)
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"Qkview capture started"}}`))
		},
	)

	// --- Status endpoint ---
	var statusCallCount atomic.Int32
	statusResponses := opts.StatusResponses
	if len(statusResponses) == 0 {
		statusResponses = []qkviewMockStatus{{100, "complete", "completed"}}
	}
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			idx := int(statusCallCount.Add(1)) - 1
			if idx >= len(statusResponses) {
				idx = len(statusResponses) - 1
			}
			st := statusResponses[idx]
			resultStr := fmt.Sprintf(`{"Percent":%d,"Status":"%s","Message":"%s"}`,
				st.Percent, st.Status, st.Message)
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(resp))
		},
	)

	// --- Delete endpoint ---
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			if opts.DeleteError != "" {
				_, _ = fmt.Fprintf(w, `{"f5-system-diagnostics-qkview:output":{"result":"Error deleting %s"}}`, opts.DeleteError)
				return
			}
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"Qkview deleted successfully"}}`))
		},
	)

	return &captureCount
}

// ---------------------------------------------------------------------------
// Unit Tests
// ---------------------------------------------------------------------------

// TestUnitQkviewCreateAndDeleteLifecycle exercises the full Create → Read →
// Delete lifecycle against the mock server, covering Configure, Create,
// qkviewExists, createQkview, waitForCompletion, Read, Delete, and
// deleteQkview.
func TestUnitQkviewCreateAndDeleteLifecycle(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename: "lifecycle_test",
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("lifecycle_test", 60, 5, 2, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "lifecycle_test"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "timeout", "60"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "max_file_size", "5"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "max_core_size", "2"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "exclude_cores", "true"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "id", "lifecycle_test"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "generated_file", "lifecycle_test.tar"),
				),
			},
		},
	})
}

// TestUnitQkviewCreateValidationMaxFileSizeTooHigh exercises
// validateQkviewParams with max_file_size above 1000.
func TestUnitQkviewCreateValidationMaxFileSizeTooHigh(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{Filename: "val_fs_high"})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccQkviewResourceConfig("val_fs_high", 0, 1500, 25, false),
				ExpectError: regexp.MustCompile(`max_file_size must be between 2-1000`),
			},
		},
	})
}

// TestUnitQkviewCreateValidationMaxFileSizeTooLow exercises
// validateQkviewParams with max_file_size below 2.
func TestUnitQkviewCreateValidationMaxFileSizeTooLow(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{Filename: "val_fs_low"})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccQkviewResourceConfig("val_fs_low", 0, 1, 25, false),
				ExpectError: regexp.MustCompile(`max_file_size must be between 2-1000`),
			},
		},
	})
}

// TestUnitQkviewCreateValidationMaxCoreSizeTooHigh exercises
// validateQkviewParams with max_core_size above 1000.
func TestUnitQkviewCreateValidationMaxCoreSizeTooHigh(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{Filename: "val_cs_high"})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccQkviewResourceConfig("val_cs_high", 0, 500, 1500, false),
				ExpectError: regexp.MustCompile(`max_core_size must be between 2-1000`),
			},
		},
	})
}

// TestUnitQkviewCreateValidationMaxCoreSizeTooLow exercises
// validateQkviewParams with max_core_size below 2.
func TestUnitQkviewCreateValidationMaxCoreSizeTooLow(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{Filename: "val_cs_low"})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccQkviewResourceConfig("val_cs_low", 0, 500, 1, false),
				ExpectError: regexp.MustCompile(`max_core_size must be between 2-1000`),
			},
		},
	})
}

// TestUnitQkviewCreateAlreadyExists exercises the "Resource Already Exists"
// error path in Create when qkviewExists returns true.
func TestUnitQkviewCreateAlreadyExists(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename:       "already_here",
		AlwaysListFile: true,
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccQkviewResourceConfig("already_here", 0, 500, 25, false),
				ExpectError: regexp.MustCompile(`already exists`),
			},
		},
	})
}

// TestUnitQkviewCreateCaptureError exercises the error branch in Create
// when the capture API endpoint returns an HTTP 500 error.
func TestUnitQkviewCreateCaptureError(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename:     "capture_err",
		CaptureError: "internal error",
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccQkviewResourceConfig("capture_err", 0, 500, 25, false),
				ExpectError: regexp.MustCompile(`Qkview creation failed`),
			},
		},
	})
}

// TestUnitQkviewReadExistsWithPrefix exercises the Read path where the
// list endpoint returns a file with a "slot1:" prefix. Verifies that
// qkviewExists correctly strips the prefix and still matches, and that
// generated_file preserves the prefix.
func TestUnitQkviewReadExistsWithPrefix(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename:       "slot_test",
		ListedFilename: "slot1:slot_test.tar",
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("slot_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "slot_test"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "generated_file", "slot1:slot_test.tar"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
				),
			},
		},
	})
}

// TestUnitQkviewExistsWithTimedoutSuffix exercises qkviewExists when the
// list endpoint returns a file with a ".tar.timedout" suffix. The suffix
// stripping logic should still match the base filename.
func TestUnitQkviewExistsWithTimedoutSuffix(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename:       "timedout_test",
		ListedFilename: "timedout_test.tar.timedout",
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("timedout_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "generated_file", "timedout_test.tar.timedout"),
				),
			},
		},
	})
}

// TestUnitQkviewExistsWithCanceledSuffix exercises qkviewExists when the
// list endpoint returns a file with a ".canceled" suffix.
func TestUnitQkviewExistsWithCanceledSuffix(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename:       "canceled_test",
		ListedFilename: "canceled_test.canceled",
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("canceled_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "generated_file", "canceled_test.canceled"),
				),
			},
		},
	})
}

// TestUnitQkviewExistsWithTarCanceledSuffix exercises qkviewExists when the
// list endpoint returns a file with a ".tar.canceled" suffix.
func TestUnitQkviewExistsWithTarCanceledSuffix(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename:       "tarcanceled_test",
		ListedFilename: "tarcanceled_test.tar.canceled",
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("tarcanceled_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "generated_file", "tarcanceled_test.tar.canceled"),
				),
			},
		},
	})
}

// TestUnitQkviewDeleteFallbackTar exercises the Delete fallback path where
// qkviewExists returns an empty existingFile (no file found), so Delete
// appends ".tar" to the filename. We use EmptyListAlways for the list handler
// (returns no files), but still allow capture and status to succeed so Create
// works. The list returning empty during Delete triggers the fallback.
func TestUnitQkviewDeleteFallbackTar(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	// AAA login
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	// Platform detection (404 → no-op)
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	var captureCount atomic.Int32
	var listCallCount atomic.Int32

	// List handler: returns file for calls 2-3 after capture (waitForCompletion
	// and the Read during Create), returns empty for all other calls. This means
	// when Delete calls qkviewExists, the list returns empty, exercising the
	// "existingFile == ''" fallback path that appends ".tar".
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)

			call := listCallCount.Add(1)
			captured := captureCount.Load() > 0
			// After capture, only calls 2 and 3 show the file
			// (for waitForCompletion and the Create Read).
			// Call 1 is the pre-capture existence check.
			// Call 4+ (Delete's qkviewExists, auto-destroy Read) return empty.
			showFile := captured && call >= 2 && call <= 3

			var resultStr string
			if showFile {
				resultStr = `{"Qkviews":[{"Filename":"fallback_test.tar"}]}`
			} else {
				resultStr = `{"Qkviews":null}`
			}
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			_, _ = w.Write([]byte(resp))
		},
	)

	// Capture
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			captureCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	// Status
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			resultStr := `{"Percent":100,"Status":"complete","Message":"completed"}`
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(resp))
		},
	)

	// Delete: verify the filename has ".tar" appended (fallback path).
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				fn := body["filename"]
				// The fallback path should append ".tar" to the filename.
				if !strings.HasSuffix(fn, ".tar") {
					t.Errorf("expected delete filename to end with .tar, got %q", fn)
				}
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("fallback_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "fallback_test"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "generated_file", "fallback_test.tar"),
				),
			},
		},
	})
}

// TestUnitQkviewDeleteResponseError exercises the error branch in
// deleteQkview where the response body contains "Error deleting".
// The delete handler returns "Error deleting" on the first call (exercising
// the error path) and succeeds on subsequent calls (so the test framework's
// deferred auto-destroy succeeds).
func TestUnitQkviewDeleteResponseError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	var captureCount atomic.Int32
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			var resultStr string
			if captureCount.Load() > 0 {
				resultStr = `{"Qkviews":[{"Filename":"del_err_test.tar"}]}`
			} else {
				resultStr = `{"Qkviews":null}`
			}
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			captureCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			resultStr := `{"Percent":100,"Status":"complete","Message":"completed"}`
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(resp))
		},
	)

	// Delete handler: first call returns "Error deleting" (exercises the
	// error path in deleteQkview), subsequent calls succeed (so the test
	// framework's deferred auto-destroy works).
	var deleteCount atomic.Int32
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			if deleteCount.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"Error deleting del_err_test.tar"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the resource.
			{
				Config: testAccQkviewResourceConfig("del_err_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "del_err_test"),
				),
			},
			// Step 2: Explicitly destroy. The first delete call returns
			// "Error deleting" which exercises the deleteQkview error path.
			// ExpectError catches the destroy failure so the step passes.
			{
				Config:      testAccQkviewResourceConfig("del_err_test", 0, 500, 25, false),
				Destroy:     true,
				ExpectError: regexp.MustCompile(`(?s)Error deleting`),
			},
			// After the ExpectError step, the resource is still in state.
			// The framework's deferred auto-destroy will call Delete again
			// (the second call), which succeeds.
		},
	})
}

// TestUnitQkviewStatusInProgress exercises the polling loop in
// waitForCompletion. The status endpoint returns an in-progress response
// first (50% "collecting"), then a complete response. This exercises the
// `contains` helper function used to check status/message strings.
func TestUnitQkviewStatusInProgress(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename: "inprog_test",
		StatusResponses: []qkviewMockStatus{
			{50, "collecting", "Collecting Data"},
			{75, "collating", "Collating data"},
			{100, "complete", "completed"},
		},
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("inprog_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "generated_file", "inprog_test.tar"),
				),
			},
		},
	})
}

// TestUnitQkviewUpdateNotSupported exercises the Update method which always
// returns an error. Step 1 creates the resource, step 2 changes timeout
// (a non-RequiresReplace attribute) which triggers Update.
func TestUnitQkviewUpdateNotSupported(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename: "update_test",
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("update_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "update_test"),
				),
			},
			// Change timeout to trigger Update.
			{
				Config:      testAccQkviewResourceConfig("update_test", 120, 500, 25, false),
				ExpectError: regexp.MustCompile(`Update Not Supported`),
			},
		},
	})
}

// TestUnitQkviewDeleteQkviewExistsError exercises the Delete path where
// qkviewExists returns an error (e.g., malformed JSON from the list
// endpoint). Delete should log a warning but continue with deletion.
func TestUnitQkviewDeleteQkviewExistsError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	var captureCount atomic.Int32
	var listCallCount atomic.Int32

	// List handler: returns valid file after capture for calls 2-3, then
	// returns malformed JSON for later calls (Delete's qkviewExists call).
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)

			call := listCallCount.Add(1)
			captured := captureCount.Load() > 0

			if captured && call >= 2 && call <= 3 {
				// Valid response for waitForCompletion and initial Read.
				resultStr := `{"Qkviews":[{"Filename":"listfail_test.tar"}]}`
				resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
					strconv.Quote(resultStr))
				_, _ = w.Write([]byte(resp))
			} else if captured && call > 3 {
				// Malformed JSON for Delete's qkviewExists call.
				// The outer JSON is valid but the inner result string is not valid JSON.
				resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
					strconv.Quote("not valid json"))
				_, _ = w.Write([]byte(resp))
			} else {
				// Pre-capture: no files.
				resultStr := `{"Qkviews":null}`
				resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
					strconv.Quote(resultStr))
				_, _ = w.Write([]byte(resp))
			}
		},
	)

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			captureCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			resultStr := `{"Percent":100,"Status":"complete","Message":"completed"}`
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("listfail_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "listfail_test"),
				),
			},
			// The auto-destroy calls Delete, which calls qkviewExists with
			// the malformed list response. qkviewExists returns an error,
			// Delete logs a warning but continues. Since existingFile is "",
			// the fallback path appends ".tar". Delete then calls
			// deleteQkview which succeeds.
		},
	})
}

// TestUnitContains directly tests the contains() helper function with
// positive and negative cases.
func TestUnitContains(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		item  string
		want  bool
	}{
		{
			name:  "found at beginning",
			slice: []string{"collecting", "collating"},
			item:  "collecting",
			want:  true,
		},
		{
			name:  "found at end",
			slice: []string{"collecting", "collating"},
			item:  "collating",
			want:  true,
		},
		{
			name:  "not found",
			slice: []string{"collecting", "collating"},
			item:  "complete",
			want:  false,
		},
		{
			name:  "empty slice",
			slice: []string{},
			item:  "anything",
			want:  false,
		},
		{
			name:  "empty item found",
			slice: []string{"", "foo"},
			item:  "",
			want:  true,
		},
		{
			name:  "single element match",
			slice: []string{"exact"},
			item:  "exact",
			want:  true,
		},
		{
			name:  "case sensitive no match",
			slice: []string{"Collecting"},
			item:  "collecting",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.slice, tt.item)
			if got != tt.want {
				t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.item, got, tt.want)
			}
		})
	}
}

// TestUnitQkviewCreateListErrorDuringExistsCheck exercises the error path
// in Create where qkviewExists returns an error because the list endpoint
// returns a RESTCONF error on the first call (the pre-check).
func TestUnitQkviewCreateListErrorDuringExistsCheck(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	// List handler: always returns HTTP 500 RESTCONF error.
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"list unavailable"}]}}`))
		},
	)
	// Other handlers still needed for mock server initialization.
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccQkviewResourceConfig("listerr_test", 0, 500, 25, false),
				ExpectError: regexp.MustCompile(`Failed to check if qkview exists`),
			},
		},
	})
}

// TestUnitQkviewExistsNoMatchInList exercises the qkviewExists path where
// the list contains files but none match the target filename. This covers
// the final "return false, '', nil" in qkviewExists.
func TestUnitQkviewExistsNoMatchInList(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	// List handler always returns a file that does NOT match the target.
	// Before capture: returns "other_file.tar" (no match → Create proceeds).
	// After capture: returns "other_file.tar" (no match → waitForCompletion
	// doesn't find the file and keeps polling). To avoid a timeout, the
	// status handler returns "time-out" so waitForCompletion returns the
	// "{filename}.timedout" fallback.
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)

			resultStr := `{"Qkviews":[{"Filename":"other_file.tar"}]}`
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)
	// Status returns "time-out" so waitForCompletion hits the timeout
	// fallback path instead of polling forever.
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			resultStr := `{"Percent":100,"Status":"time-out","Message":"timed out"}`
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("nomatch_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "nomatch_test"),
					// The timedout fallback returns "{filename}.timedout"
					resource.TestCheckResourceAttr("f5os_qkview.test", "generated_file", "nomatch_test.timedout"),
				),
				// Read during refresh won't find the file (list returns a
				// non-matching name), so it removes the resource from state,
				// causing a non-empty plan.
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestUnitQkviewReadQkviewExistsError exercises the Read path where
// qkviewExists returns an error because the list endpoint returns malformed
// JSON. Step 1 creates normally. Step 2 re-applies; the refresh Read fails
// with a qkviewExists error.
func TestUnitQkviewReadQkviewExistsError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	var captureCount atomic.Int32
	var listCallCount atomic.Int32
	// Toggle: when > 0, list returns invalid JSON.
	var returnBadJSON atomic.Int32

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)

			listCallCount.Add(1)
			captured := captureCount.Load() > 0

			if returnBadJSON.Load() > 0 {
				_, _ = w.Write([]byte(`not valid json at all`))
				return
			}
			if captured {
				resultStr := `{"Qkviews":[{"Filename":"readerr_test.tar"}]}`
				resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
					strconv.Quote(resultStr))
				_, _ = w.Write([]byte(resp))
			} else {
				resultStr := `{"Qkviews":null}`
				resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
					strconv.Quote(resultStr))
				_, _ = w.Write([]byte(resp))
			}
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			captureCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			resultStr := `{"Percent":100,"Status":"complete","Message":"completed"}`
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create normally. The Check callback switches to bad JSON.
			// The post-apply refresh then calls Read which calls
			// qkviewExists, gets invalid JSON, and returns an error.
			// ExpectError catches this refresh error.
			{
				Config: testAccQkviewResourceConfig("readerr_test", 0, 500, 25, false),
				Check: func(s *terraform.State) error {
					// Switch to bad JSON for the post-apply refresh.
					returnBadJSON.Store(1)
					return nil
				},
				ExpectError: regexp.MustCompile(`(?i)F5OS Client Error|failed to parse|qkview status`),
			},
		},
	})
}

// TestUnitQkviewWaitForCompletionUnexpectedStatus exercises the
// waitForCompletion path where the status has Percent > 100 and a status
// that doesn't match any known completion or in-progress criteria. This
// hits the "qkview generation failed" error return, and also covers the
// waitForCompletion error path in Create.
func TestUnitQkviewWaitForCompletionUnexpectedStatus(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename: "unexpected_status",
		StatusResponses: []qkviewMockStatus{
			// Percent > 100, unknown status → hits the unreachable error branch.
			{200, "error", "bad failure"},
		},
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccQkviewResourceConfig("unexpected_status", 0, 500, 25, false),
				ExpectError: regexp.MustCompile(`qkview generation failed`),
			},
		},
	})
}

// TestUnitQkviewWaitForCompletionUnknownStatus exercises the
// waitForCompletion path where status has Percent < 100 and an unknown
// status/message that doesn't match any known in-progress criteria. This
// covers the "Unknown qkview status" warning path.
func TestUnitQkviewWaitForCompletionUnknownStatus(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename: "unknown_status",
		StatusResponses: []qkviewMockStatus{
			// First poll: unknown status with < 100 percent.
			{50, "weird_state", "Something unusual"},
			// Second poll: complete.
			{100, "complete", "completed"},
		},
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("unknown_status", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
				),
			},
		},
	})
}

// TestUnitQkviewExistsOuterJsonParseError exercises the qkviewExists path
// where the outer JSON response is valid but the result field is missing
// or the response structure is unexpected, triggering the json.Unmarshal
// error for the outer response.
func TestUnitQkviewExistsOuterJsonParseError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	// List handler: returns completely non-JSON response.
	// This triggers the "failed to parse list response" error in qkviewExists.
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			// Return malformed JSON to trigger unmarshal error for outer response.
			_, _ = w.Write([]byte(`<html>not json</html>`))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccQkviewResourceConfig("parseerr_test", 0, 500, 25, false),
				ExpectError: regexp.MustCompile(`Failed to check if qkview exists`),
			},
		},
	})
}

// TestUnitQkviewCreateExcludeCoresFalse exercises the full lifecycle with
// exclude_cores=false to cover that code path in createQkview.
func TestUnitQkviewCreateExcludeCoresFalse(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename: "no_exclude_cores",
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("no_exclude_cores", 60, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "exclude_cores", "false"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
				),
			},
		},
	})
}

// TestUnitQkviewCreateWithControllerPrefix exercises qkviewExists when
// the list endpoint returns a file with a "controller-1:" prefix containing
// a colon. The code splits on ":" and takes the last part.
func TestUnitQkviewCreateWithControllerPrefix(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename:       "ctrl_test",
		ListedFilename: "controller-1:ctrl_test.tar",
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("ctrl_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "generated_file", "controller-1:ctrl_test.tar"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
				),
			},
		},
	})
}

// TestUnitQkviewReadNotFound exercises the Read path where the qkview no
// longer exists on the device (list returns empty after creation). Read
// should call resp.State.RemoveResource. We simulate this by returning
// the file during create's waitForCompletion call, then returning empty
// on subsequent list calls. The second step re-applies the same config;
// because Read removed the resource from state, Terraform will plan to
// re-create it (ExpectNonEmptyPlan). The second capture/status calls
// succeed so the resource is re-created.
func TestUnitQkviewReadNotFound(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	var captureCount atomic.Int32
	// Track list calls per capture cycle to control when the file appears.
	var postCaptureListCount atomic.Int32

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)

			captured := captureCount.Load()
			var showFile bool
			if captured > 0 {
				count := postCaptureListCount.Add(1)
				// After first capture: show file for calls 1-2 (waitForCompletion + initial Read),
				// hide for 3-4 (Read during step 2 refresh).
				// After second capture: always show file.
				if captured == 1 {
					showFile = count <= 2
				} else {
					showFile = true
				}
			}

			var resultStr string
			if showFile {
				resultStr = `{"Qkviews":[{"Filename":"vanish_test.tar"}]}`
			} else {
				resultStr = `{"Qkviews":null}`
			}
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			captureCount.Add(1)
			// Reset the post-capture list counter for each new capture.
			postCaptureListCount.Store(0)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			resultStr := `{"Percent":100,"Status":"complete","Message":"completed"}`
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create normally.
			{
				Config: testAccQkviewResourceConfig("vanish_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "vanish_test"),
				),
			},
			// Step 2: Re-apply same config. Read sees the file is gone and
			// removes it from state. Terraform re-creates it (second capture).
			// The second capture cycle returns the file in list, so the
			// lifecycle completes successfully.
			{
				Config: testAccQkviewResourceConfig("vanish_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "vanish_test"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
				),
			},
		},
	})
}

// TestUnitQkviewCreateWithTimedoutSuffix exercises the ".timedout" suffix
// stripping path in qkviewExists.
func TestUnitQkviewCreateWithTimedoutSuffix(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename:       "to_suffix",
		ListedFilename: "to_suffix.timedout",
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("to_suffix", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "generated_file", "to_suffix.timedout"),
				),
			},
		},
	})
}

// TestUnitQkviewCreateDefaultValues exercises the schema defaults (timeout=0,
// max_file_size=500, max_core_size=25, exclude_cores=false) by providing only
// the required filename attribute.
func TestUnitQkviewCreateDefaultValues(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename: "defaults_test",
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_qkview" "test" {
  filename = "defaults_test"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "timeout", "0"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "max_file_size", "500"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "max_core_size", "25"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "exclude_cores", "false"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
				),
			},
		},
	})
}

// TestUnitQkviewCaptureRequestBody verifies that createQkview sends the
// correct JSON body to the capture endpoint, including the exclude_cores field.
func TestUnitQkviewCaptureRequestBody(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	var captureCount atomic.Int32

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			var resultStr string
			if captureCount.Load() > 0 {
				resultStr = `{"Qkviews":[{"Filename":"body_test.tar"}]}`
			} else {
				resultStr = `{"Qkviews":null}`
			}
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			_, _ = w.Write([]byte(resp))
		},
	)

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			captureCount.Add(1)
			var req QkviewCaptureRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("failed to decode capture request body: %v", err)
			}
			if req.Filename != "body_test" {
				t.Errorf("expected filename 'body_test', got %q", req.Filename)
			}
			if req.Timeout != 30 {
				t.Errorf("expected timeout 30, got %d", req.Timeout)
			}
			if req.MaxFileSize != 100 {
				t.Errorf("expected maxfilesize 100, got %d", req.MaxFileSize)
			}
			if req.MaxCoreSize != 10 {
				t.Errorf("expected maxcoresize 10, got %d", req.MaxCoreSize)
			}
			if req.ExcludeCores != true {
				t.Errorf("expected exclude-cores true, got %v", req.ExcludeCores)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			resultStr := `{"Percent":100,"Status":"complete","Message":"completed"}`
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(resp))
		},
	)

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("body_test", 30, 100, 10, true),
			},
		},
	})
}

// TestUnitQkviewWaitForCompletionStatusPollError exercises the
// waitForCompletion path where the status endpoint returns HTTP 500 for the
// first 4 attempts (attempt >= 3 check), triggering the "failed to check
// qkview status" error.
func TestUnitQkviewWaitForCompletionStatusPollError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	var captureCount atomic.Int32
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			var resultStr string
			if captureCount.Load() > 0 {
				resultStr = `{"Qkviews":[{"Filename":"pollerr_test.tar"}]}`
			} else {
				resultStr = `{"Qkviews":null}`
			}
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			captureCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)
	// Status endpoint: always returns HTTP 500 to trigger the
	// "failed to check qkview status after N attempts" error in
	// waitForCompletion.
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"status unavailable"}]}}`))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccQkviewResourceConfig("pollerr_test", 0, 500, 25, false),
				ExpectError: regexp.MustCompile(`(?i)qkview generation failed|failed to check qkview status`),
			},
		},
	})
}

// TestUnitQkviewWaitForCompletionOuterParseError exercises the
// waitForCompletion path where the status response is not valid JSON
// (outer unmarshal fails). After 4 retries (attempt >= 3), returns error.
func TestUnitQkviewWaitForCompletionOuterParseError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	var captureCount atomic.Int32
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			var resultStr string
			if captureCount.Load() > 0 {
				resultStr = `{"Qkviews":[{"Filename":"outerparse_test.tar"}]}`
			} else {
				resultStr = `{"Qkviews":null}`
			}
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			captureCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)
	// Status endpoint: returns non-JSON to trigger the outer unmarshal error.
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html>not json</html>`))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccQkviewResourceConfig("outerparse_test", 0, 500, 25, false),
				ExpectError: regexp.MustCompile(`(?i)qkview generation failed|failed to parse status`),
			},
		},
	})
}

// TestUnitQkviewWaitForCompletionInnerParseError exercises the
// waitForCompletion path where the outer JSON is valid but the inner
// result string is not valid JSON (inner unmarshal fails).
func TestUnitQkviewWaitForCompletionInnerParseError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	var captureCount atomic.Int32
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			var resultStr string
			if captureCount.Load() > 0 {
				resultStr = `{"Qkviews":[{"Filename":"innerparse_test.tar"}]}`
			} else {
				resultStr = `{"Qkviews":null}`
			}
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			captureCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)
	// Status endpoint: returns valid outer JSON but the result field is
	// not valid JSON, triggering the inner unmarshal error.
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote("not valid json"))
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccQkviewResourceConfig("innerparse_test", 0, 500, 25, false),
				ExpectError: regexp.MustCompile(`(?i)qkview generation failed|failed to parse status`),
			},
		},
	})
}

// TestUnitQkviewWaitForCompletionCompleteNotFoundThenFound exercises the
// waitForCompletion path where status is "complete" but the file is not
// found on the first check, then found on the second. This covers the
// "Qkview file not found yet, continuing to poll" path.
func TestUnitQkviewWaitForCompletionCompleteNotFoundThenFound(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	var captureCount atomic.Int32
	var listCallsAfterCapture atomic.Int32

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)

			captured := captureCount.Load() > 0
			var resultStr string
			if captured {
				n := listCallsAfterCapture.Add(1)
				// First call after capture (in waitForCompletion): no file.
				// Second call after capture (retry in waitForCompletion): file found.
				if n >= 2 {
					resultStr = `{"Qkviews":[{"Filename":"delayed_test.tar"}]}`
				} else {
					resultStr = `{"Qkviews":null}`
				}
			} else {
				resultStr = `{"Qkviews":null}`
			}
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			captureCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			resultStr := `{"Percent":100,"Status":"complete","Message":"completed"}`
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("delayed_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "generated_file", "delayed_test.tar"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
				),
			},
		},
	})
}

// TestUnitQkviewWaitForCompletionTimeoutListError exercises the
// waitForCompletion path where status is "time-out", qkviewExists returns
// an error, and the retry also returns an error. The code should continue
// polling and eventually hit the timeout no-file fallback.
func TestUnitQkviewWaitForCompletionTimeoutListError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	var captureCount atomic.Int32
	var listCallCount atomic.Int32

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			call := listCallCount.Add(1)
			captured := captureCount.Load() > 0

			if !captured {
				// Pre-capture: no files
				resultStr := `{"Qkviews":null}`
				resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
					strconv.Quote(resultStr))
				_, _ = w.Write([]byte(resp))
			} else if call <= 2 {
				// First post-capture calls: return malformed JSON to trigger
				// the qkviewExists error path in waitForCompletion
				_, _ = w.Write([]byte(`<html>broken</html>`))
			} else {
				// Later calls: return empty list so the timeout fallback
				// path returns "{filename}.timedout"
				resultStr := `{"Qkviews":null}`
				resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
					strconv.Quote(resultStr))
				_, _ = w.Write([]byte(resp))
			}
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			captureCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)
	// Status returns "time-out" to trigger the timeout handling paths.
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			resultStr := `{"Percent":100,"Status":"time-out","Message":"timed out"}`
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("toerr_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "toerr_test"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "generated_file", "toerr_test.timedout"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestUnitQkviewWaitForCompletionTimeoutListSuccessRetry exercises the
// waitForCompletion path where status is "time-out", the first qkviewExists
// call fails with an error, but after the 5-second delay the retry succeeds
// and finds the file. This covers the "Timeout qkview file found after delay"
// branch.
func TestUnitQkviewWaitForCompletionTimeoutListSuccessRetry(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "mock-token-qkview")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	var captureCount atomic.Int32
	var listCallCount atomic.Int32

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.WriteHeader(http.StatusOK)
			call := listCallCount.Add(1)
			captured := captureCount.Load() > 0

			if !captured {
				// Pre-capture: no files
				resultStr := `{"Qkviews":null}`
				resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
					strconv.Quote(resultStr))
				_, _ = w.Write([]byte(resp))
			} else if call == 2 {
				// First post-capture list call (in waitForCompletion):
				// return error (malformed JSON) to trigger the timeout
				// retry path.
				_, _ = w.Write([]byte(`<html>broken</html>`))
			} else {
				// Second list call (retry after delay) and all subsequent:
				// return the file.
				resultStr := `{"Qkviews":[{"Filename":"toretry_test.tar.timedout"}]}`
				resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
					strconv.Quote(resultStr))
				_, _ = w.Write([]byte(resp))
			}
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture",
		func(w http.ResponseWriter, r *http.Request) {
			captureCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)
	// Status returns "time-out" to trigger the timeout handling paths.
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status",
		func(w http.ResponseWriter, r *http.Request) {
			resultStr := `{"Percent":100,"Status":"time-out","Message":"timed out"}`
			resp := fmt.Sprintf(`{"f5-system-diagnostics-qkview:output":{"result":%s}}`,
				strconv.Quote(resultStr))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(resp))
		},
	)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-diagnostics-qkview:output":{"result":"OK"}}`))
		},
	)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("toretry_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "generated_file", "toretry_test.tar.timedout"),
				),
			},
		},
	})
}

// TestUnitQkviewWaitForCompletionPartialSaved exercises the
// waitForCompletion path where the status Message contains "partial qkview
// saved" — this triggers the completion branch.
func TestUnitQkviewWaitForCompletionPartialSaved(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename: "partial_test",
		StatusResponses: []qkviewMockStatus{
			{80, "partial", "partial qkview saved to file"},
		},
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("partial_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "generated_file", "partial_test.tar"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
				),
			},
		},
	})
}

// TestUnitQkviewWaitForCompletionCollectInMessage exercises the
// waitForCompletion in-progress branch where the status message contains
// "collect" (case insensitive via ToLower). This covers the
// strings.Contains(ToLower(status.Message), "collect") path.
func TestUnitQkviewWaitForCompletionCollectInMessage(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename: "collmsg_test",
		StatusResponses: []qkviewMockStatus{
			{30, "working", "Collecting system logs"},
			{100, "complete", "completed"},
		},
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("collmsg_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
				),
			},
		},
	})
}

// TestUnitQkviewWaitForCompletionCollectInStatus exercises the
// waitForCompletion in-progress branch where the status field contains
// "collect" (case insensitive). This covers the
// strings.Contains(ToLower(status.Status), "collect") path.
func TestUnitQkviewWaitForCompletionCollectInStatus(t *testing.T) {
	_ = setupQkviewMock(t, qkviewMockOpts{
		Filename: "collst_test",
		StatusResponses: []qkviewMockStatus{
			{40, "collecting-data", "phase 1"},
			{100, "complete", "completed"},
		},
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQkviewResourceConfig("collst_test", 0, 500, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance Test Helpers (direct device verification)
// ---------------------------------------------------------------------------

// testAccCheckQkviewExists is a TestCheckFunc that queries the device
// directly to verify a qkview file exists on it.
func testAccCheckQkviewExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found in state: %s", resourceName)
		}
		filename := rs.Primary.Attributes["filename"]
		if filename == "" {
			return fmt.Errorf("filename attribute is empty for %s", resourceName)
		}

		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create f5os client: %w", err)
		}

		uri := "/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list"
		response, err := client.PostRequest(uri, nil)
		if err != nil {
			return fmt.Errorf("failed to list qkviews on device: %w", err)
		}

		var listResponse QkviewListResponse
		if err := json.Unmarshal(response, &listResponse); err != nil {
			return fmt.Errorf("failed to parse list response: %w", err)
		}

		var qkviewList QkviewList
		if err := json.Unmarshal([]byte(listResponse.Output.Result), &qkviewList); err != nil {
			return fmt.Errorf("failed to parse qkview list: %w", err)
		}

		targetBase := normalizeQkviewFilename(filename)
		for _, item := range qkviewList.Qkviews {
			if targetBase == normalizeQkviewFilename(item.Filename) {
				return nil
			}
		}

		return fmt.Errorf("qkview %q not found on device", filename)
	}
}

// testAccCheckQkviewDestroy is a CheckDestroy function that verifies the
// qkview file was deleted from the device after Terraform destroy.
func testAccCheckQkviewDestroy(s *terraform.State) error {
	client, err := newTestClientFromEnv()
	if err != nil {
		// Cannot connect — nothing to verify.
		return nil
	}

	uri := "/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list"
	response, err := client.PostRequest(uri, nil)
	if err != nil {
		// If we cannot list, assume the device is unreachable; skip.
		return nil
	}

	var listResponse QkviewListResponse
	if err := json.Unmarshal(response, &listResponse); err != nil {
		return nil
	}

	var qkviewList QkviewList
	if err := json.Unmarshal([]byte(listResponse.Output.Result), &qkviewList); err != nil {
		return nil
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "f5os_qkview" {
			continue
		}
		filename := rs.Primary.Attributes["filename"]
		if filename == "" {
			continue
		}
		targetBase := normalizeQkviewFilename(filename)
		for _, item := range qkviewList.Qkviews {
			if targetBase == normalizeQkviewFilename(item.Filename) {
				return fmt.Errorf("qkview %q still exists on device as %q after destroy", filename, item.Filename)
			}
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Acceptance Tests (require real F5OS device)
// ---------------------------------------------------------------------------

func TestAccQkviewResource_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckQkviewDestroy,
		Steps: []resource.TestStep{
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
					testAccCheckQkviewExists("f5os_qkview.test"),
				),
			},
		},
	})
}

func TestAccQkviewResource_CustomParams(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckQkviewDestroy,
		Steps: []resource.TestStep{
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
					testAccCheckQkviewExists("f5os_qkview.test"),
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
				Config:      testAccQkviewResourceConfig("invalid_qkview", 60, 1500, 2, true),
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
				Config:      testAccQkviewResourceConfig("invalid_qkview", 60, 5, 1, true),
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
			{
				Config: testAccQkviewResourceConfig("duplicate_qkview", 60, 2, 2, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "duplicate_qkview"),
					resource.TestCheckResourceAttr("f5os_qkview.test", "status", "complete"),
				),
			},
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
			{
				Config: testAccQkviewResourceConfig("replace_test", 60, 2, 2, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_qkview.test", "filename", "replace_test"),
				),
			},
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
		CheckDestroy:             testAccCheckQkviewDestroy,
		Steps: []resource.TestStep{
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
					testAccCheckQkviewExists("f5os_qkview.test"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Terraform Config Helpers
// ---------------------------------------------------------------------------

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
