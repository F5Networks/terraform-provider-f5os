package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	f5os "gitswarm.f5net.com/terraform-providers/f5osclient"
)

var _ resource.ResourceWithImportState = &NTPServerResource{}

type NTPServerResource struct {
	client *f5os.F5os
}

func NewNTPServerResource() resource.Resource {
	return &NTPServerResource{}
}

func (r *NTPServerResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "f5os_ntp_server"
}

func (r *NTPServerResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*f5os.F5os)
	}
}

func (r *NTPServerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage NTP servers on F5OS based systems (Velos controller or rSeries appliance).",
		Attributes: map[string]schema.Attribute{
			"server": schema.StringAttribute{
				MarkdownDescription: "IPv4/IPv6 address or FQDN of the NTP server.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key_id": schema.Int64Attribute{
				MarkdownDescription: "Key ID used for authentication with the NTP server. This should be configured with a key ID that has been already created on the system.",
				Optional:            true,
			},
			"prefer": schema.BoolAttribute{
				MarkdownDescription: "Set to true if this is the preferred server.",
				Optional:            true,
			},
			"iburst": schema.BoolAttribute{
				MarkdownDescription: "Enable iburst for faster synchronization.",
				Optional:            true,
			},
			"ntp_service": schema.BoolAttribute{
				MarkdownDescription: "Enable or disable the NTP service.",
				Optional:            true,
				Computed:            true,
			},
			"ntp_authentication": schema.BoolAttribute{
				MarkdownDescription: "Enable or disable NTP authentication.",
				Optional:            true,
				Computed:            true,
			},
			// F5OS 2.0.0+ additive config leaves. Writing these on an
			// older device (< 2.0.0) returns a clear error from
			// Create/Update; on 2.0.0+ they are passed through to the
			// device.
			"association_type": schema.StringAttribute{
				MarkdownDescription: "NTP association type. Requires F5OS 2.0.0 or later. Typical values are `SERVER`, `PEER`, or `POOL`; the device enforces the allowed set.",
				Optional:            true,
			},
			"version": schema.Int64Attribute{
				MarkdownDescription: "NTP protocol version to use with this server. Requires F5OS 2.0.0 or later.",
				Optional:            true,
			},
			"port": schema.Int64Attribute{
				MarkdownDescription: "UDP port to reach the NTP server on. Requires F5OS 2.0.0 or later.",
				Optional:            true,
			},
			// F5OS 2.0.0+ read-only state leaves. Populated by Read from
			// the device's state container. Null on pre-2.0.0 devices.
			"stratum": schema.Int64Attribute{
				MarkdownDescription: "Reported stratum of the NTP server (read-only, F5OS 2.0.0+).",
				Computed:            true,
			},
			"authenticated": schema.BoolAttribute{
				MarkdownDescription: "Whether the association is authenticated (read-only, F5OS 2.0.0+).",
				Computed:            true,
			},
			"state_address": schema.StringAttribute{
				MarkdownDescription: "Resolved address for the NTP server as reported by the device (read-only, F5OS 2.0.0+).",
				Computed:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform synthetic ID (server address).",
			},
		},
	}
}

func (r *NTPServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan f5os.NTPServerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if diags := r.validate200Fields(plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	payload, err := r.client.CreateNTPServerPayload(plan.Server.ValueString(), plan)
	if err != nil {
		resp.Diagnostics.AddError("Payload Creation Error", err.Error())
		return
	}

	if err = r.client.CreateNTPServer(plan.Server.ValueString(), payload); err != nil {
		resp.Diagnostics.AddError("NTP Create Error", err.Error())
		return
	}

	// Patch global NTP config (service enable / authentication enable)
	// when either attribute is explicitly set in the plan (not null and not
	// unknown).  Unknown means the user omitted the attribute and Terraform
	// is letting the provider compute it.
	if !plan.NTPService.IsNull() && !plan.NTPService.IsUnknown() || !plan.NTPAuthentication.IsNull() && !plan.NTPAuthentication.IsUnknown() {
		var svc, auth *bool
		if !plan.NTPService.IsNull() && !plan.NTPService.IsUnknown() {
			v := plan.NTPService.ValueBool()
			svc = &v
		}
		if !plan.NTPAuthentication.IsNull() && !plan.NTPAuthentication.IsUnknown() {
			v := plan.NTPAuthentication.ValueBool()
			auth = &v
		}
		if err := r.client.PatchNTPGlobalConfig(svc, auth); err != nil {
			resp.Diagnostics.AddError("NTP Global Config Error", err.Error())
			return
		}
	}

	// When ntp_service / ntp_authentication are omitted from the config they
	// arrive as Unknown (Computed).  Resolve them from the device so the
	// state always contains concrete values after apply.
	if plan.NTPService.IsUnknown() || plan.NTPAuthentication.IsUnknown() {
		svc, auth, err := r.client.GetNTPGlobalConfig()
		if err != nil {
			resp.Diagnostics.AddError("NTP Global Config Read Error", err.Error())
			return
		}
		if plan.NTPService.IsUnknown() {
			plan.NTPService = types.BoolValue(svc)
		}
		if plan.NTPAuthentication.IsUnknown() {
			plan.NTPAuthentication = types.BoolValue(auth)
		}
	}

	tflog.Info(ctx, "Creating NTP Server", map[string]any{
		"server": plan.Server.ValueString(),
	})

	// Populate F5OS 2.0.0+ read-only state leaves so Terraform's Computed
	// attributes have concrete values after Create. On pre-2.0.0 devices
	// these end up as null, which is a legal value for a Computed
	// attribute. Failures here are warnings (see helper) so a transient
	// post-write GET error does not leak the resource on the device.
	resp.Diagnostics.Append(r.refreshComputedStateLeaves(&plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = plan.Server
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *NTPServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state NTPServerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// write the debug print for state variable
	tflog.Debug(ctx, "MDEBUG: Current State", map[string]any{
		"server":             state.Server.ValueString(),
		"key_id":             state.KeyID.ValueInt64(),
		"prefer":             state.Prefer.ValueBool(),
		"iburst":             state.IBurst.ValueBool(),
		"ntp_service":        state.NTPService.ValueBool(),
		"ntp_authentication": state.NTPAuthentication.ValueBool(),
	})

	ntp, err := r.client.GetNTPServer(state.Server.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("NTP Read Error", err.Error())
		return
	}

	ntpService, ntpAuth, err := r.client.GetNTPGlobalConfig()
	if err != nil {
		resp.Diagnostics.AddError("NTP Global Config Read Error", err.Error())
		return
	}

	state.ID = types.StringValue(state.Server.ValueString())
	state.Server = types.StringValue(ntp.Address)
	if ntp.KeyID != nil {
		state.KeyID = types.Int64Value(*ntp.KeyID)
	} else {
		state.KeyID = types.Int64Null()
	}
	state.Prefer = types.BoolValue(ntp.Prefer)
	state.IBurst = types.BoolValue(ntp.IBurst)
	state.NTPService = types.BoolValue(ntpService)
	state.NTPAuthentication = types.BoolValue(ntpAuth)

	// F5OS 2.0.0+ additive config leaves. Mirror the device
	// unconditionally so out-of-band removal (downgrade, admin edit)
	// surfaces as drift rather than sticking to a stale plan value.
	// On pre-2.0.0 devices the leaves are always absent, so these
	// stay null.
	state.AssociationType = nullableStringToTF(ntp.AssociationType)
	state.Version = nullableInt64ToTF(ntp.Version)
	state.Port = nullableInt64ToTF(ntp.Port)

	// F5OS 2.0.0+ read-only state leaves. Devices below 2.0.0 do not
	// populate these, in which case they stay null.
	state.Stratum = nullableInt64ToTF(ntp.StateStratum)
	state.Authenticated = nullableBoolToTF(ntp.StateAuthenticated)
	state.StateAddress = nullableStringToTF(ntp.StateAddress)

	tflog.Debug(ctx, "NTP Read Result", map[string]any{
		"server":             ntp.Address,
		"key_id":             ntp.KeyID,
		"prefer":             ntp.Prefer,
		"iburst":             ntp.IBurst,
		"ntp_service":        ntpService,
		"ntp_authentication": ntpAuth,
		"id":                 state.ID.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *NTPServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan f5os.NTPServerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if diags := r.validate200Fields(plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	payload, err := r.client.UpdateNTPServerPayload(plan.Server.ValueString(), plan)
	if err != nil {
		resp.Diagnostics.AddError("Payload Creation Error", err.Error())
		return
	}

	if err := r.client.UpdateNTPServer(plan.Server.ValueString(), payload); err != nil {
		resp.Diagnostics.AddError("NTP Update Error", err.Error())
		return
	}

	// Patch global NTP config (service enable / authentication enable)
	// when either attribute is explicitly set in the plan.
	if !plan.NTPService.IsNull() && !plan.NTPService.IsUnknown() || !plan.NTPAuthentication.IsNull() && !plan.NTPAuthentication.IsUnknown() {
		var svc, auth *bool
		if !plan.NTPService.IsNull() && !plan.NTPService.IsUnknown() {
			v := plan.NTPService.ValueBool()
			svc = &v
		}
		if !plan.NTPAuthentication.IsNull() && !plan.NTPAuthentication.IsUnknown() {
			v := plan.NTPAuthentication.ValueBool()
			auth = &v
		}
		if err := r.client.PatchNTPGlobalConfig(svc, auth); err != nil {
			resp.Diagnostics.AddError("NTP Global Config Update Error", err.Error())
			return
		}
	}

	// Resolve unknown computed values from the device.
	if plan.NTPService.IsUnknown() || plan.NTPAuthentication.IsUnknown() {
		svc, auth, err := r.client.GetNTPGlobalConfig()
		if err != nil {
			resp.Diagnostics.AddError("NTP Global Config Read Error", err.Error())
			return
		}
		if plan.NTPService.IsUnknown() {
			plan.NTPService = types.BoolValue(svc)
		}
		if plan.NTPAuthentication.IsUnknown() {
			plan.NTPAuthentication = types.BoolValue(auth)
		}
	}

	// Populate F5OS 2.0.0+ read-only state leaves so Computed attributes
	// carry concrete post-apply values. Failures here are warnings.
	resp.Diagnostics.Append(r.refreshComputedStateLeaves(&plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = plan.Server
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NTPServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state NTPServerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteNTPServer(state.Server.ValueString()); err != nil {
		resp.Diagnostics.AddError("NTP Delete Error", err.Error())
		return
	}
}

func (r *NTPServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("server"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	tflog.Info(ctx, "Importing NTP Server", map[string]any{"server": req.ID})
}

// validate200Fields returns an error diagnostic when any of the F5OS
// 2.0.0+ additive attributes (association_type, version, port) are set
// on a device running an older version. It is called by Create and
// Update before any payload is built.
func (r *NTPServerResource) validate200Fields(plan f5os.NTPServerModel) diag.Diagnostics {
	var diags diag.Diagnostics
	var set []string
	if !plan.AssociationType.IsNull() && !plan.AssociationType.IsUnknown() {
		set = append(set, "association_type")
	}
	if !plan.Version.IsNull() && !plan.Version.IsUnknown() {
		set = append(set, "version")
	}
	if !plan.Port.IsNull() && !plan.Port.IsUnknown() {
		set = append(set, "port")
	}
	if len(set) == 0 {
		return diags
	}
	if !platformVersionAtLeast(r.client.PlatformVersion, "v2.0") {
		diags.AddError("Unsupported attribute",
			fmt.Sprintf("The following NTP server attribute(s) are only "+
				"supported on F5OS 2.0.0 or later: %s. Detected device "+
				"version: %q. Remove these attributes or target a 2.0.0+ device.",
				strings.Join(set, ", "), r.client.PlatformVersion))
	}
	return diags
}

// refreshComputedStateLeaves fetches the current NTP server entry from
// the device and copies the read-only 2.0.0+ state leaves plus any
// config leaves the device chose to echo back onto plan. This gives
// Terraform's Computed attributes concrete values after Create/Update.
// On pre-2.0.0 devices the state leaves come back nil, in which case
// the corresponding plan fields end up null — the legal empty value
// for a Computed attribute.
//
// Failures here are downgraded to warnings: the device write already
// succeeded, so aborting Create/Update with an error would leak the
// resource on the device while Terraform believes nothing was
// created, forcing a subsequent apply to fail with "already exists".
// The next Read cycle fills the Computed leaves.
func (r *NTPServerResource) refreshComputedStateLeaves(plan *f5os.NTPServerModel) diag.Diagnostics {
	var diags diag.Diagnostics
	ntp, err := r.client.GetNTPServer(plan.Server.ValueString())
	if err != nil {
		diags.AddWarning("NTP Post-Write Read Warning",
			"The NTP server was written to the device successfully, but the "+
				"follow-up read used to populate computed state attributes failed: "+
				err.Error()+". Terraform will retry the read on the next apply.")
		// Set Computed leaves to null so Terraform accepts state as
		// consistent (Computed attributes may not remain Unknown in
		// final state). The next Read will overwrite with real values.
		plan.Stratum = types.Int64Null()
		plan.Authenticated = types.BoolNull()
		plan.StateAddress = types.StringNull()
		return diags
	}
	// Config leaves: only overwrite if the device returned a value.
	// This preserves plan-declared values on devices that echo config
	// verbatim, but also picks up device-side defaults when the caller
	// omitted the leaf.
	if ntp.AssociationType != nil {
		plan.AssociationType = types.StringValue(*ntp.AssociationType)
	}
	if ntp.Version != nil {
		plan.Version = types.Int64Value(*ntp.Version)
	}
	if ntp.Port != nil {
		plan.Port = types.Int64Value(*ntp.Port)
	}
	// Read-only state leaves. nullableXToTF maps nil to *Null().
	plan.Stratum = nullableInt64ToTF(ntp.StateStratum)
	plan.Authenticated = nullableBoolToTF(ntp.StateAuthenticated)
	plan.StateAddress = nullableStringToTF(ntp.StateAddress)
	return diags
}

type NTPServerModel struct {
	ID                types.String `tfsdk:"id"`
	Server            types.String `tfsdk:"server"`
	KeyID             types.Int64  `tfsdk:"key_id"`
	Prefer            types.Bool   `tfsdk:"prefer"`
	IBurst            types.Bool   `tfsdk:"iburst"`
	NTPService        types.Bool   `tfsdk:"ntp_service"`
	NTPAuthentication types.Bool   `tfsdk:"ntp_authentication"`
	// F5OS 2.0.0+ additive config leaves.
	AssociationType types.String `tfsdk:"association_type"`
	Version         types.Int64  `tfsdk:"version"`
	Port            types.Int64  `tfsdk:"port"`
	// F5OS 2.0.0+ additive read-only state leaves.
	Stratum       types.Int64  `tfsdk:"stratum"`
	Authenticated types.Bool   `tfsdk:"authenticated"`
	StateAddress  types.String `tfsdk:"state_address"`
}

// nullableInt64ToTF converts an optional int64 (nil = leaf absent on
// device) into a Terraform Int64 value, mapping nil to Int64Null().
func nullableInt64ToTF(v *int64) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*v)
}

// nullableBoolToTF converts an optional bool into a Terraform Bool
// value, mapping nil to BoolNull().
func nullableBoolToTF(v *bool) types.Bool {
	if v == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*v)
}

// nullableStringToTF converts an optional string pointer into a
// Terraform String value, mapping nil to StringNull().
func nullableStringToTF(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}
