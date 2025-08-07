package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// var (
//	mutex sync.Mutex
// )

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TenantResource{}
var _ resource.ResourceWithImportState = &TenantResource{}

func NewTenantResource() resource.Resource {
	return &TenantResource{}
}

// TenantResource defines the resource implementation.
type TenantResource struct {
	client   *f5ossdk.F5os
	teemData *TeemData
}

// TenantResourceModel describes the resource data model.
type TenantResourceModel struct {
	Name                types.String `tfsdk:"name"`
	DeploymentFile      types.String `tfsdk:"deployment_file"`
	ImageName           types.String `tfsdk:"image_name"`
	Cryptos             types.String `tfsdk:"cryptos"`
	Type                types.String `tfsdk:"type"`
	RunningState        types.String `tfsdk:"running_state"`
	MgmtIP              types.String `tfsdk:"mgmt_ip"`
	MgmtGateway         types.String `tfsdk:"mgmt_gateway"`
	MgmtPrefix          types.Int64  `tfsdk:"mgmt_prefix"`
	CpuCores            types.Int64  `tfsdk:"cpu_cores"`
	Nodes               types.List   `tfsdk:"nodes"`
	Vlans               types.List   `tfsdk:"vlans"`
	Status              types.String `tfsdk:"status"`
	MacBlockSize        types.String `tfsdk:"mac_block_size"`
	DagIpv6prefixLength types.Int64  `tfsdk:"dag_ipv6_prefix_length"`
	Timeout             types.Int64  `tfsdk:"timeout"`
	VirtualdiskSize     types.Int64  `tfsdk:"virtual_disk_size"`
	Memory              types.Int64  `tfsdk:"memory"`
	Id                  types.String `tfsdk:"id"`
}

func (r *TenantResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenant"
}

func (r *TenantResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Resource used for Manage F5OS tenant on chassis partition/rSeries Appliance\n\n" +
			"~> **NOTE** `f5os_tenant` resource is used with chassis partition/rSeries appliance, More info on [Tenant](https://techdocs.f5.com/en-us/velos-1-5-0/velos-systems-administration-configuration/title-tenant-management.html#title-tenant-management)." +
			"\nProvider `f5os` credentials will be chassis partition/rSeries appliance `host`,`username` and `password`",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the tenant.\nThe first character must be a letter.\nOnly lowercase alphanumeric characters are allowed.\nNo special or extended characters are allowed except for hyphens.\nThe name cannot exceed 50 characters.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"image_name": schema.StringAttribute{
				MarkdownDescription: "Name of the tenant image to be used.\nRequired for create operations",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"deployment_file": schema.StringAttribute{
				MarkdownDescription: "Deployment file used for BIG-IP-Next .\nRequired for if `type` is `BIG-IP-Next`.",
				Optional:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Name of the tenant image to be used.\nRequired for create operations",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"BIG-IP", "BIG-IP-Next"}...),
				},
				Default: stringdefault.StaticString("BIG-IP"),
			},
			"mac_block_size": schema.StringAttribute{
				MarkdownDescription: "Configure a BIG-IP tenant on these systems to use contiguous block of MAC allocation.\nDefault value is `one`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"one", "small", "medium", "large"}...),
				},
			},
			"dag_ipv6_prefix_length": schema.Int64Attribute{
				MarkdownDescription: "Configuring DAG Global IPv6 Prefix Length,value Range from `1` to `128`.Default is `128`.",
				Optional:            true,
				Default:             int64default.StaticInt64(128),
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.Between(1, 128),
				},
			},
			"cpu_cores": schema.Int64Attribute{
				MarkdownDescription: "The number of vCPUs that should be added to the tenant.\nRequired for create operations.",
				Required:            true,
			},
			"running_state": schema.StringAttribute{
				MarkdownDescription: "Desired running_state of the tenant.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"configured", "deployed"}...),
				},
				Default: stringdefault.StaticString("configured"),
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
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"enabled", "disabled"}...),
				},
				Default: stringdefault.StaticString("enabled"),
			},
			"nodes": schema.ListAttribute{
				MarkdownDescription: "List of integers. Specifies on which blades nodes the tenants are deployed.\nRequired for create operations.\nFor single blade platforms like rSeries only the value of 1 should be provided.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.Int64Type,
				Default: listdefault.StaticValue(
					types.ListValueMust(
						types.Int64Type,
						[]attr.Value{types.Int64Value(1)},
					),
				),
			},
			"vlans": schema.ListAttribute{
				MarkdownDescription: "The existing VLAN IDs in the chassis partition that should be added to the tenant.\nThe order of these VLANs is ignored.\nThis module orders the VLANs automatically, if you deliberately re-order them in subsequent tasks, this module will not register a change.\nRequired for create operations",
				Optional:            true,
				ElementType:         types.Int64Type,
			},
			"timeout": schema.Int64Attribute{
				MarkdownDescription: "The number of seconds to wait for image import to finish.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(360),
			},
			"virtual_disk_size": schema.Int64Attribute{
				MarkdownDescription: "Minimum virtual disk size required for Tenant deployment",
				Required:            true,
			},
			"memory": schema.Int64Attribute{
				MarkdownDescription: "The amount of memory that should be provided to the tenant in MB.\n More information on memory sizing for [Velos](https://clouddocs.f5.com/training/community/velos-training/html/velos_performance_and_sizing.html#memory-sizing)/[rSeries](https://clouddocs.f5.com/training/community/rseries-training/html/rseries_performance_and_sizing.html#memory-sizing)",
				Optional:            true,
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Tenant status",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique F5OS Tenant identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *TenantResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
	teemData.ProviderName = "f5os"
	teemData.ResourceName = "f5os_tenant"
	r.teemData = teemData
}

func (r *TenantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *TenantResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, fmt.Sprintf("[CREATE] Tenant:%+v", data.Name.ValueString()))
	if r.client.PlatformType == "Velos Controller" {
		resp.Diagnostics.AddError("Unsupported platform for resource", "`f5os_tenant` resource is supported with Velos Partition level (or) rSeries appliance")
		return
	}
	if data.Type.ValueString() == "BIG-IP-Next" {
		if data.DeploymentFile.IsNull() {
			resp.Diagnostics.AddError("Invalid Config for resource", "if `f5os_tenant` resource attribute `type` is `BIG-IP-Next`,then `deployment_file` option should also be specified")
			return
		}
	}
	stop := r.client.F5OsKeepAlive(15 * time.Second)
	imageObj, err := r.client.GetImage(data.ImageName.ValueString())
	if err != nil {
		stop <- true
		resp.Diagnostics.AddError(fmt.Sprintf("%v", err), "")
		return
	}
	var availableFlag = true
	for _, val := range imageObj.TenantImages {
		if val.Name == data.ImageName.ValueString() && val.Status == "not-present" {
			availableFlag = false
		}
	}
	if !availableFlag {
		stop <- true
		resp.Diagnostics.AddError(fmt.Sprintf("%v", err), "")
		return
	}

	tenantConfig := r.getTenantCreateConfig(ctx, req, resp)

	if data.Type.ValueString() == "BIG-IP-Next" {
		tenantConfig.F5TenantsTenant[0].Config.DeploymentFile = data.DeploymentFile.ValueString()
	}
	tflog.Info(ctx, fmt.Sprintf("tenantConfig Data:%+v", tenantConfig))

	// mutex.Lock()
	teemInfo := make(map[string]any)
	teemInfo["teemData"] = r.teemData
	r.client.Metadata = teemInfo
	tflog.Info(ctx, fmt.Sprintf("Timeout :%+v", int(data.Timeout.ValueInt64())))
	respByte, err := r.client.CreateTenant(tenantConfig, int(data.Timeout.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("%v", err.Error()), "")
		if strings.Contains(err.Error(), "400 Bad Request") {
			stop <- true
			return
		}
		if strings.Contains(err.Error(), "object already exists") {
			stop <- true
			return
		}
		_ = r.client.DeleteTenant(data.Name.ValueString())
		stop <- true
		return
	}
	tflog.Info(ctx, fmt.Sprintf("tenantConfig Response:%+v", string(respByte)))

	// save into the Terraform state.
	data.Id = types.StringValue(data.Name.ValueString())

	respByte2, err := r.client.GetTenant(data.Name.ValueString())
	if err != nil {
		stop <- true
		resp.Diagnostics.AddError(fmt.Sprintf("%v", err.Error()), "")
		return
	}
	stop <- true
	tflog.Info(ctx, fmt.Sprintf("get tenantConfig :%+v", respByte2))
	r.tenantResourceModeltoState(ctx, respByte2, data)
	// mutex.Unlock()

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
	// respByte, err := r.client.GetTenant(data.Name.ValueString())
	stop := r.client.F5OsKeepAlive(15 * time.Second)
	respByte, err := r.client.GetTenant(data.Id.ValueString())
	if err != nil {
		stop <- true
		resp.Diagnostics.AddError(fmt.Sprintf("%v", err.Error()), "")
		return
	}
	stop <- true
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
	tenantConfig := r.getTenantUpdateConfig(ctx, req, resp)

	if data.Type.ValueString() == "BIG-IP-Next" {
		tenantConfig.F5TenantsTenants.Tenant[0].Config.DeploymentFile = data.DeploymentFile.ValueString()
	}
	tflog.Info(ctx, fmt.Sprintf("[Update] tenantConfig :%+v", tenantConfig))
	// mutex.Lock()
	stop := r.client.F5OsKeepAlive(15 * time.Second)
	respByte, err := r.client.UpdateTenant(tenantConfig, int(data.Timeout.ValueInt64()))
	if err != nil {
		stop <- true
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Tenant Deploy failed, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("[Update] tenantConfig resp :%+v", string(respByte)))

	respByte2, err := r.client.GetTenant(data.Name.ValueString())
	if err != nil {
		stop <- true
		resp.Diagnostics.AddError(fmt.Sprintf("%v", err.Error()), "")
		return
	}
	stop <- true
	r.tenantResourceModeltoState(ctx, respByte2, data)
	tflog.Info(ctx, fmt.Sprintf("Updated State:%+v", data))
	// mutex.Unlock()
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
	stop := r.client.F5OsKeepAlive(15 * time.Second)
	err := r.client.DeleteTenant(data.Name.ValueString())
	stop <- true
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("%v", err.Error()), "")
		return
	}
}

func (r *TenantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *TenantResource) tenantResourceModeltoState(ctx context.Context, respData *f5ossdk.F5RespTenants, data *TenantResourceModel) {
	tflog.Info(ctx, fmt.Sprintf("tenantResourceModeltoState:%+v", respData))
	data.ImageName = types.StringValue(respData.F5TenantsTenant[0].State.Image)
	data.Name = types.StringValue(respData.F5TenantsTenant[0].Name)
	data.RunningState = types.StringValue(respData.F5TenantsTenant[0].State.RunningState)
	data.MgmtIP = types.StringValue(respData.F5TenantsTenant[0].State.MgmtIp)
	data.MgmtPrefix = types.Int64Value(int64(respData.F5TenantsTenant[0].State.PrefixLength))
	data.CpuCores = types.Int64Value(int64(respData.F5TenantsTenant[0].State.VcpuCoresPerNode))
	data.Nodes, _ = types.ListValueFrom(ctx, types.Int64Type, respData.F5TenantsTenant[0].Config.Nodes)
	data.MgmtGateway = types.StringValue(respData.F5TenantsTenant[0].State.Gateway)
	data.Status = types.StringValue(respData.F5TenantsTenant[0].State.Status)
	data.DagIpv6prefixLength = types.Int64Value(int64(respData.F5TenantsTenant[0].State.DagIpv6PrefixLength))
	if respData.F5TenantsTenant[0].State.MacData.MacPoolSize == 1 {
		data.MacBlockSize = types.StringValue("one")
	}
	if respData.F5TenantsTenant[0].State.MacData.MacPoolSize == 8 {
		data.MacBlockSize = types.StringValue("small")
	}
	if respData.F5TenantsTenant[0].State.MacData.MacPoolSize == 16 {
		data.MacBlockSize = types.StringValue("medium")
	}
	if respData.F5TenantsTenant[0].State.MacData.MacPoolSize == 32 {
		data.MacBlockSize = types.StringValue("large")
	}
	if respData.F5TenantsTenant[0].State.Storage.Size == respData.F5TenantsTenant[0].Config.Storage.Size {
		data.VirtualdiskSize = types.Int64Value(int64(respData.F5TenantsTenant[0].State.Storage.Size))
	} else {
		data.VirtualdiskSize = types.Int64Value(int64(respData.F5TenantsTenant[0].Config.Storage.Size))
	}
	memoryInt, _ := strconv.Atoi(respData.F5TenantsTenant[0].State.Memory)
	if !data.Memory.IsNull() {
		data.Memory = types.Int64Value(int64(memoryInt))
	}
	data.Cryptos = types.StringValue(respData.F5TenantsTenant[0].State.Cryptos)
}

func (r *TenantResource) getTenantCreateConfig(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) *f5ossdk.F5ReqTenants {
	var data *TenantResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	tenantSubbj := f5ossdk.F5ReqTenant{}
	tenantSubbj.Name = data.Name.ValueString()
	tenantSubbj.Config.Name = data.Name.ValueString()
	tenantSubbj.Config.Image = data.ImageName.ValueString()
	tenantSubbj.Config.Gateway = data.MgmtGateway.ValueString()
	tenantSubbj.Config.Type = data.Type.ValueString()
	tenantSubbj.Config.MgmtIp = data.MgmtIP.ValueString()
	tenantSubbj.Config.PrefixLength = int(data.MgmtPrefix.ValueInt64())
	tenantSubbj.Config.VcpuCoresPerNode = int(data.CpuCores.ValueInt64())
	tenantSubbj.Config.DagIpv6PrefixLength = int(data.DagIpv6prefixLength.ValueInt64())
	if !data.MacBlockSize.IsNull() && !data.MacBlockSize.IsUnknown() {
		tenantSubbj.Config.MacData.F5TenantL2InlineMacBlockSize = data.MacBlockSize.ValueString()
		// tenantSubbj.Config.MacData.F5TenantL2InlineMacBlockSize = "one"
	}
	//  else {
	// 	tenantSubbj.Config.MacData.F5TenantL2InlineMacBlockSize = data.MacBlockSize.ValueString()
	// }
	// tenantSubbj.Config.MacData.F5TenantL2InlineMacBlockSize = data.MacBlockSize.ValueString()
	tenantSubbj.Config.Memory = calculateMemory(data, r.client.PlatformType)
	data.Vlans.ElementsAs(ctx, &tenantSubbj.Config.Vlans, false)
	tenantSubbj.Config.RunningState = data.RunningState.ValueString()
	tenantSubbj.Config.Cryptos = data.Cryptos.ValueString()
	data.Nodes.ElementsAs(ctx, &tenantSubbj.Config.Nodes, false)
	tenantSubbj.Config.Storage.Size = int(data.VirtualdiskSize.ValueInt64())

	tenantConfig := new(f5ossdk.F5ReqTenants)
	tenantConfig.F5TenantsTenant = append(tenantConfig.F5TenantsTenant, tenantSubbj)
	return tenantConfig
}

func (r *TenantResource) getTenantUpdateConfig(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) *f5ossdk.F5ReqTenantsPatch {
	var data *TenantResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	tenantSubbj := f5ossdk.F5ReqTenant{}
	tenantSubbj.Name = data.Name.ValueString()
	tenantSubbj.Config.Name = data.Name.ValueString()
	tenantSubbj.Config.Image = data.ImageName.ValueString()
	tenantSubbj.Config.Gateway = data.MgmtGateway.ValueString()
	tenantSubbj.Config.Type = data.Type.ValueString()
	tenantSubbj.Config.MgmtIp = data.MgmtIP.ValueString()
	tenantSubbj.Config.PrefixLength = int(data.MgmtPrefix.ValueInt64())
	tenantSubbj.Config.VcpuCoresPerNode = int(data.CpuCores.ValueInt64())
	tenantSubbj.Config.DagIpv6PrefixLength = int(data.DagIpv6prefixLength.ValueInt64())
	tenantSubbj.Config.MacData.F5TenantL2InlineMacBlockSize = data.MacBlockSize.ValueString()
	tenantSubbj.Config.Memory = calculateMemory(data, r.client.PlatformType)
	data.Nodes.ElementsAs(ctx, &tenantSubbj.Config.Nodes, false)
	data.Vlans.ElementsAs(ctx, &tenantSubbj.Config.Vlans, false)
	tenantSubbj.Config.RunningState = data.RunningState.ValueString()
	tenantSubbj.Config.Cryptos = data.Cryptos.ValueString()
	tenantSubbj.Config.Storage.Size = int(data.VirtualdiskSize.ValueInt64())

	tenantpatchConfig := new(f5ossdk.F5ReqTenantsPatch)
	tenantpatchConfig.F5TenantsTenants.Tenant = append(tenantpatchConfig.F5TenantsTenants.Tenant, tenantSubbj)
	tflog.Info(ctx, fmt.Sprintf("getTenantUpdateConfig:%+v", tenantpatchConfig))
	return tenantpatchConfig
}

// Helper for platform type check
func isRSeriesPlatform(platform string) bool {
	return platform == "r2800" || platform == "r2000" || platform == "r4000" || platform == "r4800"
}

// Helper for memory sizing
func calculateMemory(data *TenantResourceModel, platformType string) int {
	if !data.Memory.IsNull() && !data.Memory.IsUnknown() {
		return int(data.Memory.ValueInt64())
	}
	cpuCores := int(data.CpuCores.ValueInt64())
	if isRSeriesPlatform(platformType) {
		return 3 * 1024 * cpuCores
	}
	return int(3.5*1024*float64(cpuCores)) + 512
}
