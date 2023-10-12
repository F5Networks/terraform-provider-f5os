package provider

import (
	"context"
	"fmt"
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
var _ resource.Resource = &LagResource{}
var _ resource.ResourceWithImportState = &LagResource{}

func NewLagResource() resource.Resource {
	return &LagResource{}
}

// LagResource defines the resource implementation.
type LagResource struct {
	client *f5ossdk.F5os
}

type LagResourceModel struct {
	Name       types.String `tfsdk:"name"`
	NativeVlan types.Int64  `tfsdk:"native_vlan"`
	TrunkVlans types.List   `tfsdk:"trunk_vlans"`
	Status     types.String `tfsdk:"status"`
	Members    types.List   `tfsdk:"members"`
	Id         types.String `tfsdk:"id"`
}

func (r *LagResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lag"
}

func (r *LagResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Resource to Manage network Link Aggregation Group (LAG) interfaces on F5OS systems like VELOS chassis partitions or rSeries platforms",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the Link Aggregation Group interface (LAG) interface to configure",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"native_vlan": schema.Int64Attribute{
				MarkdownDescription: "Configures the VLAN ID to associate with LAG interface.\nThe `native_vlan` parameter is used for untagged traffic.",
				Optional:            true,
			},
			"trunk_vlans": schema.ListAttribute{
				MarkdownDescription: "Configures multiple VLAN IDs to associate with the LAG interface.\nThe `trunk_vlans` parameter is used for tagged traffic",
				Optional:            true,
				ElementType:         types.Int64Type,
			},
			"members": schema.ListAttribute{
				MarkdownDescription: "List of physical interfaces that are members of the LAG.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Operational state of the LAG interface.",
				Computed:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for LAG Interface resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *LagResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (r *LagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *LagResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	if r.client.PlatformType == "Velos Controller" {
		resp.Diagnostics.AddError("Client Error", "`f5os_lag` resource is supported with Velos Partition level/rSeries appliance.")
		return
	}
	tflog.Info(ctx, fmt.Sprintf("[CREATE] Config LAG Interface :%+v", data.Name.ValueString()))
	interfaceReqConfig := getLagInterfaceConfig(ctx, data)

	tflog.Debug(ctx, fmt.Sprintf("lagInterfaceReqConfig Data:%+v", interfaceReqConfig))

	membersConfig := getLagMembersConfig(ctx, data)

	respByte, err := r.client.CreateLagInterface(interfaceReqConfig, membersConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Creating LAG interface failed, got error: %s", err))
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("lagInterfaceReqConfig Response:%+v", string(respByte)))
	data.Id = types.StringValue(data.Name.ValueString())

	intfData, err := r.client.GetLagInterface(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read/Get LAG Interface, got error: %s", err))
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("LAG interface Resp :%+v", intfData))
	r.lagInterfaceResourceModelToState(ctx, intfData, data)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

func (r *LagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *LagResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, fmt.Sprintf("[READ] Reading LAG interface :%+v", data.Id.ValueString()))

	intfData, err := r.client.GetLagInterface(data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read/Get LAG interface, got error: %s", err))
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("LAG interface Resp :%+v", intfData))
	r.lagInterfaceResourceModelToState(ctx, intfData, data)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *LagResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client.PlatformType == "Velos Controller" {
		resp.Diagnostics.AddError("Client Error", "`f5os_lag` resource is supported with Velos Partition level/rSeries appliance.")
		return
	}
	tflog.Info(ctx, fmt.Sprintf("[UPDATE] Config LAG interface :%+v", data.Name.ValueString()))
	lagInterfaceReqConfig := getLagInterfaceConfig(ctx, data)
	tflog.Info(ctx, fmt.Sprintf("lagInterfaceReqConfig Data:%+v", lagInterfaceReqConfig))

	if !data.Members.IsNull() && !data.Members.IsUnknown() {
		memberData, err := r.client.GetLagInterface(data.Id.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read LAG interface members got error: %s", err))
			return
		}
		if memberData != nil {
			var haveMembers []string
			for _, member := range memberData.OpenconfigInterfacesInterface[0].OpenconfigIfAggregateAggregation.State.Members.Member {
				haveMembers = append(haveMembers, member.Name)
			}
			err := r.client.RemoveLagMembers(haveMembers)
			if err != nil {
				resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to remove members from LAG interface, got error: %s", err))
				return
			}
			membersConfig := getLagMembersConfig(ctx, data)
			_, err = r.client.UpdateLagMembers(membersConfig)
			if err != nil {
				resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to update LAG interface members, got error: %s", err))
				return
			}
		}
	}

	respByte, err := r.client.UpdateLagInterface(data.Id.ValueString(), lagInterfaceReqConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Update LAG interface failed, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("lagInterfaceReqConfig Response:%+v", string(respByte)))

	data.Id = types.StringValue(data.Name.ValueString())

	intfData, err := r.client.GetLagInterface(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read/Get LAG Interface, got error: %s", err))
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("LAG interface Resp :%+v", intfData))
	r.lagInterfaceResourceModelToState(ctx, intfData, data)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *LagResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	// Check if we have any physical interfaces that are a member of the LAG interface
	memberData, err1 := r.client.GetLagInterface(data.Id.ValueString())
	if err1 != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read LAG interface members got error: %s", err1))
		return
	}
	// Remove any associated interfaces
	if memberData != nil {
		var haveMembers []string
		for _, member := range memberData.OpenconfigInterfacesInterface[0].OpenconfigIfAggregateAggregation.State.Members.Member {
			haveMembers = append(haveMembers, member.Name)
		}
		err := r.client.RemoveLagMembers(haveMembers)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Removing LAG interface member failed, got error: %s", err))
			return
		}
	}

	err2 := r.client.RemoveLagInterface(data.Id.ValueString())
	if err2 != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to delete LAG interface, got error: %s", err2))
		return
	}

}

func (r *LagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *LagResource) lagInterfaceResourceModelToState(ctx context.Context, respData *f5ossdk.F5RespLagInterfaces, data *LagResourceModel) {
	data.Name = types.StringValue(respData.OpenconfigInterfacesInterface[0].Name)
	data.NativeVlan = types.Int64Value(int64(respData.OpenconfigInterfacesInterface[0].OpenconfigIfAggregateAggregation.OpenconfigVlanSwitchedVlan.Config.NativeVlan))
	data.TrunkVlans, _ = types.ListValueFrom(ctx, types.Int64Type, respData.OpenconfigInterfacesInterface[0].OpenconfigIfAggregateAggregation.OpenconfigVlanSwitchedVlan.Config.TrunkVlans)
	data.Status = types.StringValue(respData.OpenconfigInterfacesInterface[0].State.OperStatus)

	var members []string
	for _, member := range respData.OpenconfigInterfacesInterface[0].OpenconfigIfAggregateAggregation.State.Members.Member {
		members = append(members, member.Name)
	}
	data.Members, _ = types.ListValueFrom(ctx, types.StringType, members)
}

func getLagInterfaceConfig(ctx context.Context, data *LagResourceModel) *f5ossdk.F5ReqLagInterfaces {
	interfaceReq := f5ossdk.F5ReqLagInterface{}
	interfaceReq.Name = data.Name.ValueString()
	interfaceReq.Config.Name = data.Name.ValueString()
	interfaceReq.Config.Type = "iana-if-type:ieee8023adLag"
	interfaceReq.Config.Enabled = true
	interfaceReq.OpenconfigIfAggregateAggregation.Config.LagType = "LACP"
	interfaceReq.OpenconfigIfAggregateAggregation.Config.DistributioHash = "src-dst-ipport"
	interfaceReq.OpenconfigIfAggregateAggregation.OpenconfigVlanSwitchedVlan.Config.NativeVlan = int(data.NativeVlan.ValueInt64())
	var trunkIds []int
	data.TrunkVlans.ElementsAs(ctx, &trunkIds, false)
	interfaceReq.OpenconfigIfAggregateAggregation.OpenconfigVlanSwitchedVlan.Config.TrunkVlans = trunkIds
	interfaceOpenconfigReq := f5ossdk.F5ReqLagInterfaces{}
	interfaceOpenconfigReq.OpenconfigInterfacesInterfaces.Interface = append(interfaceOpenconfigReq.OpenconfigInterfacesInterfaces.Interface, interfaceReq)
	return &interfaceOpenconfigReq
}

func getLagMembersConfig(ctx context.Context, data *LagResourceModel) *f5ossdk.F5ReqLagInterfaces {
	memberConfigReq := f5ossdk.F5ReqLagInterfaces{}
	memberReq := f5ossdk.F5ReqLagInterface{}
	var members []string
	data.Members.ElementsAs(ctx, &members, false)
	for _, member := range members {
		memberReq.Name = member
		memberReq.Config.Name = member
		memberReq.OpenconfigIfEthernetEthernet.Config.Name = data.Name.ValueString()
		memberConfigReq.OpenconfigInterfacesInterfaces.Interface = append(memberConfigReq.OpenconfigInterfacesInterfaces.Interface, memberReq)
	}
	return &memberConfigReq
}
