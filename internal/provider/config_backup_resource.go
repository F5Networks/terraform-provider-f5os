package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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

var _ resource.Resource = &CfgBackupResource{}

// var _ resource.ResourceWithImportState = &CfgBackupResource{}

func NewCfgBackupResource() resource.Resource {
	return &CfgBackupResource{}
}

type CfgBackupResource struct {
	client *f5ossdk.F5os
}

type CfgBackupResourceModel struct {
	Name           types.String `tfsdk:"name"`
	RemoteHost     types.String `tfsdk:"remote_host"`
	RemoteUser     types.String `tfsdk:"remote_user"`
	RemotePassword types.String `tfsdk:"remote_password"`
	RemotePath     types.String `tfsdk:"remote_path"`
	Protocol       types.String `tfsdk:"protocol"`
	Timeout        types.Int64  `tfsdk:"timeout"`
	Id             types.String `tfsdk:"id"`
}

func (r *CfgBackupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_config_backup"
}

func (r *CfgBackupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Resource used to manage F5OS config backup",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the config backup file.",
				Required:            true,
			},
			"remote_host": schema.StringAttribute{
				MarkdownDescription: "The hostname or IP address of the remote server used for storing the config backup file.",
				Required:            true,
			},
			"remote_user": schema.StringAttribute{
				MarkdownDescription: "User name for the remote server used for exporting the created config backup file.",
				Required:            true,
			},
			"remote_password": schema.StringAttribute{
				MarkdownDescription: "User password for the remote server used for exporting the created config backup file.",
				Sensitive:           true,
				Required:            true,
			},
			"remote_path": schema.StringAttribute{
				MarkdownDescription: "The path on the remote server used for uploading the created config backup file.",
				Required:            true,
			},
			"protocol": schema.StringAttribute{
				MarkdownDescription: "Protocol for config backup file transfer.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("scp", "https", "sftp"),
				},
			},
			"timeout": schema.Int64Attribute{
				MarkdownDescription: "The number of seconds to wait for config backup file export to finish. The value must be between 150 and 3600",
				Optional:            true,
				Default:             int64default.StaticInt64(150),
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.Between(150, 3600),
				},
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

func (r *CfgBackupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (r *CfgBackupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *CfgBackupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		tflog.Error(ctx, "unexpected failure occurred when converting plan data to backup model")
		return
	}

	name := data.Name.ValueString()
	exportConfig := backupModelToExportConfig(data)
	timeout := data.Timeout.ValueInt64()
	_, err := r.client.CreateConfigBackup(name, timeout, exportConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating config backup, got error: %s", err))
		return
	}

	data.Id = data.Name
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CfgBackupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *CfgBackupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, fmt.Sprintf("[READ] Reading Config Backups :%+v", data.Id.ValueString()))
	name := data.Name.ValueString()

	res, err := r.client.GetConfigBackup()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Config Backups",
			"unexpected error occurred while trying to get the list of config backup files: "+err.Error(),
		)
		return
	}

	// obj := make(map[string]map[string][]map[string]string)
	obj := make(map[string]any)
	err = json.NewDecoder(bytes.NewReader(res)).Decode(&obj)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing Config Backup Response",
			"unexpected error occurred while trying to get the list of config backup files: "+err.Error(),
		)
		return
	}

	entries := obj["f5-utils-file-transfer:output"].(map[string]any)["entries"].([]any)
	exists := false
	for _, v := range entries {
		m := v.(map[string]any)
		if m["name"].(string) == name {
			exists = true
			break
		}
	}

	if !exists {
		req.State.RemoveResource(ctx)
	}
}

func (r *CfgBackupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *CfgBackupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *CfgBackupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *CfgBackupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	fileName := fmt.Sprintf("configs/%s", data.Name.ValueString())
	err := r.client.DeleteConfigBackup(fileName)

	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while destroying config backup, got error: %s", err))
		return
	}
}

func backupModelToExportConfig(model *CfgBackupResourceModel) f5ossdk.FileExport {
	exportConfig := f5ossdk.FileExport{}
	exportConfig.Insecure = ""
	exportConfig.RemoteHost = model.RemoteHost.ValueString()
	exportConfig.RemotePath = model.RemotePath.ValueString()
	exportConfig.LocalFile = fmt.Sprintf("configs/%s", model.Name.ValueString())
	exportConfig.Protocol = model.Protocol.ValueString()
	exportConfig.Username = model.RemoteUser.ValueString()
	exportConfig.Password = model.RemotePassword.ValueString()

	return exportConfig
}
