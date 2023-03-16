package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// Ensure provider defined types fully satisfy framework interfaces
var (
	_ datasource.DataSource = &SoftwareInfoDataSource{}
	// _ datasource.DataSourceWithConfigure = &SoftwareInfoDataSource{}
)

func NewSoftwareInfoDataSource() datasource.DataSource {
	return &SoftwareInfoDataSource{}
}

// SoftwareInfoDataSource defines the data source implementation.
type SoftwareInfoDataSource struct {
	client *f5ossdk.F5os
}

// SoftwareInfoDataSourceModel describes the data source data model.
type SoftwareInfoDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	SoftwareInfo types.String `tfsdk:"software_info"`
}

func (d *SoftwareInfoDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_softwareinfo"
	tflog.Info(ctx, resp.TypeName)
}

func (d *SoftwareInfoDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Get information about the VLANs on f5os platform.\n\n" +
			"Use this data source to get information, such as vlan",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier of this data source: hashing of the certificates in the chain.",
			},
			"software_info": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier of this data source: hashing of the certificates in the chain.",
			},
		},
	}
}

func (d *SoftwareInfoDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (d *SoftwareInfoDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SoftwareInfoDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	systemData, err := d.client.GetSoftwareComponentVersions()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read Vlan, got error: %s", err))
		return
	}

	// For the purposes of this example code, hardcoding a response value to
	// save into the Terraform state.
	// data.VlanID = types.Int64Value(int64(vlanIDs.OpenconfigVlanVlan[0].VlanID))
	tflog.Info(ctx, fmt.Sprintf("f5os client :%+v", d.client))
	tflog.Info(ctx, fmt.Sprintf("software info:%+v", string(systemData)))
	data.ID = types.StringValue(hashForState(string(systemData)))
	data.SoftwareInfo = types.StringValue(string(systemData))

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "read a data source")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
