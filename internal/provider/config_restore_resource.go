package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &CfgRestoreResource{}
var _ resource.ResourceWithValidateConfig = &CfgRestoreResource{}

func NewCfgRestoreResource() resource.Resource {
	return &CfgRestoreResource{}
}

// CfgRestoreResource complements CfgBackupResource (f5os_config_backup):
// f5os_config_backup exports a snapshot of the platform config database
// off the device; f5os_config_restore restores one back onto the device.
// Like f5os_user_password_change, this resource models a one-shot,
// non-idempotent device action rather than continuously-reconciled
// config -- see Read/Update/Delete below.
type CfgRestoreResource struct {
	client *f5ossdk.F5os
}

type CfgRestoreResourceModel struct {
	Name           types.String `tfsdk:"name"`
	RemoteHost     types.String `tfsdk:"remote_host"`
	RemoteUser     types.String `tfsdk:"remote_user"`
	RemotePassword types.String `tfsdk:"remote_password"`
	RemotePath     types.String `tfsdk:"remote_path"`
	RemotePort     types.Int64  `tfsdk:"remote_port"`
	Protocol       types.String `tfsdk:"protocol"`
	Insecure       types.Bool   `tfsdk:"insecure"`
	Timeout        types.Int64  `tfsdk:"timeout"`
	Result         types.String `tfsdk:"result"`
	Id             types.String `tfsdk:"id"`
}

func (r *CfgRestoreResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_config_restore"
}

func (r *CfgRestoreResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Resource used to restore the F5OS platform configuration database from a backup file, complementing `f5os_config_backup`. " +
			"The named backup must already exist under `configs/` on the device -- either created by a prior `f5os_config_backup` " +
			"(in this or an earlier apply), or fetched from a remote server first by setting `remote_host`/`remote_path` here, which " +
			"pulls the file down via F5OS's file-transfer import before invoking the restore. This resource performs a one-shot, " +
			"non-idempotent device action (not continuously-reconciled config): a `terraform apply` triggers a real restore every " +
			"time `name` (or any remote-fetch attribute) changes; re-applying an unchanged configuration does not re-trigger it. " +
			"`terraform destroy` only removes this resource from state -- restoring a device's configuration cannot be \"undone\" by " +
			"this provider, so Delete is a no-op on the device.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the config backup file to restore, matching the `name` used to create it (e.g. via `f5os_config_backup`). " +
					"Must already exist under `configs/` on the device unless `remote_host` is also set.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"remote_host": schema.StringAttribute{
				MarkdownDescription: "The hostname or IP address of the remote server from which to fetch the backup file named `name`, before restoring it. " +
					"Leave unset to restore a backup that already exists under `configs/` on the device (for example, one created by " +
					"`f5os_config_backup` in this or an earlier apply) without fetching anything first.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"remote_user": schema.StringAttribute{
				MarkdownDescription: "User name for the remote server referenced by `remote_host`. Requires `remote_host`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("remote_host")),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"remote_password": schema.StringAttribute{
				MarkdownDescription: "Password for `remote_user` on the remote server referenced by `remote_host`. Requires `remote_host`.",
				Optional:            true,
				Sensitive:           true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("remote_host")),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"remote_path": schema.StringAttribute{
				MarkdownDescription: "The path to the backup file on the remote server referenced by `remote_host`. Requires `remote_host`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("remote_host")),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"remote_port": schema.Int64Attribute{
				MarkdownDescription: "The port on `remote_host` to connect to. If not provided, a default port for the selected protocol is used. Requires `remote_host`.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.AlsoRequires(path.MatchRoot("remote_host")),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"protocol": schema.StringAttribute{
				MarkdownDescription: "Protocol used to fetch the backup file from `remote_host`. Requires `remote_host`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("scp", "https", "sftp"),
					stringvalidator.AlsoRequires(path.MatchRoot("remote_host")),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"insecure": schema.BoolAttribute{
				MarkdownDescription: "When set to `true`, skips TLS certificate verification on `remote_host` when fetching the backup file (for example, a server with a " +
					"self-signed certificate). Only meaningful with `remote_host`; has no effect otherwise. Requires `remote_host`.",
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"timeout": schema.Int64Attribute{
				MarkdownDescription: "The number of seconds to wait for the remote backup file fetch to finish, when `remote_host` is set. " +
					"The restore action itself is synchronous and is not bounded by this timeout. The value must be between 150 and 3600. " +
					"Changing this value re-triggers the restore action (see the resource-level description), consistent with every " +
					"other attribute on this one-shot-action resource.",
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(150),
				Validators: []validator.Int64{
					int64validator.Between(150, 3600),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"result": schema.StringAttribute{
				MarkdownDescription: "The raw result message returned by the device's `f5-database:config-restore` action.",
				Computed:            true,
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

func (r *CfgRestoreResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

// ValidateConfig enforces that insecure is only meaningful alongside
// remote_host. Every other remote-fetch attribute (remote_user,
// remote_password, remote_path, remote_port, protocol) declares this
// same constraint via stringvalidator/int64validator.AlsoRequires in the
// schema itself; boolvalidator.AlsoRequires is not vendored in this repo
// (no other resource currently needs it), so insecure's cross-field
// check is expressed here instead, following the same
// resource.ResourceWithValidateConfig pattern tenant_image_resource.go
// uses for its own cross-field constraints.
func (r *CfgRestoreResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data CfgRestoreResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	remoteHostSet := !data.RemoteHost.IsNull() && !data.RemoteHost.IsUnknown() && data.RemoteHost.ValueString() != ""
	insecureSet := !data.Insecure.IsNull() && !data.Insecure.IsUnknown() && data.Insecure.ValueBool()

	if insecureSet && !remoteHostSet {
		resp.Diagnostics.AddAttributeError(
			path.Root("insecure"),
			"Missing Attribute Configuration",
			"insecure requires remote_host to also be set: it only affects TLS certificate verification when fetching the backup file from a remote server.",
		)
	}
}

func (r *CfgRestoreResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *CfgRestoreResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "unexpected failure occurred when converting plan data to config restore model")
		return
	}

	name := data.Name.ValueString()
	importCfg := restoreModelToImportConfig(data)
	timeout := data.Timeout.ValueInt64()

	result, err := r.client.RestoreConfigBackup(name, timeout, importCfg)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while restoring config backup %q, got error: %s", name, err))
		return
	}

	data.Result = types.StringValue(result)
	data.Id = data.Name
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read is a no-op that preserves prior state: a config restore is a
// one-shot device action, not a resource with a persistent, independently
// re-readable representation on the device (unlike f5os_config_backup,
// whose backup *file* remains listed under configs/ and can be checked
// for on every Read). Mirrors f5os_user_password_change's Read for the
// same reason -- see that resource's comment.
func (r *CfgRestoreResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *CfgRestoreResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update re-runs the restore. Every attribute is RequiresReplace, so in
// practice Update is only reached when a change is limited to attributes
// Terraform itself does not consider plan-significant (this method exists
// primarily so the resource satisfies resource.Resource; RequiresReplace
// on every configurable attribute means Create, not Update, is the path
// exercised by a real attribute change).
func (r *CfgRestoreResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *CfgRestoreResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	importCfg := restoreModelToImportConfig(data)
	timeout := data.Timeout.ValueInt64()

	result, err := r.client.RestoreConfigBackup(name, timeout, importCfg)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while restoring config backup %q, got error: %s", name, err))
		return
	}

	data.Result = types.StringValue(result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete only removes the resource from Terraform state. Restoring a
// device's configuration cannot be "undone" -- there is no supported
// operation that reverts a config-restore, so unlike f5os_config_backup's
// Delete (which removes the backup file artifact from the device),
// there is nothing on the device for this resource's Delete to touch.
func (r *CfgRestoreResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "config restore resource deleted from state - restoring a device configuration is not reversible")
}

func restoreModelToImportConfig(model *CfgRestoreResourceModel) *f5ossdk.FileImport {
	if model.RemoteHost.IsNull() || model.RemoteHost.ValueString() == "" {
		return nil
	}

	importCfg := &f5ossdk.FileImport{}
	importCfg.RemoteHost = model.RemoteHost.ValueString()
	importCfg.RemoteFile = model.RemotePath.ValueString()
	if !model.Protocol.IsNull() && !model.Protocol.IsUnknown() {
		importCfg.Protocol = model.Protocol.ValueString()
	}
	if !model.RemoteUser.IsNull() && !model.RemoteUser.IsUnknown() {
		importCfg.Username = model.RemoteUser.ValueString()
	}
	if !model.RemotePassword.IsNull() && !model.RemotePassword.IsUnknown() {
		importCfg.Password = model.RemotePassword.ValueString()
	}
	if !model.RemotePort.IsNull() && !model.RemotePort.IsUnknown() {
		importCfg.RemotePort = int(model.RemotePort.ValueInt64())
	}
	if !model.Insecure.IsNull() && !model.Insecure.IsUnknown() && model.Insecure.ValueBool() {
		// The F5OS RESTCONF API models "insecure" as a YANG empty leaf.
		// Per RFC 7951 the JSON encoding of an empty leaf is [null].
		// When the field is nil it is omitted by omitempty (insecure
		// disabled) -- see tenant_image_resource.go's importImage for
		// the same pattern against the same file-transfer endpoint.
		importCfg.Insecure = []interface{}{nil}
	}

	return importCfg
}
