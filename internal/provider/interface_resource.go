package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &InterfaceResource{}
var _ resource.ResourceWithImportState = &InterfaceResource{}

func NewInterfaceResource() resource.Resource {
	return &InterfaceResource{}
}

// InterfaceResource defines the resource implementation.
type InterfaceResource struct {
	client *f5ossdk.F5os
}

type InterfaceResourceModel struct {
	Name       types.String `tfsdk:"name"`
	NativeVlan types.Int64  `tfsdk:"native_vlan"`
	TrunkVlans types.List   `tfsdk:"trunk_vlans"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Status     types.String `tfsdk:"status"`
	Id         types.String `tfsdk:"id"`
}

func (r *InterfaceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_interface"
}

func (r *InterfaceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Resource to Manage network interfaces on F5OS systems like VELOS chassis partitions or rSeries platforms",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the interface to configure.\nFor VELOS partitions blade/port format is required e.g. `1/1.0`",
				Optional:            true,
			},
			"native_vlan": schema.Int64Attribute{
				MarkdownDescription: "Configures the VLAN ID to associate with the interface.\nThe `native_vlan` parameter is used for untagged traffic.",
				Optional:            true,
			},
			"trunk_vlans": schema.ListAttribute{
				MarkdownDescription: "Configures multiple VLAN IDs to associate with the interface.\nThe `trunk_vlans` parameter is used for tagged traffic",
				Optional:            true,
				ElementType:         types.Int64Type,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Enables or disables interface.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Operational state of the interface.",
				Computed:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for Interface resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *InterfaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (r *InterfaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *InterfaceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	if r.client.PlatformType == "Velos Controller" {
		resp.Diagnostics.AddError("Client Error", "`f5os_vlan` resource is supported with Velos Partition level/rSeries appliance.")
		return
	}
	tflog.Info(ctx, fmt.Sprintf("[CREATE] Config Interface :%+v", data.Name.ValueString()))
	interfaceReqConfig := getInterfaceConfig(ctx, data)

	tflog.Debug(ctx, fmt.Sprintf("interfaceReqConfig Data:%+v", interfaceReqConfig))

	respByte, err := r.client.UpdateInterface(data.Name.ValueString(), interfaceReqConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Updating Interface failed, got error: %s", err))
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("interfaceReqConfig Response:%+v", string(respByte)))
	data.Id = types.StringValue(data.Name.ValueString())

	intfData, err := r.client.GetInterface(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read/Get Interface, got error: %s", err))
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("Interface Resp :%+v", intfData))
	r.interfaceResourceModelToState(ctx, intfData, data)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

func (r *InterfaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *InterfaceResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, fmt.Sprintf("[READ] Reading Interface :%+v", data.Id.ValueString()))

	intfData, err := r.client.GetInterface(data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read/Get Interface, got error: %s", err))
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("Interface Resp :%+v", intfData))
	r.interfaceResourceModelToState(ctx, intfData, data)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *InterfaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *InterfaceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client.PlatformType == "Velos Controller" {
		resp.Diagnostics.AddError("Client Error", "`f5os_vlan` resource is supported with Velos Partition level.")
		return
	}
	tflog.Info(ctx, fmt.Sprintf("[UPDATE] Config Interface :%+v", data.Name.ValueString()))
	interfaceReqConfig := getInterfaceConfig(ctx, data)
	tflog.Info(ctx, fmt.Sprintf("interfaceReqConfig Data:%+v", interfaceReqConfig))

	respByte, err := r.client.UpdateInterface(data.Name.ValueString(), interfaceReqConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Update Vlan failed, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("interfaceReqConfig Response:%+v", string(respByte)))
	data.Id = types.StringValue(data.Name.ValueString())
	intfData, err := r.client.GetInterface(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read/Get Interface, got error: %s", err))
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("Interface Resp :%+v", intfData))
	r.interfaceResourceModelToState(ctx, intfData, data)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *InterfaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *InterfaceResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.RemoveNativeVlans(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Removing Native vlan failed, got error: %s", err))
		return
	}
	var trunkIds []int
	data.TrunkVlans.ElementsAs(ctx, &trunkIds, false)
	for _, trunkId := range trunkIds {
		err := r.client.RemoveTrunkVlans(data.Name.ValueString(), trunkId)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Removing Trunk vlan ID failed, got error: %s", err))
			return
		}
	}
}

func (r *InterfaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *InterfaceResource) interfaceResourceModelToState(ctx context.Context, respData *f5ossdk.F5RespOpenconfigInterface, data *InterfaceResourceModel) {
	data.Name = types.StringValue(respData.OpenconfigInterfacesInterface[0].Name)
	data.Enabled = types.BoolValue(respData.OpenconfigInterfacesInterface[0].State.Enabled)
	data.Status = types.StringValue(respData.OpenconfigInterfacesInterface[0].State.OperStatus)
	data.NativeVlan = types.Int64Value(int64(respData.OpenconfigInterfacesInterface[0].OpenconfigIfEthernetEthernet.OpenconfigVlanSwitchedVlan.Config.NativeVlan))
	data.TrunkVlans, _ = types.ListValueFrom(ctx, types.Int64Type, respData.OpenconfigInterfacesInterface[0].OpenconfigIfEthernetEthernet.OpenconfigVlanSwitchedVlan.Config.TrunkVlans)
}

func getInterfaceConfig(ctx context.Context, data *InterfaceResourceModel) *f5ossdk.F5ReqOpenconfigInterface {
	interfaceReq := f5ossdk.F5ReqInterface{}
	interfaceReq.Name = data.Name.ValueString()
	interfaceReq.Config.Name = data.Name.ValueString()
	interfaceReq.Config.Type = "iana-if-type:ethernetCsmacd"
	interfaceReq.Config.Enabled = data.Enabled.ValueBool()
	interfaceReq.OpenconfigIfEthernetEthernet.OpenconfigVlanSwitchedVlan.Config.NativeVlan = int(data.NativeVlan.ValueInt64())
	var trunkIds []int
	data.TrunkVlans.ElementsAs(ctx, &trunkIds, false)
	interfaceReq.OpenconfigIfEthernetEthernet.OpenconfigVlanSwitchedVlan.Config.TrunkVlans = trunkIds
	interfaceOpenconfigReq := f5ossdk.F5ReqOpenconfigInterface{}
	interfaceOpenconfigReq.OpenconfigInterfacesInterfaces.Interface = append(interfaceOpenconfigReq.OpenconfigInterfacesInterfaces.Interface, interfaceReq)
	return &interfaceOpenconfigReq
}
