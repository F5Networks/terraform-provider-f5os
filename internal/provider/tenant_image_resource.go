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
	Protocol       types.String `tfsdk:"protocol"`
	RemoteHost     types.String `tfsdk:"remote_host"`
	RemoteUser     types.String `tfsdk:"remote_user"`
	RemotePassword types.String `tfsdk:"remote_password"`
	RemotePath     types.String `tfsdk:"remote_path"`
	RemotePort     types.Int64  `tfsdk:"remote_port"`
	Timeout        types.Int64  `tfsdk:"timeout"`
	Id             types.String `tfsdk:"id"`
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
				MarkdownDescription: "The path on the F5OS where the the tenant image is to be uploaded.",
				Optional:            true,
				//Validators:          []string{"images/import", ""},
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
				PlanModifiers:       []planmodifier.Int64{attribute_plan_modifier.Int64DefaultValue(types.Int64Value(360))},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Example identifier",
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
	tflog.Info(ctx, fmt.Sprintf("Create data :%+v", data))
	timeout := int(data.Timeout.ValueInt64())
	tflog.Info(ctx, fmt.Sprintf("timeout data :%+v", timeout))
	importConfig := &f5ossdk.F5TenantImage{}
	importConfig.Insecure = ""
	importConfig.RemoteHost = data.RemoteHost.ValueString()
	importConfig.RemoteFile = fmt.Sprintf("%s/%s", data.RemotePath.ValueString(), data.ImageName.ValueString())
	importConfig.LocalFile = data.LocalPath.ValueString()
	respByte, err := r.client.ImportImage(importConfig, timeout)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to Import Image, got error: %s", err))
		return
	}
	if string(respByte) != "Import Image Transfer Success" {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Import Image failed"))
		return
	}

	// For the purposes of this example code, hardcoding a response value to
	// save into the Terraform state.
	data.Id = types.StringValue(data.ImageName.ValueString())

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	//tflog.Trace(ctx, "created a resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
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
	respByte, err := r.client.GetImage(data.ImageName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to Import Image, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("respByte :%+v", respByte))

	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read example, got error: %s", err))
	//     return
	// }

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

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update example, got error: %s", err))
	//     return
	// }

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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to Import Image, got error: %s", err))
		return
	}
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete example, got error: %s", err))
	//     return
	// }
}

func (r *TenantImageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

//
//func Int64DefaultValue(v types.Int64) planmodifier.Int64 {
//	return &int64DefaultValuePlanModifier{v}
//}
//
//type int64DefaultValuePlanModifier struct {
//	DefaultValue types.Int64
//}
//
//var _ planmodifier.Int64 = (*int64DefaultValuePlanModifier)(nil)
//
//func (apm *int64DefaultValuePlanModifier) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, res *planmodifier.Int64Response) {
//	// If the attribute configuration is not null, we are done here
//	if !req.ConfigValue.IsNull() {
//		return
//	}
//
//	// If the attribute plan is "known" and "not null", then a previous plan modifier in the sequence
//	// has already been applied, and we don't want to interfere.
//	if !req.PlanValue.IsUnknown() && !req.PlanValue.IsNull() {
//		return
//	}
//
//	res.PlanValue = apm.DefaultValue
//}
