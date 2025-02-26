package provider

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

var _ datasource.DataSource = &DeviceInfoDataSource{}

func NewDeviceInfoDataSource() datasource.DataSource {
	return &DeviceInfoDataSource{}
}

type DeviceInfoDataSource struct {
	client   *f5ossdk.F5os
	teemData *TeemData
}

type InterfacesInfo struct {
	Name              types.String `tfsdk:"name"`
	Type              types.String `tfsdk:"type"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	OperationalStatus types.String `tfsdk:"operational_status"`
	Mtu               types.Int64  `tfsdk:"mtu"`
	PortSpeed         types.String `tfsdk:"port_speed"`
	L3Counters        types.Map    `tfsdk:"l3_counters"`
}

type VlansInfo struct {
	VlanId   types.Int64  `tfsdk:"vlan_id"`
	VlanName types.String `tfsdk:"vlan_name"`
}

type TenantsImageInfo struct {
	ImageName types.String `tfsdk:"image_name"`
	InUse     types.Bool   `tfsdk:"in_use"`
	Type      types.String `tfsdk:"type"`
	Status    types.String `tfsdk:"status"`
	Date      types.String `tfsdk:"date"`
	Size      types.String `tfsdk:"size"`
}

type IsoImagesInfo struct {
	Version types.String `tfsdk:"version"`
	Service types.String `tfsdk:"service"`
	Os      types.String `tfsdk:"os"`
}

type DeviceInfoDataSourceModel struct {
	Id               types.String       `tfsdk:"id"`
	GatherInfoOf     types.List         `tfsdk:"gather_info_of"`
	Interfaces       []InterfacesInfo   `tfsdk:"interfaces"`
	Vlans            []VlansInfo        `tfsdk:"vlans"`
	ControllerImages []IsoImagesInfo    `tfsdk:"controller_images"`
	PartitionImages  []IsoImagesInfo    `tfsdk:"partition_images"`
	TenantImages     []TenantsImageInfo `tfsdk:"tenant_images"`
}

func (d *DeviceInfoDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_device_info"
	teemData := &TeemData{}
	teemData.ProviderName = req.ProviderTypeName
	teemData.ResourceName = resp.TypeName
	d.teemData = teemData
}

func (d *DeviceInfoDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Get Information about the various components of F5OS device. Currently the various components whose information is fetched are `interfaces`, `vlans`, `tenant images`, `controller images` and `partition images`. Information about partition and controller images can only be fetched from the Velos controller so please set you provider block to point to a Velos controller when you want information for partition and controller images",
		Attributes: map[string]schema.Attribute{
			"gather_info_of": schema.ListAttribute{
				ElementType: types.StringType,
				Required:    true,
				MarkdownDescription: "List of components for which to gather information. This attribute accept the following values:" + "\n" +
					"[`all`,`interfaces`,`vlans`,`tenant_images`,`partition_images`,`controller_images`,`!all`,`!interfaces`,`!vlans`,`!tenant_images`,`!partition_images`,`!controller_images`]",
				Validators: []validator.List{
					listvalidator.ValueStringsAre(
						stringvalidator.OneOf(
							"all",
							"interfaces",
							"vlans",
							"controller_images",
							"partition_images",
							"tenant_images",
							"!all",
							"!interfaces",
							"!vlans",
							"!controller_images",
							"!partition_images",
							"!tenant_images",
						),
					),
				},
			},
			"interfaces": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Interface name",
						},
						"type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Interface type",
						},
						"enabled": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Interface enabled",
						},
						"operational_status": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Interface operational status",
						},
						"mtu": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Interface mtu",
						},
						"port_speed": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Interface port speed",
						},
						"l3_counters": schema.MapAttribute{
							Computed:    true,
							ElementType: types.StringType,
						},
					},
				},
				Computed:            true,
				MarkdownDescription: "Information about existing interfaces",
			},
			"vlans": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"vlan_id": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Vlan id",
						},
						"vlan_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Vlan name",
						},
					},
				},
				Computed:            true,
				MarkdownDescription: "Information about existing vlans",
			},
			"controller_images": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"version": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Version of the ISO image",
						},
						"service": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Service number of the ISO image",
						},
						"os": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "OS of the ISO image",
						},
					},
				},
				Computed:            true,
				MarkdownDescription: "Information about existing controller images",
			},
			"partition_images": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"version": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Version of the ISO image",
						},
						"service": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Service number of the ISO image",
						},
						"os": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "OS of the ISO image",
						},
					},
				},
				Computed:            true,
				MarkdownDescription: "Device info",
			},
			"tenant_images": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"image_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Image name",
						},
						"in_use": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "In use",
						},
						"type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Image Type",
						},
						"status": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Image Status",
						},
						"date": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Image Date",
						},
						"size": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Image Size",
						},
					},
				},
				Computed:            true,
				MarkdownDescription: "Information about existing tenant images",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier for Device Info",
			},
		},
	}
}

func (d *DeviceInfoDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (d *DeviceInfoDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DeviceInfoDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gather_subsets := make([]string, 0)

	data.GatherInfoOf.ElementsAs(ctx, &gather_subsets, true)

	gather_subsets = filterGatherSubsets(gather_subsets)

	for _, item := range gather_subsets {

		if item == "interfaces" {
			interfacesResp, err := d.client.GetInterfaceInfo()
			if err != nil {
				resp.Diagnostics.AddError("Error getting interface info", err.Error())
				return
			}

			interfacesInfo := convertInterfacesInfo(interfacesResp)
			data.Interfaces = interfacesInfo
		}

		if item == "vlans" {
			vlansResp, err := d.client.GetVlansInfo()
			if err != nil {
				resp.Diagnostics.AddError("Error getting vlans info", err.Error())
				return
			}

			vlansInfo := convertVlansInfo(vlansResp)
			data.Vlans = vlansInfo
		}

		if item == "controller_images" {
			controllerImagesResp, err := d.client.GetControllerImagesInfo()
			if err != nil {
				resp.Diagnostics.AddError("Error getting controller images info", err.Error())
				return
			}

			controllerImagesInfo := convertIsoImagesInfo(controllerImagesResp)
			data.ControllerImages = controllerImagesInfo
		}

		if item == "partition_images" {
			partitionImagesResp, err := d.client.GetPartitionImagesInfo()
			if err != nil {
				resp.Diagnostics.AddError("Error getting partition images info", err.Error())
				return
			}

			partitionImagesInfo := convertIsoImagesInfo(partitionImagesResp)
			data.PartitionImages = partitionImagesInfo
		}

		if item == "tenant_images" {
			tenantImagesResp, err := d.client.GetTenantImagesInfo()
			if err != nil {
				resp.Diagnostics.AddError("Error getting tenant images info", err.Error())
				return
			}

			tenantImagesInfo := convertTenantImagesInfo(tenantImagesResp)
			data.TenantImages = tenantImagesInfo
		}
	}

	id := fmt.Sprintf("device_info_%d", time.Now().UnixMilli())

	data.Id = types.StringValue(id)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func filterGatherSubsets(gatherSubset []string) []string {
	include := make([]string, 0)
	set := make(map[string]struct{})

	if slices.Contains(gatherSubset, "!all") {
		return include
	}

	if slices.Contains(gatherSubset, "all") {
		idx := slices.Index(gatherSubset, "all")
		gatherSubset = slices.Delete(gatherSubset, idx, idx+1)
		set["interfaces"] = struct{}{}
		set["vlans"] = struct{}{}
		set["controller_images"] = struct{}{}
		set["partition_images"] = struct{}{}
		set["tenant_images"] = struct{}{}
	}

	for _, item := range gatherSubset {
		if !strings.HasPrefix(item, "!") {
			set[item] = struct{}{}
		}
	}

	for _, item := range gatherSubset {
		if strings.HasPrefix(item, "!") {
			delete(set, item[1:])
		}
	}

	for key := range set {
		include = append(include, key)
	}

	return include
}

func convertInterfacesInfo(interfacesResp f5ossdk.F5RespOpenconfigInterface) []InterfacesInfo {
	var interfaces []InterfacesInfo

	for _, intf := range interfacesResp.OpenconfigInterfacesInterface {

		l3 := map[string]attr.Value{
			"in_octets":          types.StringValue(intf.State.Counters.InOctets),
			"in_unicast_pkts":    types.StringValue(intf.State.Counters.InUnicastPkts),
			"in_broadcast_pkts":  types.StringValue(intf.State.Counters.InBroadcastPkts),
			"in_multicast_pkts":  types.StringValue(intf.State.Counters.InMulticastPkts),
			"in_discards":        types.StringValue(intf.State.Counters.InDiscards),
			"in_errors":          types.StringValue(intf.State.Counters.InErrors),
			"in_fcs_errors":      types.StringValue(intf.State.Counters.InFcsErrors),
			"out_octets":         types.StringValue(intf.State.Counters.OutOctets),
			"out_unicast_pkts":   types.StringValue(intf.State.Counters.OutUnicastPkts),
			"out_broadcast_pkts": types.StringValue(intf.State.Counters.OutBroadcastPkts),
			"out_multicast_pkts": types.StringValue(intf.State.Counters.OutMulticastPkts),
			"out_discards":       types.StringValue(intf.State.Counters.OutDiscards),
			"out_errors":         types.StringValue(intf.State.Counters.OutErrors),
		}

		l3_val, _ := types.MapValue(types.StringType, l3)

		interfaces = append(interfaces, InterfacesInfo{
			Name:              types.StringValue(intf.Name),
			Type:              types.StringValue(intf.Config.Type),
			Enabled:           types.BoolValue(intf.Config.Enabled),
			OperationalStatus: types.StringValue(intf.State.OperStatus),
			Mtu:               types.Int64Value(int64(intf.State.Mtu)),
			PortSpeed:         types.StringValue(intf.OpenconfigIfEthernetEthernet.Config.PortSpeed),
			L3Counters:        l3_val,
		})
	}

	return interfaces
}

func convertVlansInfo(vlansResp f5ossdk.F5RespVlan) []VlansInfo {
	var vlans []VlansInfo

	for _, vlan := range vlansResp.OpenconfigVlanVlan {
		vlans = append(vlans, VlansInfo{
			VlanId:   types.Int64Value(int64(vlan.Config.VlanID)),
			VlanName: types.StringValue(vlan.Config.Name),
		})
	}

	return vlans
}

func convertTenantImagesInfo(tenantImagesResp f5ossdk.F5TenantImagesInfo) []TenantsImageInfo {
	var tenantImages []TenantsImageInfo

	for _, tenantImage := range tenantImagesResp.Images {
		tenantImages = append(tenantImages, TenantsImageInfo{
			ImageName: types.StringValue(tenantImage.Name),
			InUse:     types.BoolValue(tenantImage.InUse),
			Type:      types.StringValue(tenantImage.Type),
			Status:    types.StringValue(tenantImage.Status),
			Date:      types.StringValue(tenantImage.Date),
			Size:      types.StringValue(tenantImage.Size),
		})
	}

	return tenantImages
}

func convertIsoImagesInfo(isoImagesResp f5ossdk.F5IsoImagesInfo) []IsoImagesInfo {
	var isoImagesInfo []IsoImagesInfo

	for _, isoImage := range isoImagesResp.Images {
		isoImagesInfo = append(isoImagesInfo, IsoImagesInfo{
			Version: types.StringValue(isoImage.Version),
			Service: types.StringValue(isoImage.Service),
			Os:      types.StringValue(isoImage.Os),
		})
	}

	return isoImagesInfo
}
