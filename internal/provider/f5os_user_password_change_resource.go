package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &UserPasswordChangeResource{}

func NewUserPasswordChangeResource() resource.Resource {
	return &UserPasswordChangeResource{}
}

// UserPasswordChangeResource defines the resource implementation.
type UserPasswordChangeResource struct {
	client *f5ossdk.F5os
}

type UserPasswordChangeResourceModel struct {
	UserName    types.String `tfsdk:"user_name"`
	OldPassword types.String `tfsdk:"old_password"`
	NewPassword types.String `tfsdk:"new_password"`
	Id          types.String `tfsdk:"id"`
}

func (r *UserPasswordChangeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_password_change"
}

func (r *UserPasswordChangeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Resource used to change passwords for F5OS user accounts. Supports updating passwords for default users (admin, root) as well as other accounts. This resource is not idempotent and should be used carefully.",

		Attributes: map[string]schema.Attribute{
			"user_name": schema.StringAttribute{
				MarkdownDescription: "Name of the F5OS user account for which to change the password.",
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
				MarkdownDescription: "New password for the specified user account. Must meet F5OS device password policy requirements.",
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

func (r *UserPasswordChangeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (r *UserPasswordChangeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *UserPasswordChangeResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that old and new passwords are different
	if data.OldPassword.ValueString() == data.NewPassword.ValueString() {
		resp.Diagnostics.AddError("Validation Error", "Old and new password cannot be the same.")
		return
	}

	userName := data.UserName.ValueString()
	oldPassword := data.OldPassword.ValueString()
	newPassword := data.NewPassword.ValueString()

	tflog.Info(ctx, fmt.Sprintf("Changing password for user: %s", userName))

	// Execute password change
	err := r.changeUserPassword(ctx, userName, oldPassword, newPassword)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("User password change failed, got error: %s", err))
		return
	}

	data.Id = types.StringValue(userName)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserPasswordChangeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *UserPasswordChangeResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Password change is not idempotent, so we just preserve the state
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserPasswordChangeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *UserPasswordChangeResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that old and new passwords are different
	if data.OldPassword.ValueString() == data.NewPassword.ValueString() {
		resp.Diagnostics.AddError("Validation Error", "Old and new password cannot be the same.")
		return
	}

	userName := data.UserName.ValueString()
	oldPassword := data.OldPassword.ValueString()
	newPassword := data.NewPassword.ValueString()

	tflog.Info(ctx, fmt.Sprintf("Updating password for user: %s", userName))

	// Execute password change
	err := r.changeUserPassword(ctx, userName, oldPassword, newPassword)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("User password change failed, got error: %s", err))
		return
	}

	data.Id = types.StringValue(userName)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserPasswordChangeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *UserPasswordChangeResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Password change is not reversible, so we just remove from state
	tflog.Info(ctx, "Password change resource deleted from state - password change is not reversible")
}

// changeUserPassword handles the password change logic based on user type and authentication context
func (r *UserPasswordChangeResource) changeUserPassword(ctx context.Context, userName, oldPassword, newPassword string) error {
	// Based on the Ansible module logic:
	// 1. For admin user - use change-password endpoint
	// 2. For authenticated user changing their own password - use change-password endpoint
	// 3. For admin changing other user's password - use set-password endpoint

	if userName == "admin" {
		return r.changePasswordWithOldPassword(ctx, userName, oldPassword, newPassword)
	}

	// For simplicity, we'll use the change-password endpoint for all cases
	// In a production environment, you might want to detect the authentication context
	// and use set-password for admin changing other users' passwords
	return r.changePasswordWithOldPassword(ctx, userName, oldPassword, newPassword)
}

// changePasswordWithOldPassword uses the SDK's PartitionPasswordChange method
func (r *UserPasswordChangeResource) changePasswordWithOldPassword(ctx context.Context, userName, oldPassword, newPassword string) error {
	tflog.Debug(ctx, "Changing password with old password using SDK", map[string]any{
		"username": userName,
	})

	// Use the F5OS SDK's PartitionPasswordChange method
	passwordChangeConfig := &f5ossdk.F5ReqPartitionPassChange{
		OldPassword:     oldPassword,
		NewPassword:     newPassword,
		ConfirmPassword: newPassword, // API requires confirmation password
	}

	_, err := r.client.PartitionPasswordChange(userName, passwordChangeConfig)
	if err != nil {
		errStr := err.Error()
		tflog.Debug(ctx, "Password Change API Error", map[string]any{
			"error": errStr,
		})

		// Check for common password policy violations
		if strings.Contains(errStr, "password") &&
			(strings.Contains(errStr, "length") ||
				strings.Contains(errStr, "character") ||
				strings.Contains(errStr, "dictionary check") ||
				strings.Contains(errStr, "simplistic") ||
				strings.Contains(errStr, "systematic")) {
			return fmt.Errorf("password does not meet F5OS device policy requirements: %w", err)
		}

		// Check for old password incorrect
		if strings.Contains(errStr, "incorrect") || strings.Contains(errStr, "invalid") || strings.Contains(errStr, "authentication") {
			return fmt.Errorf("old password is incorrect: %w", err)
		}

		return fmt.Errorf("API request failed: %w", err)
	}

	tflog.Debug(ctx, "Password change completed successfully", map[string]any{
		"username": userName,
	})

	return nil
}
