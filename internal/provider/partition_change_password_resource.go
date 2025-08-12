package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &PartitionChangePasswordResource{}

func NewPartitionChangePasswordResource() resource.Resource {
	return &PartitionChangePasswordResource{}
}

// PartitionChangePasswordResource defines the resource implementation.
type PartitionChangePasswordResource struct {
	client *f5ossdk.F5os
}

type PartitionChangePasswordResourceModel struct {
	UserName    types.String `tfsdk:"user_name"`
	OldPassword types.String `tfsdk:"old_password"`
	NewPassword types.String `tfsdk:"new_password"`
	Id          types.String `tfsdk:"id"`
}

func (r *PartitionChangePasswordResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_partition_change_password"
}

func (r *PartitionChangePasswordResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Resource used to manage password of a specific user on a velos chassis partition.",

		Attributes: map[string]schema.Attribute{
			"user_name": schema.StringAttribute{
				MarkdownDescription: "Name of the chassis partition user account.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"old_password": schema.StringAttribute{
				MarkdownDescription: "Current password for the specified user account.",
				Required:            true,
				Sensitive:           true,
			},
			"new_password": schema.StringAttribute{
				MarkdownDescription: "New password for the specified user account.",
				Required:            true,
				Sensitive:           true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for resource.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *PartitionChangePasswordResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (r *PartitionChangePasswordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *PartitionChangePasswordResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if r.client.PlatformType != "Velos Partition" {
		resp.Diagnostics.AddError("Client Error", "`f5os_partition_change_password` resource is supported with Velos Partition level.")
		return
	}

	passwordChangeConfig := getPartitionPasswordChangeConfig(data)
	tflog.Info(ctx, fmt.Sprintf("passwordChangeConfig Data:%+v", passwordChangeConfig))

	respByte, err := r.client.PartitionPasswordChange(data.UserName.ValueString(), passwordChangeConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Partition Password change failed, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("passwordChangeConfig Response:%+v", string(respByte)))

	data.Id = types.StringValue(data.UserName.ValueString())

	// r.partitionResourceModelToState(ctx, partData, data)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

func (r *PartitionChangePasswordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *PartitionChangePasswordResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PartitionChangePasswordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *PartitionChangePasswordResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	passwordChangeConfig := getPartitionPasswordChangeConfig(data)
	tflog.Info(ctx, fmt.Sprintf("passwordChangeConfig Data:%+v", passwordChangeConfig))

	respByte, err := r.client.PartitionPasswordChange(data.UserName.ValueString(), passwordChangeConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Partition Password change failed, got error: %s", err))
		return
	}
	tflog.Info(ctx, fmt.Sprintf("passwordChangeConfig Response:%+v", string(respByte)))

	data.Id = types.StringValue(data.UserName.ValueString())

	// r.partitionResourceModelToState(ctx, partData, data)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PartitionChangePasswordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *PartitionChangePasswordResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
}

func getPartitionPasswordChangeConfig(data *PartitionChangePasswordResourceModel) *f5ossdk.F5ReqPartitionPassChange {
	passwordObj := f5ossdk.F5ReqPartitionPassChange{}
	passwordObj.OldPassword = data.OldPassword.ValueString()
	passwordObj.NewPassword = data.NewPassword.ValueString()
	passwordObj.ConfirmPassword = data.NewPassword.ValueString()
	return &passwordObj
}
