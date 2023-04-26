package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
	"gitswarm.f5net.com/terraform-providers/terraform-provider-f5os/internal/provider/attribute_plan_modifier"
	"net"
	"regexp"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &PartitionResource{}
var _ resource.ResourceWithImportState = &PartitionResource{}

func NewPartitionResource() resource.Resource {
	return &PartitionResource{}
}

// PartitionResource defines the resource implementation.
type PartitionResource struct {
	client *f5ossdk.F5os
}

type PartitionResourceModel struct {
	Name            types.String `tfsdk:"name"`
	IPv4MgmtAddress types.String `tfsdk:"ipv4_mgmt_address"`
	IPv4MgmtGateway types.String `tfsdk:"ipv4_mgmt_gateway"`
	IPv6MgmtAddress types.String `tfsdk:"ipv6_mgmt_address"`
	IPv6MgmtGateway types.String `tfsdk:"ipv6_mgmt_gateway"`
	OsVersion       types.String `tfsdk:"os_version"`
	Slots           types.List   `tfsdk:"slots"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	Timeout         types.Int64  `tfsdk:"timeout"`
	Id              types.String `tfsdk:"id"`
}

func (r *PartitionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_partition"
}

func (r *PartitionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Resource used for Manage VELOS chassis partition",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the chassis partition.\nThe first character must be a letter.\nOnly lowercase alphanumeric characters are allowed.\nNo special or extended characters are allowed except for hyphens.\nThe name cannot exceed 50 characters.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ipv4_mgmt_address": schema.StringAttribute{
				MarkdownDescription: "Specifies the IPv4 address and subnet mask used to access the chassis partition.\nThe address must be specified in CIDR notation e.g. 192.168.1.1/24.\nRequired for create operations.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^([01]?\d{1,2}|2[0-4]\d|25[0-5])\.([01]?\d{1,2}|2[0-4]\d|25[0-5])\.([01]?\d{1,2}|2[0-4]\d|25[0-5])\.([01]?\d{1,2}|2[0-4]\d|25[0-5])/([12]?\d|3[0-2])$`),
						"given ipv4_mgmt_address must be a valid IPV4 address in CIDR format",
					),
				},
			},
			"ipv4_mgmt_gateway": schema.StringAttribute{
				MarkdownDescription: "Specifies the IPv4 chassis partition management gateway.\nRequired for create operations.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^([01]?\d{1,2}|2[0-4]\d|25[0-5])\.([01]?\d{1,2}|2[0-4]\d|25[0-5])\.([01]?\d{1,2}|2[0-4]\d|25[0-5])\.([01]?\d{1,2}|2[0-4]\d|25[0-5])$`),
						"given ipv4_mgmt_gateway is not a valid IPV4 address",
					),
				},
			},
			"ipv6_mgmt_address": schema.StringAttribute{
				MarkdownDescription: "Specifies the IPv6 address and subnet mask used to access the chassis partition.\nThe address must be specified in CIDR notation e.g. 2002::1234:abcd:ffff:c0a8:101/64.\nRequired for create operations.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}/(1[0-1]\d|[12]\d|[0-9])$`),
						"given ipv6_mgmt_address must be a valid IPV6 address in CIDR format",
					),
				},
			},
			"ipv6_mgmt_gateway": schema.StringAttribute{
				MarkdownDescription: "Specifies the IPv6 chassis partition management gateway.\nRequired for create operations.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$`),
						"given ipv6_mgmt_gateway is not a valid IPV6 address",
					),
				},
			},
			"os_version": schema.StringAttribute{
				MarkdownDescription: "Specifies the partition F5OS-C OS version.",
				Optional:            true,
				Computed:            true,
			},
			"slots": schema.ListAttribute{
				MarkdownDescription: "List of integers.\nSpecifies which slots with which the chassis partition should associated.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.Int64Type,
				Validators: []validator.List{
					listvalidator.ValueInt64sAre(int64validator.Between(0, 32)),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Enables or disables partition.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					attribute_plan_modifier.BoolDefaultValue(types.BoolValue(true)),
				},
			},
			"timeout": schema.Int64Attribute{
				MarkdownDescription: "The number of seconds to wait for partition to transition to running state.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					attribute_plan_modifier.Int64DefaultValue(types.Int64Value(360)),
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Tenant identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *PartitionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (r *PartitionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *PartitionResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if r.client.PlatformType != "Velos Controller" {
		resp.Diagnostics.AddError("Client Error", "`velos_partition` resource is supported on Velos Controllers only")
		return
	}

	partitionConfig := getPartitionCreateConfig(ctx, req, resp)
	tflog.Info(ctx, fmt.Sprintf("partitionConfig Data:%+v", partitionConfig))

	respByte, err := r.client.CreatePartition(partitionConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Create Partition failed, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("partitionConfig Response:%+v", string(respByte)))

	if !data.Slots.IsNull() && !data.Slots.IsUnknown() {
		var slots []int64
		data.Slots.ElementsAs(ctx, &slots, false)
		_, err := r.client.SetSlot(data.Name.ValueString(), slots)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to add slots to Partition, got error: %s", err))
			return
		}
	}
	respByte3, err := r.client.CheckPartitionState(data.Name.ValueString(), int(data.Timeout.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Waiting for Partition deploy, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("Partition Deploy Response:%+v", string(respByte3)))
	data.Id = types.StringValue(data.Name.ValueString())

	partData, err := r.client.GetPartition(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read/Get Partition, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("get partitionConfig :%+v", partData))

	slotData, err := r.client.GetPartitionSlots(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read Partition slots got error: %s", err))
		return
	}

	r.partitionResourceModelToState(ctx, partData, data)

	// Add slots to model if slots associated with partition found
	if slotData != nil {
		slots, diags := types.ListValueFrom(ctx, types.Int64Type, slotData)

		resp.Diagnostics.Append(diags...)

		if resp.Diagnostics.HasError() {
			return
		}

		data.Slots = slots
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

func (r *PartitionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *PartitionResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	partData, err := r.client.GetPartition(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read/Get Partition, got error: %s", err))
		return
	}

	slotData, err := r.client.GetPartitionSlots(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read Partition slots got error: %s", err))
		return
	}

	r.partitionResourceModelToState(ctx, partData, data)

	// Add slots to model if slots associated with partition found
	if slotData != nil {
		slots, diags := types.ListValueFrom(ctx, types.Int64Type, slotData)

		resp.Diagnostics.Append(diags...)

		if resp.Diagnostics.HasError() {
			return
		}

		data.Slots = slots
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PartitionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *PartitionResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.OsVersion.IsNull() && !data.OsVersion.IsUnknown() {
		success, err := r.client.UpdatePartitionIso(data.Name.ValueString(), data.OsVersion.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to change partition os_version got error: %s", err))
			return
		}
		if success {
			tflog.Info(ctx, "Updated ISO version on partition successfully")
		}
	}

	if !data.Slots.IsNull() && !data.Slots.IsUnknown() {
		slotData, err := r.client.GetPartitionSlots(data.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read Partition slots got error: %s", err))
			return
		}
		if slotData != nil {
			// first we determine if a subset of slots on partition are not included in user data, and if yes we remove them first
			var slots []int64
			data.Slots.ElementsAs(ctx, &slots, false)
			slotDiff := getSliceDifference(slotData, slots)
			if len(slotDiff) > 0 {
				_, err := r.client.SetSlot("none", slotDiff)
				if err != nil {
					resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to disassociate slots from Partition, got error: %s", err))
					return
				}
			}
			// next we update slots on partition
			data.Slots.ElementsAs(ctx, &slots, false)
			_, err := r.client.SetSlot(data.Name.ValueString(), slots)
			if err != nil {
				resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to update slots on Partition, got error: %s", err))
				return
			}
		}
	}

	partitionConfig := getPartitionUpdateConfig(ctx, req, resp)

	respByte, err := r.client.UpdatePartition(data.Name.ValueString(), partitionConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Tenant Deploy failed, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("partitionConfig Data:%+v", string(respByte)))

	respByte2, err := r.client.CheckPartitionState(data.Name.ValueString(), int(data.Timeout.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Waiting for Partition state after update, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("Partition Deploy Response:%+v", string(respByte2)))
	data.Id = types.StringValue(data.Name.ValueString())

	partData, err := r.client.GetPartition(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read/Get Partition, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("get partitionConfig :%+v", partData))

	slotData, err := r.client.GetPartitionSlots(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read Partition slots got error: %s", err))
		return
	}

	r.partitionResourceModelToState(ctx, partData, data)

	// Add slots to model if slots associated with partition found
	if slotData != nil {
		slots, diags := types.ListValueFrom(ctx, types.Int64Type, slotData)

		resp.Diagnostics.Append(diags...)

		if resp.Diagnostics.HasError() {
			return
		}

		data.Slots = slots
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PartitionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *PartitionResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// first we read slots associated with partition to disassociate them
	slotData, err1 := r.client.GetPartitionSlots(data.Name.ValueString())
	if err1 != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to Read Partition slots got error: %s", err1))
		return
	}

	if slotData != nil {
		_, err2 := r.client.SetSlot("none", slotData)
		if err2 != nil {
			resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to disassociate slots from Partition, got error: %s", err2))
			return
		}
	}

	err3 := r.client.DeletePartition(data.Name.ValueString())
	if err3 != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to Partition, got error: %s", err3))
		return
	}
}

func (r *PartitionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func getPartitionUpdateConfig(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) *f5ossdk.F5ReqPartition {
	var data *PartitionResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	partitionReq := f5ossdk.F5ReqPartition{}
	partitionReq.Config.IsoVersion = data.OsVersion.ValueString()
	partitionReq.Config.Enabled = data.Enabled.ValueBool()

	if !data.IPv4MgmtAddress.IsNull() && !data.IPv4MgmtAddress.IsUnknown() {
		prefix, ip, err := extractSubnet(data.IPv4MgmtAddress.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Parameter Error:", fmt.Sprintf("Error parsing provided IPv4MgmtAddress : %s", err))
			return nil
		}
		partitionReq.Config.MgmtIp.Ipv4.Address = ip
		partitionReq.Config.MgmtIp.Ipv4.PrefixLength = prefix
		partitionReq.Config.MgmtIp.Ipv4.Gateway = data.IPv4MgmtGateway.ValueString()

	}
	if !data.IPv6MgmtAddress.IsNull() && !data.IPv6MgmtAddress.IsUnknown() {
		prefix, ip, err := extractSubnet(data.IPv6MgmtAddress.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Parameter Error:", fmt.Sprintf("Error parsing provided IPv6MgmtAddress : %s", err))
			return nil
		}
		partitionReq.Config.MgmtIp.Ipv6.Address = ip
		partitionReq.Config.MgmtIp.Ipv6.PrefixLength = prefix
		partitionReq.Config.MgmtIp.Ipv6.Gateway = data.IPv6MgmtGateway.ValueString()
	}

	tflog.Info(ctx, fmt.Sprintf("getPartitionUpdateConfig:%+v", partitionReq))
	return &partitionReq
}

func (r *PartitionResource) partitionResourceModelToState(ctx context.Context, respData *f5ossdk.F5RespPartitions, data *PartitionResourceModel) {
	data.Name = types.StringValue(respData.Partition[0].Name)
	data.Enabled = types.BoolValue(respData.Partition[0].Config.Enabled)
	data.OsVersion = types.StringValue(respData.Partition[0].Config.IsoVersion)

	if respData.Partition[0].Config.MgmtIp.Ipv4.PrefixLength != 0 {
		data.IPv4MgmtAddress = types.StringValue(fmt.Sprintf("%s/%d", respData.Partition[0].Config.MgmtIp.Ipv4.Address, int64(respData.Partition[0].Config.MgmtIp.Ipv4.PrefixLength)))
		data.IPv4MgmtGateway = types.StringValue(respData.Partition[0].Config.MgmtIp.Ipv4.Gateway)
	}

	if respData.Partition[0].Config.MgmtIp.Ipv6.PrefixLength != 0 {
		data.IPv6MgmtAddress = types.StringValue(fmt.Sprintf("%s/%d", respData.Partition[0].Config.MgmtIp.Ipv6.Address, int64(respData.Partition[0].Config.MgmtIp.Ipv6.PrefixLength)))
		data.IPv6MgmtGateway = types.StringValue(respData.Partition[0].Config.MgmtIp.Ipv6.Gateway)
	}
}

func getPartitionCreateConfig(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) *f5ossdk.F5ReqPartitions {
	var data *PartitionResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	partitionReq := f5ossdk.F5ReqPartition{}
	partitionReq.Name = data.Name.ValueString()
	partitionReq.Config.IsoVersion = data.OsVersion.ValueString()
	partitionReq.Config.Enabled = data.Enabled.ValueBool()

	if !data.IPv4MgmtAddress.IsNull() && !data.IPv4MgmtAddress.IsUnknown() {
		prefix, ip, err := extractSubnet(data.IPv4MgmtAddress.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Parameter Error:", fmt.Sprintf("Error parsing provided IPv4MgmtAddress : %s", err))
			return nil
		}
		partitionReq.Config.MgmtIp.Ipv4.Address = ip
		partitionReq.Config.MgmtIp.Ipv4.PrefixLength = prefix
		partitionReq.Config.MgmtIp.Ipv4.Gateway = data.IPv4MgmtGateway.ValueString()

	}
	if !data.IPv6MgmtAddress.IsNull() && !data.IPv6MgmtAddress.IsUnknown() {
		prefix, ip, err := extractSubnet(data.IPv6MgmtAddress.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Parameter Error:", fmt.Sprintf("Error parsing provided IPv6MgmtAddress : %s", err))
			return nil
		}
		partitionReq.Config.MgmtIp.Ipv6.Address = ip
		partitionReq.Config.MgmtIp.Ipv6.PrefixLength = prefix
		partitionReq.Config.MgmtIp.Ipv6.Gateway = data.IPv6MgmtGateway.ValueString()
	}

	partitionConfig := new(f5ossdk.F5ReqPartitions)
	partitionConfig.Partition = partitionReq
	return partitionConfig
}

func extractSubnet(cidr string) (int, string, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return 0, "", err
	}

	ones, _ := ipNet.Mask.Size()
	return ones, ip.String(), nil
}

func getSliceDifference(slice1, slice2 []int64) []int64 {
	var diff []int64
	for _, num1 := range slice1 {
		found := false
		for _, num2 := range slice2 {
			if num1 == num2 {
				found = true
				break
			}
		}
		if !found {
			diff = append(diff, num1)
		}
	}
	return diff
}
