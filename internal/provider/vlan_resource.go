package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &VlanResource{}
var _ resource.ResourceWithImportState = &VlanResource{}

func NewVlanResource() resource.Resource {
	return &VlanResource{}
}

// VlanResource defines the resource implementation.
type VlanResource struct {
	client   *f5ossdk.F5os
	teemData *TeemData
}

type VlanResourceModel struct {
	Name   types.String `tfsdk:"name"`
	VlanId types.Int64  `tfsdk:"vlan_id"`
	Id     types.String `tfsdk:"id"`
}

func (r *VlanResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vlan"
}

func (r *VlanResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Resource to Manage VLANs on F5OS based systems like chassis partitions or rSeries platforms",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Specifies the name of the VLAN to configure on the F5OS platform.\nThis parameter is required when creating a resource.\nThe first character must be a letter, alphanumeric characters are allowed.\nPeriods, commas, hyphens, and underscores are allowed.\nThe name cannot exceed 58 characters.",
				Optional:            true,
			},
			"vlan_id": schema.Int64Attribute{
				MarkdownDescription: "The ID for the VLAN.\nValid value range is from `0` to `4095`.",
				Required:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for Vlan resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *VlanResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
	//teemData := &TeemData{}
	teemData.ProviderName = "f5os"
	teemData.ResourceName = "f5os_vlan"
	r.teemData = teemData
}

func (r *VlanResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *VlanResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	if r.client.PlatformType == "Velos Controller" {
		resp.Diagnostics.AddError("Client Error", "`f5os_vlan` resource is supported with Velos Partition level/rSeries appliance.")
		return
	}
	tflog.Info(ctx, fmt.Sprintf("[CREATE] Vlan ID:%+v", data.VlanId.ValueInt64()))
	vlanReqConfig := getPartitionVlanConfig(data)

	tflog.Debug(ctx, fmt.Sprintf("vlanReqConfig Data:%+v", vlanReqConfig))

	teemInfo := make(map[string]any)
	teemInfo["teemData"] = r.teemData
	err := r.client.SendTeem(teemInfo)
	if err != nil {
		resp.Diagnostics.AddError("Teem Error", fmt.Sprintf("Sending Teem Data failed: %s", err))
	}
	respByte, err := r.client.VlanConfig(vlanReqConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Create Vlan failed, got error: %s", err))
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("vlanReqConfig Response:%+v", string(respByte)))
	data.Id = types.StringValue(fmt.Sprintf("%d", int(data.VlanId.ValueInt64())))

	partData, err := r.client.GetVlan(int(data.VlanId.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read/Get Vlan, got error: %s", err))
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("VlanResp :%+v", partData))
	r.vlanResourceModelToState(ctx, partData, data)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

func (r *VlanResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *VlanResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, fmt.Sprintf("[READ] Vlan :%+v", data.Id.ValueString()))
	vlanId, err := strconv.Atoi(data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("[F5OS] %v", err), fmt.Sprintf("String to Int conversion failed for ID:%s", data.Id.ValueString()))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("[READ] Vlan :%+v", vlanId))
	partData, err := r.client.GetVlan(vlanId)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("%v", err), fmt.Sprintf("Unable to Read/Get Vlan ID:%d", vlanId))
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("VlanResp :%+v", partData))
	r.vlanResourceModelToState(ctx, partData, data)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VlanResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *VlanResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client.PlatformType == "Velos Controller" {
		resp.Diagnostics.AddError("Client Error", "`f5os_vlan` resource is supported with Velos Partition level.")
		return
	}
	tflog.Info(ctx, fmt.Sprintf("[UPDATE] Vlan ID:%+v", data.VlanId.ValueInt64()))
	vlanReqConfig := getPartitionVlanConfig(data)
	tflog.Info(ctx, fmt.Sprintf("vlanReqConfig Data:%+v", vlanReqConfig))

	respByte, err := r.client.VlanConfig(vlanReqConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Update Vlan failed, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("vlanReqConfig Response:%+v", string(respByte)))

	data.Id = types.StringValue(fmt.Sprintf("%d", int(data.VlanId.ValueInt64())))
	partData, err := r.client.GetVlan(int(data.VlanId.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read/Get Vlan, got error: %s", err))
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("VlanResp :%+v", partData))
	r.vlanResourceModelToState(ctx, partData, data)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VlanResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *VlanResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteVlan(int(data.VlanId.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to Delete Vlan, got error: %s", err))
		return
	}
}

func (r *VlanResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *VlanResource) vlanResourceModelToState(ctx context.Context, respData *f5ossdk.F5RespVlan, data *VlanResourceModel) {
	data.Name = types.StringValue(respData.OpenconfigVlanVlan[0].Config.Name)
	data.VlanId = types.Int64Value(int64(respData.OpenconfigVlanVlan[0].Config.VlanID))
}

func getPartitionVlanConfig(data *VlanResourceModel) *f5ossdk.F5ReqVlansConfig {
	partitionVlanReq := f5ossdk.F5ReqVlanConfig{}
	partitionVlanReq.Config.Name = data.Name.ValueString()
	partitionVlanReq.Config.VlanId = int(data.VlanId.ValueInt64())
	partitionVlanReq.VlanId = fmt.Sprintf("%d", int(data.VlanId.ValueInt64()))
	vlanReqConfig := f5ossdk.F5ReqVlansConfig{}
	vlanReqConfig.OpenconfigVlanVlans.Vlan = append(vlanReqConfig.OpenconfigVlanVlans.Vlan, partitionVlanReq)
	return &vlanReqConfig
}
