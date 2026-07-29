package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

var (
	_ resource.Resource                = &PartitionCertKeyResource{}
	_ resource.ResourceWithImportState = &PartitionCertKeyResource{}
)

func NewPartitionCertKeyResource() resource.Resource {
	return &PartitionCertKeyResource{}
}

type PartitionCertKeyResource struct {
	client   *f5ossdk.F5os
	teemData *TeemData
}

type PartitionCertKeyResourceModel struct {
	Name                   types.String `tfsdk:"name"`
	SubjectAlternativeName types.String `tfsdk:"subject_alternative_name"`
	DaysValid              types.Int64  `tfsdk:"days_valid"`
	Email                  types.String `tfsdk:"email"`
	City                   types.String `tfsdk:"city"`
	Province               types.String `tfsdk:"province"`
	Country                types.String `tfsdk:"country"`
	Organization           types.String `tfsdk:"organization"`
	Unit                   types.String `tfsdk:"unit"`
	Version                types.Int64  `tfsdk:"version"`
	KeyType                types.String `tfsdk:"key_type"`
	KeySize                types.Int64  `tfsdk:"key_size"`
	KeyCurve               types.String `tfsdk:"key_curve"`
	KeyPassphrase          types.String `tfsdk:"key_passphrase"`
	ConfirmKeyPassphrase   types.String `tfsdk:"confirm_key_passphrase"`
	// F5OS 2.0.0+ additive: supply an existing certificate/key pair
	// instead of generating a self-signed cert. When either is set
	// the resource takes the "import" path and skips the
	// create-self-signed-cert RPC.
	Certificate      types.String `tfsdk:"certificate"`
	Key              types.String `tfsdk:"key"`
	StateCertificate types.String `tfsdk:"state_certificate"`
	Id               types.String `tfsdk:"id"`
}

func (r *PartitionCertKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tls_cert_key"
}

func (r *PartitionCertKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Resource used to manage tls cert and key on F5OS partitions",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the tls certificate.",
			},
			"subject_alternative_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The subject alternative name of the tls certificate. This attribute is required for F5OS v1.8 and above and not supported for F5OS below v1.8",
			},
			"days_valid": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(30),
				MarkdownDescription: "The number of days for which the certificate is valid, the default value is 30 days",
			},
			"email": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The email address of the certificate holder.",
			},
			"city": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The residing cty of the certificate holder.",
			},
			"province": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The residing province of the certificate holder.",
			},
			"country": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The residing country of the certificate holder.",
			},
			"organization": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The organization of the certificate holder",
			},
			"unit": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The organizational unit of the certificate holder.",
			},
			"version": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
				MarkdownDescription: "The version of the certificate",
			},
			"key_type": schema.StringAttribute{
				Optional:            true,
				Validators:          []validator.String{stringvalidator.OneOf("rsa", "ecdsa", "encrypted-rsa", "encrypted-ecdsa")},
				MarkdownDescription: "The type of the tls key",
			},
			"key_size": schema.Int64Attribute{
				Optional:            true,
				Validators:          []validator.Int64{int64validator.OneOf(2048, 3072, 4096)},
				MarkdownDescription: "This specifies the length of the key, this is only applicable for RSA keys. This attribute is required when `key_type` is set to `rsa` or `encrypted-rsa`",
			},
			"key_curve": schema.StringAttribute{
				Optional:            true,
				Validators:          []validator.String{stringvalidator.OneOf("prime256v1", "secp384r1")},
				MarkdownDescription: "This specifies the specific elliptic curve used in ECC, this is only applicable for ECDSA keys. This attribute is required when `key_type` is set to `ecdsa` or `encrypted-ecdsa`",
			},
			"key_passphrase": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "This specifies the passphrase for the key. This attribute is required when `key_type` is set to `encrypted-rsa` or `encrypted-ecdsa`",
				Sensitive:           true,
			},
			"confirm_key_passphrase": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "This specifies the confirmation of the passphrase for the key, the value should be the same as the `key_passphrase`. This attribute is required when `key_type` is set to `encrypted-rsa` or `encrypted-ecdsa`",
				Sensitive:           true,
			},
			"certificate": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "PEM-encoded certificate to import into the F5OS aaa-tls tls container. " +
					"Setting `certificate` (and optionally `key`) switches the resource into import mode: " +
					"the `create-self-signed-cert` RPC is skipped and the supplied material is PATCHed onto " +
					"`config.certificate`. The device may canonicalize the PEM formatting; drift caused by " +
					"canonicalization is not reflected on the `certificate` attribute — inspect " +
					"`state_certificate` for the device-reported value. Only supported on F5OS 2.0.0 or later.",
			},
			"key": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "PEM-encoded private key that pairs with `certificate`. Sent to the device via " +
					"PATCH of `config.key`; the device never returns this leaf on Read, so drift cannot be " +
					"detected automatically. Only supported on F5OS 2.0.0 or later.",
				Sensitive: true,
			},
			"state_certificate": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "PEM-encoded certificate reported by the device under `state.certificate`. " +
					"Populated by Read on F5OS 2.0.0+; empty on older devices.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique resource identifier",
			},
		},
	}
}

func (r *PartitionCertKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
	teemData.ProviderName = "f5os"
	teemData.ResourceName = "f5os_partition_cert_key"
	r.teemData = teemData
}

// isImportMode returns true when the caller populated the F5OS 2.0.0+
// certificate/key leaves in the plan, signaling that the resource
// should PATCH an existing cert/key rather than call
// create-self-signed-cert.
//
// Any known (non-null, non-unknown) value on certificate or key opts
// into import mode — including an explicit empty string. This routes
// bad input (e.g., `certificate = ""`) into applyImport, which
// version-gates and then relies on ImportTlsCertKey to return a
// deterministic "at least one of certificate or key must be
// non-empty" error, rather than silently falling into the self-signed
// workflow and restarting the HTTPS service.
func isImportMode(data *PartitionCertKeyResourceModel) bool {
	certSet := !data.Certificate.IsNull() && !data.Certificate.IsUnknown()
	keySet := !data.Key.IsNull() && !data.Key.IsUnknown()
	return certSet || keySet
}

func (r *PartitionCertKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *PartitionCertKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if isImportMode(data) {
		r.applyImport(ctx, data, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
		return
	}

	if platformVersionAtLeast(r.client.PlatformVersion, "v1.8") {
		if data.SubjectAlternativeName.IsNull() || data.SubjectAlternativeName.IsUnknown() {
			resp.Diagnostics.AddError("subject_alternative_name is required for platform version v1.8 and above", "")
			return
		}
	} else {
		if !data.SubjectAlternativeName.IsNull() || data.SubjectAlternativeName.IsUnknown() {
			resp.Diagnostics.AddError("subject_alternative_name is not supported for platform version below v1.8", "")
			return
		}
	}

	tlsConfig := getTLSConfig(data)

	err := r.client.CreateTlsCertKey(tlsConfig)

	if err != nil {
		resp.Diagnostics.AddError("Failed to create partition cert key", err.Error())
		return
	}

	// Creating a self-signed cert restarts the F5OS HTTPS service. Wait
	// briefly so subsequent API calls (e.g., Terraform's post-apply refresh)
	// find the service available.
	if err := waitForTLSService(ctx, r.client); err != nil {
		resp.Diagnostics.AddWarning("RESTCONF service may still be restarting", err.Error())
	}

	data.Id = types.StringValue(tlsConfig.Name)
	// state_certificate is Computed; only the import (2.0.0+) path
	// populates it. On the self-signed path we don't round-trip the
	// device's view here — set concrete null so Terraform accepts state.
	if data.StateCertificate.IsUnknown() {
		data.StateCertificate = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *PartitionCertKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *PartitionCertKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// On F5OS 2.0.0+ the tls container exposes config.certificate and
	// state.certificate leaves. Refresh state_certificate (read-only)
	// unconditionally so drift on the device-reported certificate
	// surfaces on the next plan.
	//
	// We intentionally do NOT populate data.Certificate from
	// cfg.Certificate. On the self-signed workflow the user never
	// sets it in config; if Read wrote a device-supplied value into
	// state, subsequent applies would flip between null (plan) and
	// the device value (state), producing a perpetual diff. Users who
	// need the device view of the certificate should read
	// state_certificate instead.
	if platformVersionAtLeast(r.client.PlatformVersion, "v2.0") {
		_, state, err := r.client.GetTlsCertKey()
		if err != nil {
			resp.Diagnostics.AddWarning("Failed to refresh TLS cert/key from device",
				"The resource will remain in state and Terraform will retry on the next apply. Original error: "+err.Error())
		} else {
			if state.Certificate != "" {
				data.StateCertificate = types.StringValue(state.Certificate)
			} else {
				data.StateCertificate = types.StringNull()
			}
		}
	} else {
		// Pre-2.0.0 devices do not expose the container fields. Clear
		// state_certificate unconditionally so a previously populated
		// value from a 2.0.0+ device does not persist after a
		// downgrade / device swap and mask real drift. Leaving stale
		// content here can also confuse support/debug escalations by
		// implying the device still exposes the state leaf.
		data.StateCertificate = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PartitionCertKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *PartitionCertKeyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if isImportMode(data) {
		r.applyImport(ctx, data, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
		return
	}

	if platformVersionAtLeast(r.client.PlatformVersion, "v1.8") {
		if data.SubjectAlternativeName.IsNull() || data.SubjectAlternativeName.IsUnknown() {
			resp.Diagnostics.AddError("subject_alternative_name is required for platform version v1.8 and above", "")
			return
		}
	} else {
		if !data.SubjectAlternativeName.IsNull() || data.SubjectAlternativeName.IsUnknown() {
			resp.Diagnostics.AddError("subject_alternative_name is not supported for platform version below v1.8", "")
			return
		}
	}

	tlsConfig := getTLSConfig(data)

	err := r.client.CreateTlsCertKey(tlsConfig)

	if err != nil {
		resp.Diagnostics.AddError("Failed to update partition cert key", err.Error())
		return
	}

	// Updating the cert restarts the F5OS HTTPS service. Wait briefly so
	// subsequent API calls find the service available.
	if err := waitForTLSService(ctx, r.client); err != nil {
		resp.Diagnostics.AddWarning("RESTCONF service may still be restarting", err.Error())
	}

	data.Id = types.StringValue(tlsConfig.Name)
	if data.StateCertificate.IsUnknown() {
		data.StateCertificate = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *PartitionCertKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *PartitionCertKeyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteTlsCertKey(data.Id.ValueString())

	if err != nil {
		resp.Diagnostics.AddError("Failed to delete partition cert key", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

// ImportState wires `terraform import f5os_tls_cert_key.<label> <name>` so
// operators can adopt an existing device-side certificate into Terraform
// state. Only the name is passed through; Read fills in the rest from
// the device.
func (r *PartitionCertKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// applyImport handles the F5OS 2.0.0+ import workflow: version-gate,
// PATCH config.certificate and/or config.key, wait for the RESTCONF
// service to recover, and populate the resource's Computed leaves
// (Id, StateCertificate).
func (r *PartitionCertKeyResource) applyImport(ctx context.Context, data *PartitionCertKeyResourceModel, diags *diag.Diagnostics) {
	if !platformVersionAtLeast(r.client.PlatformVersion, "v2.0") {
		diags.AddError("Unsupported attribute",
			"The certificate and key attributes (TLS import workflow) require "+
				"F5OS 2.0.0 or later. Detected device version: "+r.client.PlatformVersion+". "+
				"The self-signed workflow (subject_alternative_name / key_type / key_size / "+
				"key_curve / days_valid / ...) remains available on older versions — remove the "+
				"certificate and key attributes to fall back to it, or target a 2.0.0+ device to "+
				"import an existing cert/key pair.")
		return
	}
	cert := ""
	if !data.Certificate.IsNull() && !data.Certificate.IsUnknown() {
		cert = data.Certificate.ValueString()
	}
	key := ""
	if !data.Key.IsNull() && !data.Key.IsUnknown() {
		key = data.Key.ValueString()
	}
	if err := r.client.ImportTlsCertKey(cert, key); err != nil {
		diags.AddError("Failed to import TLS cert/key", err.Error())
		return
	}

	// Importing a cert/key restarts the F5OS HTTPS service, same as
	// the create-self-signed-cert path.
	if err := waitForTLSService(ctx, r.client); err != nil {
		diags.AddWarning("RESTCONF service may still be restarting", err.Error())
	}

	data.Id = types.StringValue(data.Name.ValueString())
	// Refresh state.certificate from the device so Terraform sees a
	// concrete value; fall back to null if the read fails.
	_, state, err := r.client.GetTlsCertKey()
	if err != nil {
		diags.AddWarning("Failed to refresh TLS cert/key state after import",
			"Terraform will retry on the next apply. Original error: "+err.Error())
		data.StateCertificate = types.StringNull()
		return
	}
	if state.Certificate != "" {
		data.StateCertificate = types.StringValue(state.Certificate)
	} else {
		data.StateCertificate = types.StringNull()
	}
}

func getTLSConfig(data *PartitionCertKeyResourceModel) *f5ossdk.TlsCertKey {

	certKeyConfig := &f5ossdk.TlsCertKey{
		Name: data.Name.ValueString(),
		// SubjectAlternativeName: data.SubjectAlternativeName.ValueString(),
		DaysValid:            data.DaysValid.ValueInt64(),
		Email:                data.Email.ValueString(),
		City:                 data.City.ValueString(),
		Province:             data.Province.ValueString(),
		Country:              data.Country.ValueString(),
		Organization:         data.Organization.ValueString(),
		Unit:                 data.Unit.ValueString(),
		Version:              data.Version.ValueInt64(),
		KeyType:              data.KeyType.ValueString(),
		KeySize:              data.KeySize.ValueInt64(),
		KeyCurve:             data.KeyCurve.ValueString(),
		KeyPassphrase:        data.KeyPassphrase.ValueString(),
		ConfirmKeyPassphrase: data.ConfirmKeyPassphrase.ValueString(),
		StoreTls:             true,
	}

	if !data.SubjectAlternativeName.IsNull() && !data.SubjectAlternativeName.IsUnknown() {
		certKeyConfig.SubjectAlternativeName = data.SubjectAlternativeName.ValueString()
	}

	return certKeyConfig
}

// waitForTLSService waits for the F5OS RESTCONF API to become available after a
// TLS cert/key operation. Creating or updating a self-signed certificate
// restarts the HTTPS service; this helper sleeps for an initial grace period,
// then polls until the service responds with valid data or a timeout is reached.
//
// The wait is skipped when the client host is a loopback address (unit tests
// with httptest.Server) to avoid adding unnecessary latency.
func waitForTLSService(ctx context.Context, client *f5ossdk.F5os) error {
	// Skip the wait for unit tests targeting localhost/127.0.0.1.
	if strings.Contains(client.Host, "127.0.0.1") || strings.Contains(client.Host, "localhost") {
		return nil
	}

	// Initial grace period to let the HTTPS service fully restart. The
	// service typically takes 5-15s to come back; without this sleep the
	// first poll might hit a partially-restarted service (TLS handshake
	// failure) and confuse the retry logic.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(15 * time.Second):
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		resp, err := client.GetRequest("/openconfig-platform:components/component")
		if err == nil && len(resp) > 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("RESTCONF API did not become available within 45 seconds after TLS cert/key operation")
}
