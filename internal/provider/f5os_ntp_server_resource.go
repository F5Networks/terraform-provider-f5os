package provider

import (
	"context"

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

type NTPServerModel struct {
	ID                types.String `tfsdk:"id"`
	Server            types.String `tfsdk:"server"`
	KeyID             types.Int64  `tfsdk:"key_id"`
	Prefer            types.Bool   `tfsdk:"prefer"`
	IBurst            types.Bool   `tfsdk:"iburst"`
	NTPService        types.Bool   `tfsdk:"ntp_service"`
	NTPAuthentication types.Bool   `tfsdk:"ntp_authentication"`
}
