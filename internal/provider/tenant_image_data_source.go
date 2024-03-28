package provider

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// Ensure provider defined types fully satisfy framework interfaces
var (
	_ datasource.DataSource = &ImageInfoDataSource{}
)

func NewImageInfoDataSource() datasource.DataSource {
	return &ImageInfoDataSource{}
}

// ImageInfoDataSource defines the data source implementation.
type ImageInfoDataSource struct {
	client   *f5ossdk.F5os
	teemData *TeemData
}

// ImageInfoDataSourceModel describes the data source data model.
type ImageInfoDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	ImageName   types.String `tfsdk:"image_name"`
	ImageStatus types.String `tfsdk:"image_status"`
}

func (d *ImageInfoDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenant_image"
	teemData := &TeemData{}
	teemData.ProviderName = req.ProviderTypeName
	teemData.ResourceName = resp.TypeName
	d.teemData = teemData
}

func (d *ImageInfoDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Get information about the tenant Image on f5os platform.\n\n" +
			"Use this data source to get information, whether image available on platform or not",

		Attributes: map[string]schema.Attribute{
			"image_name": schema.StringAttribute{
				MarkdownDescription: "Name of the tenant image to check",
				Required:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier of this data source",
			},
			"image_status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Status of Image on the F5OS Platforms",
			},
		},
	}
}

func (d *ImageInfoDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (d *ImageInfoDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ImageInfoDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	var availableFlag = true
	timeBefore := time.Now().Add(6 * time.Minute)
	for time.Now().Before(timeBefore) {
		availableFlag = false
		imageObj, err := d.client.GetImage(data.ImageName.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to Get Image Details", fmt.Sprintf("Error:%s", err))
			return
		}
		for _, val := range imageObj.TenantImages {
			log.Printf("[DEBUG] Image Status: %+v", val.Status)
			if val.Name == data.ImageName.ValueString() {
				if val.Status == "not-present" || val.Status == "processing" {
					availableFlag = false
				}
				if val.Status == "replicated" || val.Status == "processed" {
					diff := time.Until(timeBefore)
					if !(diff > 2*time.Minute) {
						time.Sleep(2 * time.Minute)
					}
					availableFlag = true
					data.ImageName = types.StringValue(val.Name)
					data.ImageStatus = types.StringValue(val.Status)
					break
				}
			}
		}
		if availableFlag {
			break
		}
		time.Sleep(2 * time.Minute)
	}
	if !availableFlag {
		resp.Diagnostics.AddError("Unable to Get Image Details", fmt.Sprintf("Get Image: %s failed with error:%s", data.ImageName.ValueString(), "not-present"))
		return
	}

	data.ID = types.StringValue(data.ImageName.ValueString())
	teemData.ResourceName = "f5os_tenant_image"
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
