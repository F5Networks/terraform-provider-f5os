package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	f5os "gitswarm.f5net.com/terraform-providers/f5osclient"
)

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
			},
			"ntp_authentication": schema.BoolAttribute{
				MarkdownDescription: "Enable or disable NTP authentication.",
				Optional:            true,
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
	tflog.Info(ctx, "MDEBUG: Creating NTP Server", map[string]any{
		"server": plan.Server.ValueString(),
	})

	// print all the plan fields here
	tflog.Debug(ctx, "MDEBUG: Create Plan", map[string]any{
		"server":             plan.Server.ValueString(),
		"key_id":             plan.KeyID.ValueInt64(),
		"prefer":             plan.Prefer.ValueBool(),
		"iburst":             plan.IBurst.ValueBool(),
		"ntp_service":        plan.NTPService.ValueBool(),
		"ntp_authentication": plan.NTPAuthentication.ValueBool(),
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

	state.ID = types.StringValue(state.Server.ValueString())
	state.Server = types.StringValue(state.Server.ValueString())
	// state.Server = types.StringValue(ntp.Address)
	state.KeyID = types.Int64Value(int64(ntp.KeyID))
	state.Prefer = types.BoolValue(ntp.Prefer)
	state.IBurst = types.BoolValue(ntp.IBurst)
	state.NTPService = types.BoolValue(ntp.NTPService)
	state.NTPAuthentication = types.BoolValue(ntp.NTPAuthentication)

	tflog.Debug(ctx, "MDEBUG: Current Read Result", map[string]any{
		"server": ntp.Address,
		"key_id": ntp.KeyID,
		"id":     state.ID,
	})
	if state.ID.IsNull() || state.ID.IsUnknown() {
		tflog.Error(ctx, "MDEBUG: ID is missing after read", nil)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *NTPServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan f5os.NTPServerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, err := r.client.CreateNTPServerPayload(plan.Server.ValueString(), plan) // Reusing CreateNTPServerPayload
	if err != nil {
		resp.Diagnostics.AddError("Payload Creation Error", err.Error())
		return
	}

	if err := r.client.UpdateNTPServer(plan.Server.ValueString(), payload); err != nil {
		resp.Diagnostics.AddError("NTP Update Error", err.Error())
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

type NTPServerModel struct {
	ID                types.String `tfsdk:"id"`
	Server            types.String `tfsdk:"server"`
	KeyID             types.Int64  `tfsdk:"key_id"`
	Prefer            types.Bool   `tfsdk:"prefer"`
	IBurst            types.Bool   `tfsdk:"iburst"`
	NTPService        types.Bool   `tfsdk:"ntp_service"`
	NTPAuthentication types.Bool   `tfsdk:"ntp_authentication"`
}

// type NTPServerModel struct {
// 	Server            string `tfsdk:"server"`
// 	KeyID             int    `tfsdk:"key_id"`
// 	Prefer            bool   `tfsdk:"prefer"`
// 	IBurst            bool   `tfsdk:"iburst"`
// 	NTPService        bool   `tfsdk:"ntp_service"`
// 	NTPAuthentication bool   `tfsdk:"ntp_authentication"`
// }
