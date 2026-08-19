package provider

import (
	"context"
	"net/http"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestUnitTlsCertKeyReadClearsStateCertificatePre200 is a regression
// guard for the pre-2.0 state cleanup contract in Read
// (f5os_tls_cert_key_resource.go, pre-2.0 branch).
//
// Contract: on a device running < 2.0.0, Read must set
// state_certificate to null unconditionally. Previously the code only
// nulled the leaf when it was Unknown, so a value populated by an
// earlier 2.0.0+ read (or a device swap / downgrade) could linger
// indefinitely and mask real drift — the exact scenario support
// engineers hit when a state_certificate value persists after the
// device has been rolled back to a pre-2.0.0 image.
//
// The test seeds state with a populated state_certificate, points the
// resource at a mock that reports platform version 1.8.x, invokes
// Read directly, and asserts the leaf comes back null.
func TestUnitTlsCertKeyReadClearsStateCertificatePre200(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	// Mock a pre-2.0 rSeries device so client.PlatformVersion falls
	// into the pre-2.0 branch of Read.
	setupMockPlatformVersion(mux, "1.8.3-23453")
	tlsCertKeyMockAuth(t)

	// On a pre-2.0 device, Read must NOT hit the aaa-tls tls
	// container. If it does, fail loudly — this guards the version
	// gate itself.
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-openconfig-aaa-tls:tls",
		func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("Read unexpectedly issued %s to aaa-tls tls container on a pre-2.0 device", r.Method)
			w.WriteHeader(http.StatusInternalServerError)
		})

	client, err := newTestClientFromEnv()
	if err != nil {
		t.Fatalf("failed to create test client against mock: %s", err)
	}
	if platformVersionAtLeast(client.PlatformVersion, "v2.0") {
		t.Fatalf("mock reported %q; expected pre-2.0", client.PlatformVersion)
	}

	r := &PartitionCertKeyResource{client: client}

	// Fetch the schema so we can construct a tfsdk.State with the
	// right Terraform type.
	schemaResp := &fwresource.SchemaResponse{}
	r.Schema(context.Background(), fwresource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Schema returned diagnostics: %v", schemaResp.Diagnostics)
	}
	sch := schemaResp.Schema

	// Seed initial state: state_certificate is populated with a stale
	// value (as it would be after a prior 2.0.0+ read).
	initial := PartitionCertKeyResourceModel{
		Name:             types.StringValue("legacy-cert"),
		Id:               types.StringValue("legacy-cert"),
		DaysValid:        types.Int64Value(30),
		Version:          types.Int64Value(1),
		StateCertificate: types.StringValue("-----BEGIN CERTIFICATE-----\nSTALE-FROM-2.0\n-----END CERTIFICATE-----\n"),
	}

	ctx := context.Background()
	state := tfsdk.State{
		Schema: sch,
		Raw:    tftypes.NewValue(sch.Type().TerraformType(ctx), nil),
	}
	if diags := state.Set(ctx, &initial); diags.HasError() {
		t.Fatalf("state.Set(initial) returned diagnostics: %v", diags)
	}

	req := fwresource.ReadRequest{State: state}
	resp := &fwresource.ReadResponse{State: state}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read returned error diagnostics: %v", resp.Diagnostics)
	}

	var got PartitionCertKeyResourceModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("failed to read back state: %v", diags)
	}

	if !got.StateCertificate.IsNull() {
		t.Errorf("expected state_certificate to be null after pre-2.0 Read, got %q",
			got.StateCertificate.ValueString())
	}
	// Sanity: other fields should be preserved by Read (Read only
	// modifies the state_certificate leaf on the pre-2.0 branch).
	if got.Name.ValueString() != "legacy-cert" {
		t.Errorf("expected name preserved, got %q", got.Name.ValueString())
	}
}
