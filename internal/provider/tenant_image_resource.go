package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	go_path "path"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TenantImageResource{}
var _ resource.ResourceWithImportState = &TenantImageResource{}

func NewTenantImageResource() resource.Resource {
	return &TenantImageResource{}
}

// TenantImageResource defines the resource implementation.
type TenantImageResource struct {
	client *f5ossdk.F5os
}

// TenantImageResourceModel describes the resource data model.
type TenantImageResourceModel struct {
	ImageName      types.String `tfsdk:"image_name"`
	LocalPath      types.String `tfsdk:"local_path"`
	UploadFromPath types.String `tfsdk:"upload_from_path"`
	Protocol       types.String `tfsdk:"protocol"`
	RemoteHost     types.String `tfsdk:"remote_host"`
	RemoteUser     types.String `tfsdk:"remote_user"`
	RemotePassword types.String `tfsdk:"remote_password"`
	RemotePath     types.String `tfsdk:"remote_path"`
	RemotePort     types.Int64  `tfsdk:"remote_port"`
	Timeout        types.Int64  `tfsdk:"timeout"`
	Id             types.String `tfsdk:"id"`
	Status         types.String `tfsdk:"status"`
}

func (r *TenantImageResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenant_image"
}

func (r *TenantImageResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Resource used for Manage F5OS tenant images",

		Attributes: map[string]schema.Attribute{
			"image_name": schema.StringAttribute{
				MarkdownDescription: "Name of the tenant image.",
				Required:            true,
			},
			"local_path": schema.StringAttribute{
				MarkdownDescription: "The path on the F5OS where the the tenant image is to be imported to.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"images/tenant", "images", "images/staging", "images/import/iso"}...),
				},
			},
			"upload_from_path": schema.StringAttribute{
				MarkdownDescription: "The path to image on the local machine which is to be uploaded",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("local_path")),
					stringvalidator.ConflictsWith(path.MatchRoot("remote_host")),
					stringvalidator.ConflictsWith(path.MatchRoot("remote_port")),
					stringvalidator.ConflictsWith(path.MatchRoot("remote_user")),
					stringvalidator.ConflictsWith(path.MatchRoot("remote_password")),
				},
			},
			"protocol": schema.StringAttribute{
				MarkdownDescription: "Protocol for image transfer.",
				Optional:            true,
			},
			"remote_host": schema.StringAttribute{
				MarkdownDescription: "The hostname or IP address of the remote server on which the tenant image is stored.\nThe server must make the image accessible via the specified protocol.",
				Optional:            true,
			},
			"remote_user": schema.StringAttribute{
				MarkdownDescription: "User name for the remote server on which the tenant image is stored.",
				Optional:            true,
			},
			"remote_password": schema.StringAttribute{
				MarkdownDescription: "Password for the user on the remote server on which the tenant image is stored.",
				Optional:            true,
				Sensitive:           true,
			},
			"remote_path": schema.StringAttribute{
				MarkdownDescription: "The path to the tenant image on the remote server.",
				Optional:            true,
			},
			"remote_port": schema.Int64Attribute{
				MarkdownDescription: "The port on the remote host to which you want to connect.\nIf the port is not provided, a default port for the selected protocol is used.",
				Optional:            true,
			},
			"timeout": schema.Int64Attribute{
				MarkdownDescription: "The number of seconds to wait for image import to finish.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(360),
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Example identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Status of Imported Image",
			},
		},
	}
}

func (r *TenantImageResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (r *TenantImageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *TenantImageResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	if r.client.PlatformType == "Velos Controller" {
		resp.Diagnostics.AddError("Client Error", "`f5os_tenant_image` resource is supported with Velos Partition level (or) rSeries appliance")
		return
	}

	resp1Byte, _ := r.client.GetImage(data.ImageName.ValueString())

	// if err != nil {
	// 	resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to Import Image, got error: %s", err))
	// 	return
	// }

	if resp1Byte == nil || len(resp1Byte.TenantImages) == 0 {
		if data.UploadFromPath.IsNull() {
			respByte, err := r.importImage(ctx, data)
			if err != nil {
				resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Unable to Import Image, got error: %s", err))
				return
			}
			if string(respByte) != "Import Image Transfer Success" {
				resp.Diagnostics.AddError("Client Error", "Import Image failed")
				return
			}
		} else {
			respByte, err := r.uploadImage(ctx, data)
			if err != nil {
				resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("unable to upload image, got error: %s", err))
				return
			}
			ret := make(map[string]string)
			err = json.NewDecoder(bytes.NewReader(respByte)).Decode(&ret)
			if err != nil {
				resp.Diagnostics.AddError("F5OS Client Error:", "could not parse the response from image upload endpoint")
			}
			result, ok := ret["result-tag"]
			if !ok {
				resp.Diagnostics.AddError("F5OS Client Error:", "unable to get image upload status")
				return
			}
			if result != "uploaded-successfully" {
				resp.Diagnostics.AddError("F5OS Client Error:", "image upload failed")
				return
			}
		}
	}

	// For the purposes of this example code, hardcoding a response value to
	// save into the Terraform state.
	respByte, err := r.client.GetImage(data.ImageName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to Read/Get Imported Image, got error: %s", err))
		return
	}
	if len(respByte.TenantImages) > 0 {
		r.tenantImageResourceModeltoState(ctx, respByte, data)
	} else {
		data.Id = types.StringValue("")
	}
	// Save data into Terraform state
	data.Id = types.StringValue(data.ImageName.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TenantImageResource) importImage(ctx context.Context, data *TenantImageResourceModel) ([]byte, error) {
	timeout := int(data.Timeout.ValueInt64())
	tflog.Info(ctx, fmt.Sprintf("timeout data :%+v", timeout))
	importConfig := &f5ossdk.F5ReqTenantImage{}
	importConfig.Insecure = ""
	importConfig.RemoteHost = data.RemoteHost.ValueString()
	importConfig.RemoteFile = fmt.Sprintf("%s/%s", data.RemotePath.ValueString(), data.ImageName.ValueString())
	importConfig.LocalFile = data.LocalPath.ValueString()
	tflog.Info(ctx, fmt.Sprintf("Create Data:%+v", importConfig))
	return r.client.ImportImage(importConfig, timeout)
}

func (r *TenantImageResource) uploadImage(ctx context.Context, data *TenantImageResourceModel) ([]byte, error) {
	timeout := int(data.Timeout.ValueInt64())
	tflog.Info(ctx, fmt.Sprintf("timeout data :%+v", timeout))
	imageDir := data.UploadFromPath.ValueString()
	imageName := data.ImageName.ValueString()
	filePath := go_path.Join(imageDir, imageName)
	tflog.Info(ctx, "Uploading image")
	r.client.ConfigOptions.APICallTimeout = time.Duration(time.Duration(timeout).Seconds())
	return r.client.UploadImage(filePath)
}

func (r *TenantImageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *TenantImageResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	respByte, err := r.client.GetImage(data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to Read/Get Imported Image, got error: %s", err))
		return
	}
	if len(respByte.TenantImages) > 0 {
		r.tenantImageResourceModeltoState(ctx, respByte, data)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TenantImageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *TenantImageResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	respByte, err := r.client.GetImage(data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to Read/Get Imported Image, got error: %s", err))
		return
	}
	if len(respByte.TenantImages) > 0 {
		r.tenantImageResourceModeltoState(ctx, respByte, data)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TenantImageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *TenantImageResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	err := r.client.DeleteTenantImage(data.ImageName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to Delete Imported Image, got error: %s", err))
		return
	}
}

func (r *TenantImageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *TenantImageResource) tenantImageResourceModeltoState(ctx context.Context, respData *f5ossdk.F5RespTenantImagesStatus, data *TenantImageResourceModel) {
	tflog.Info(ctx, fmt.Sprintf("respData :%+v", respData))
	data.ImageName = types.StringValue(respData.TenantImages[0].Name)
	data.Status = types.StringValue(respData.TenantImages[0].Status)
}
