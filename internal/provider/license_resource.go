package provider

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

var _ resource.Resource = &LicenseResource{}

func NewLicenseResource() resource.Resource {
	return &LicenseResource{}
}

type LicenseResource struct {
	client *f5ossdk.F5os
}

type LicenseResourceModel struct {
	RegistrationKey types.String `tfsdk:"registration_key"`
	AddonKeys       types.List   `tfsdk:"addon_keys"`
	LicenseServer   types.String `tfsdk:"license_server"`
	Id              types.String `tfsdk:"id"`
}

func (r *LicenseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_license"
}

func (r *LicenseResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Resource to manage license activation and deactivation.",

		Attributes: map[string]schema.Attribute{
			"registration_key": schema.StringAttribute{
				MarkdownDescription: "The Base registration key from a license server for the device license activation.",
				Required:            true,
				Sensitive:           true,
			},
			"addon_keys": schema.ListAttribute{
				MarkdownDescription: "The additional registration keys from a license server for the device license activation.",
				Required:            false,
				ElementType:         types.StringType,
				Sensitive:           true,
				Optional:            true,
			},
			"license_server": schema.StringAttribute{
				MarkdownDescription: "The license server url.",
				Required:            false,
				Optional:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the license.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *LicenseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (r *LicenseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *LicenseResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	var addonKeys []string

	if !data.AddonKeys.IsNull() || !data.AddonKeys.IsUnknown() {
		data.AddonKeys.ElementsAs(ctx, &addonKeys, false)
	}

	regKey := data.RegistrationKey.ValueString()

	err := r.client.Eula(regKey, addonKeys)

	if err != nil {
		resp.Diagnostics.AddError("Error during EULA", err.Error())
		return
	}

	err = r.client.LicenseInstall(regKey, addonKeys)
	if err != nil {
		resp.Diagnostics.AddError("Error during License Install", err.Error())
		return
	}

	id := fmt.Sprintf("license_res_id_%d", time.Now().UnixMilli())
	data.Id = types.StringValue(id)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LicenseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *LicenseResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	license, err := r.client.GetLicense()
	if err != nil {
		resp.Diagnostics.AddError("Error during Get License", err.Error())
		return
	}

	regKey := license.Licensing.State.RegKey.Base
	data.RegistrationKey = types.StringValue(regKey)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LicenseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *LicenseResourceModel
	var state *LicenseResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	var planAddonKeys []string
	var stateAddonKeys []string

	if !plan.AddonKeys.IsNull() || !plan.AddonKeys.IsUnknown() {
		plan.AddonKeys.ElementsAs(ctx, &planAddonKeys, false)
	}
	if !state.AddonKeys.IsNull() || !state.AddonKeys.IsUnknown() {
		state.AddonKeys.ElementsAs(ctx, &stateAddonKeys, false)
	}

	slices.Sort(planAddonKeys)
	slices.Sort(stateAddonKeys)

	var updateAddonKeys []string
	var regKey string

	if !slices.Equal(planAddonKeys, stateAddonKeys) {
		updateAddonKeys = planAddonKeys
	}

	if plan.RegistrationKey.ValueString() != state.RegistrationKey.ValueString() {
		regKey = plan.RegistrationKey.ValueString()
		updateAddonKeys = planAddonKeys
	}

	err := r.client.Eula(regKey, updateAddonKeys)
	if err != nil {
		resp.Diagnostics.AddError("Error during EULA", err.Error())
		return
	}

	err = r.client.LicenseInstall(regKey, updateAddonKeys)
	if err != nil {
		resp.Diagnostics.AddError("Error during License Install", err.Error())
		return
	}

	plan.Id = state.Id
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LicenseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No-op
}

func (r *LicenseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
