package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
	"gitswarm.f5net.com/terraform-providers/terraform-provider-f5os/internal/provider/attribute_plan_modifier"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TenantResource{}
var _ resource.ResourceWithImportState = &TenantResource{}

func NewTenantResource() resource.Resource {
	return &TenantResource{}
}

// TenantResource defines the resource implementation.
type TenantResource struct {
	client *f5ossdk.F5os
}

// TenantResourceModel describes the resource data model.
type TenantResourceModel struct {
	Name            types.String `tfsdk:"name"`
	ImageName       types.String `tfsdk:"image_name"`
	Cryptos         types.String `tfsdk:"cryptos"`
	Type            types.String `tfsdk:"type"`
	RunningState    types.String `tfsdk:"running_state"`
	MgmtIP          types.String `tfsdk:"mgmt_ip"`
	MgmtGateway     types.String `tfsdk:"mgmt_gateway"`
	MgmtPrefix      types.Int64  `tfsdk:"mgmt_prefix"`
	CpuCores        types.Int64  `tfsdk:"cpu_cores"`
	Nodes           types.List   `tfsdk:"nodes"`
	Vlans           types.List   `tfsdk:"vlans"`
	Status          types.String `tfsdk:"status"`
	Timeout         types.Int64  `tfsdk:"timeout"`
	VirtualdiskSize types.Int64  `tfsdk:"virtual_disk_size"`
	Id              types.String `tfsdk:"id"`
}

func (r *TenantResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenant"
}

func (r *TenantResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Resource used for Manage F5OS tenant",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the tenant.\nThe first character must be a letter.\nOnly lowercase alphanumeric characters are allowed.\nNo special or extended characters are allowed except for hyphens.\nThe name cannot exceed 50 characters.",
				Required:            true,
			},
			"image_name": schema.StringAttribute{
				MarkdownDescription: "Name of the tenant image to be used.\nRequired for create operations",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Name of the tenant image to be used.\nRequired for create operations",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					attribute_plan_modifier.StringDefaultValue(types.StringValue("BIG-IP"))},
			},
			"cpu_cores": schema.Int64Attribute{
				MarkdownDescription: "The number of vCPUs that should be added to the tenant.\nRequired for create operations.",
				Optional:            true,
			},
			"running_state": schema.StringAttribute{
				MarkdownDescription: "Desired running_state of the tenant.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					attribute_plan_modifier.StringDefaultValue(types.StringValue("configured"))},
			},
			"mgmt_ip": schema.StringAttribute{
				MarkdownDescription: "IP address used to connect to the deployed tenant.\nRequired for create operations.",
				Required:            true,
			},
			"mgmt_gateway": schema.StringAttribute{
				MarkdownDescription: "Tenant management gateway.",
				Required:            true,
			},
			"mgmt_prefix": schema.Int64Attribute{
				MarkdownDescription: "Tenant management CIDR prefix.",
				Required:            true,
			},
			"cryptos": schema.StringAttribute{
				MarkdownDescription: "Whether crypto and compression hardware offload should be enabled on the tenant.\nWe recommend it is enabled, otherwise crypto and compression may be processed in CPU.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					attribute_plan_modifier.StringDefaultValue(types.StringValue("disabled"))},
			},
			"nodes": schema.ListAttribute{
				MarkdownDescription: "List of integers. Specifies on which blades nodes the tenants are deployed.\nRequired for create operations.\nFor single blade platforms like rSeries only the value of 1 should be provided.",
				Optional:            true,
				ElementType:         types.Int64Type,
			},
			"vlans": schema.ListAttribute{
				MarkdownDescription: "The existing VLAN IDs in the chassis partition that should be added to the tenant.\nThe order of these VLANs is ignored.\nThis module orders the VLANs automatically, if you deliberately re-order them in subsequent tasks, this module will not register a change.\nRequired for create operations",
				Optional:            true,
				ElementType:         types.Int64Type,
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Tenant status",
			},
			"timeout": schema.Int64Attribute{
				MarkdownDescription: "The number of seconds to wait for image import to finish.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					attribute_plan_modifier.Int64DefaultValue(types.Int64Value(360))},
			},
			"virtual_disk_size": schema.Int64Attribute{
				MarkdownDescription: "Minimum virtual disk size required for Tenant deployment",
				Optional:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Tenant identifier",
			},
		},
	}
}

func (r *TenantResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (r *TenantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *TenantResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	if r.client.PlatformType == "Velos Controller" {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("`f5os_tenant` resource is supported with Velos Partition level (or) rSeries appliance"))
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.

	tenantConfig := getTenantCreateConfig(ctx, req, resp)

	tflog.Info(ctx, fmt.Sprintf("tenantConfig Data:%+v", tenantConfig))
	tflog.Info(ctx, fmt.Sprintf("PlatformType Data:%+v", r.client.PlatformType))
	if r.client.PlatformType == "Velos Partition" {
		tenantConfig.F5TenantsTenant[0].Config.Memory = 3.5*1024*int(data.CpuCores.ValueInt64()) + (512)
	}

	respByte, err := r.client.CreateTenant(tenantConfig, int(data.Timeout.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Tenant Deploy failed, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("tenantConfig Data:%+v", string(respByte)))

	// For the purposes of this example code, hardcoding a response value to
	// save into the Terraform state.
	data.Id = types.StringValue(data.Name.ValueString())

	respByte2, err := r.client.GetTenant(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read/Get Tenants, got error: %s", err))
		return
	}
	r.tenantResourceModeltoState(ctx, respByte2, data)
	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	//tflog.Trace(ctx, "created a resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TenantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *TenantResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	respByte, err := r.client.GetTenant(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read/Get Tenants, got error: %s", err))
		return
	}
	r.tenantResourceModeltoState(ctx, respByte, data)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TenantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *TenantResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	tenantConfig := getTenantUpdateConfig(ctx, req, resp)

	if r.client.PlatformType == "Velos Partition" {
		tenantConfig.F5TenantsTenants.Tenant[0].Config.Memory = 3.5*1024*int(data.CpuCores.ValueInt64()) + (512)
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	respByte, err := r.client.UpdateTenant(tenantConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Tenant Deploy failed, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("tenantConfig Data:%+v", string(respByte)))
	respByte2, err := r.client.GetTenant(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read/Get Tenants, got error: %s", err))
		return
	}
	r.tenantResourceModeltoState(ctx, respByte2, data)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TenantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *TenantResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	err := r.client.DeleteTenant(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to Delete Imported Image, got error: %s", err))
		return
	}
}

func (r *TenantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *TenantResource) tenantResourceModeltoState(ctx context.Context, respData *f5ossdk.TenantsStatusObj, data *TenantResourceModel) {
	tflog.Info(ctx, fmt.Sprintf("respData :%+v", respData))
	data.ImageName = types.StringValue(respData.F5TenantsTenant[0].State.Image)
	data.Name = types.StringValue(respData.F5TenantsTenant[0].Name)
	data.RunningState = types.StringValue(respData.F5TenantsTenant[0].State.RunningState)
	data.MgmtIP = types.StringValue(respData.F5TenantsTenant[0].State.MgmtIp)
	data.MgmtGateway = types.StringValue(respData.F5TenantsTenant[0].State.Gateway)
	data.Status = types.StringValue(respData.F5TenantsTenant[0].State.Status)
	data.VirtualdiskSize = types.Int64Value(int64(respData.F5TenantsTenant[0].State.Storage.Size))
}
func getTenantCreatebackupConfig(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) *f5ossdk.TenantObj {
	var data *TenantResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	tenantConfig := &f5ossdk.TenantObj{}
	tenantConfig.Name = data.Name.ValueString()
	tenantSubConfig := f5ossdk.TenantConfig{}
	tenantSubConfig.Image = data.ImageName.ValueString()
	tenantSubConfig.MgmtIp = data.MgmtIP.ValueString()
	tenantSubConfig.Nodes = []int{1}
	tenantSubConfig.Gateway = data.MgmtGateway.ValueString()
	tenantSubConfig.PrefixLength = int(data.MgmtPrefix.ValueInt64())
	tenantSubConfig.VcpuCoresPerNode = int(data.CpuCores.ValueInt64())
	tenantSubConfig.Memory = 3 * 1024 * int(data.CpuCores.ValueInt64())
	tenantSubConfig.RunningState = data.RunningState.String()
	data.Vlans.ElementsAs(ctx, &tenantSubConfig.Vlans, false)
	tenantConfig.Config = tenantSubConfig
	return tenantConfig
}

func getTenantCreateConfig(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) *f5ossdk.TenantsObj {
	var data *TenantResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	tenantSubbj := f5ossdk.TenantObjs{}
	tenantSubbj.Name = data.Name.ValueString()
	tenantSubbj.Config.Name = data.Name.ValueString()
	tenantSubbj.Config.Image = data.ImageName.ValueString()
	tenantSubbj.Config.Gateway = data.MgmtGateway.ValueString()
	tenantSubbj.Config.Type = data.Type.ValueString()
	tenantSubbj.Config.MgmtIp = data.MgmtIP.ValueString()
	tenantSubbj.Config.PrefixLength = int(data.MgmtPrefix.ValueInt64())
	tenantSubbj.Config.VcpuCoresPerNode = int(data.CpuCores.ValueInt64())
	tenantSubbj.Config.Memory = 3 * 1024 * int(data.CpuCores.ValueInt64())
	data.Vlans.ElementsAs(ctx, tenantSubbj.Config.Vlans, false)
	tenantSubbj.Config.PrefixLength = int(data.MgmtPrefix.ValueInt64())
	tenantSubbj.Config.RunningState = data.RunningState.ValueString()
	tenantSubbj.Config.Nodes = []int{1}
	tenantSubbj.Config.Storage.Size = int(data.VirtualdiskSize.ValueInt64())

	tenantConfig := new(f5ossdk.TenantsObj)
	tenantConfig.F5TenantsTenant = append(tenantConfig.F5TenantsTenant, tenantSubbj)
	return tenantConfig
}

func getTenantUpdateConfig(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) *f5ossdk.TenantsPatchObj {
	var data *TenantResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	tenantSubbj := f5ossdk.TenantObjs{}
	tenantSubbj.Name = data.Name.ValueString()
	tenantSubbj.Config.Name = data.Name.ValueString()
	tenantSubbj.Config.Image = data.ImageName.ValueString()
	tenantSubbj.Config.Gateway = data.MgmtGateway.ValueString()
	tenantSubbj.Config.Type = data.Type.ValueString()
	tenantSubbj.Config.MgmtIp = data.MgmtIP.ValueString()
	tenantSubbj.Config.PrefixLength = int(data.MgmtPrefix.ValueInt64())
	tenantSubbj.Config.VcpuCoresPerNode = int(data.CpuCores.ValueInt64())
	tenantSubbj.Config.Memory = 3 * 1024 * int(data.CpuCores.ValueInt64())
	data.Vlans.ElementsAs(ctx, tenantSubbj.Config.Vlans, false)
	tenantSubbj.Config.PrefixLength = int(data.MgmtPrefix.ValueInt64())
	tenantSubbj.Config.RunningState = data.RunningState.ValueString()
	tenantSubbj.Config.Nodes = []int{1}
	tenantSubbj.Config.Storage.Size = int(data.VirtualdiskSize.ValueInt64())

	tenantpatchConfig := new(f5ossdk.TenantsPatchObj)
	tenantpatchConfig.F5TenantsTenants.Tenant = append(tenantpatchConfig.F5TenantsTenants.Tenant, tenantSubbj)
	tflog.Info(ctx, fmt.Sprintf("getTenantUpdateConfig:%+v", tenantpatchConfig))
	return tenantpatchConfig
}
