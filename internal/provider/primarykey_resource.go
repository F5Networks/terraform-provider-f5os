package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

var _ resource.Resource = &PrimaryKeyResource{}
var _ resource.ResourceWithImportState = &PrimaryKeyResource{}

func NewPrimaryKeyResource() resource.Resource {
	return &PrimaryKeyResource{}
}

type PrimaryKeyResource struct {
	client   *f5ossdk.F5os
	teemData *TeemData
}

type PrimaryKeyResourceModel struct {
	Id          types.String `tfsdk:"id"`
	Passphrase  types.String `tfsdk:"passphrase"`
	Salt        types.String `tfsdk:"salt"`
	Status      types.String `tfsdk:"status"`
	Hash        types.String `tfsdk:"hash"`
	ForceUpdate types.Bool   `tfsdk:"force_update"`
}

func (r *PrimaryKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_primarykey"
}

func (r *PrimaryKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage system primary-key using passphrase and salt on F5OS devices.",

		Attributes: map[string]schema.Attribute{
			"force_update": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Force update the primary key on F5OS device.",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Status of primary key operation (e.g., COMPLETE)",
			},
			"hash": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Hash of the primary key as returned by the system.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform resource ID for primary key. Constant for now.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"passphrase": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Specifies passphrase for generating primary key.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(), // Optional: forces recreation on change
				},
			},

			"salt": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Specifies salt for generating primary key.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(), // Optional: forces recreation on change
				},
			},
		},
	}
}

func (r *PrimaryKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Extract and validate the F5OS client from the provider data
	client, diagnostics := toF5osProvider(req.ProviderData)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.client = client

	// Set up telemetry metadata (optional: can be removed if not needed)
	teemData := &TeemData{
		ProviderName: "f5os",
		ResourceName: "f5os_primarykey",
	}
	r.teemData = teemData
}

// primaryKeyPollInterval is the delay between status-check attempts.
// Overridden in tests to avoid real wall-clock waits.
var primaryKeyPollInterval = 2 * time.Second

// primaryKeyMigrationTimeout is the maximum time to wait for COMPLETE status.
// Overridden in tests to keep suite fast.
var primaryKeyMigrationTimeout = 60 * time.Second

// primaryKeyInitialReadDelay gives the device time to settle after SetPrimaryKey
// before the polling loop begins. Overridden in tests to avoid wall-clock sleeps.
var primaryKeyInitialReadDelay = 5 * time.Second

// waitForPrimaryKeyMigration polls GetPrimaryKey until status is COMPLETE or
// primaryKeyMigrationTimeout elapses.
func (r *PrimaryKeyResource) waitForPrimaryKeyMigration(ctx context.Context) error {
	maxPolls := int(primaryKeyMigrationTimeout / primaryKeyPollInterval)
	if maxPolls < 1 {
		maxPolls = 1
	}

	tflog.Info(ctx, "Waiting for primary key migration to complete")

	if primaryKeyInitialReadDelay > 0 {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for primary key migration")
		case <-time.After(primaryKeyInitialReadDelay):
		}
	}

	for poll := 0; poll < maxPolls; poll++ {
		keyData, err := r.client.GetPrimaryKey()
		switch {
		case err != nil:
			tflog.Warn(ctx, fmt.Sprintf("Error polling primary key status: %s", err))
		case keyData == nil:
			tflog.Warn(ctx, "Error polling primary key status: empty response from device")
		default:
			status := keyData.PrimaryKey.State.Status
			tflog.Debug(ctx, fmt.Sprintf("Primary key migration status: %s", status))
			if strings.Contains(status, "COMPLETE") {
				tflog.Info(ctx, "Primary key migration completed successfully")
				return nil
			}
		}

		// Sleep before the next poll (skip sleep on the last iteration)
		if poll < maxPolls-1 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled while waiting for primary key migration")
			case <-time.After(primaryKeyPollInterval):
			}
		}
	}

	return fmt.Errorf("primary key migration did not complete within %s", primaryKeyMigrationTimeout)
}

func (r *PrimaryKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *PrimaryKeyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "[CREATE] Creating PrimaryKey resource")
	primaryKeyReq := getPrimaryKeyConfig(data)

	tflog.Debug(ctx, fmt.Sprintf("PrimaryKey Request Payload: %+v", primaryKeyReq))

	_ = r.client.SendTeem(map[string]any{"teemData": r.teemData})

	// Always set the key on Create. Create is only called for new or
	// recreated resources (passphrase/salt have RequiresReplace), so
	// SetPrimaryKey must always run to apply the configured credentials.
	_, err := r.client.SetPrimaryKey(primaryKeyReq)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Failed to create PrimaryKey: %s", err))
		return
	}

	// Wait for async migration to complete before reading state
	if err := r.waitForPrimaryKeyMigration(ctx); err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Primary key migration failed: %s", err))
		return
	}

	// Now get the state from the device and update the Terraform state
	keyData, err := r.client.GetPrimaryKey()
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Failed to fetch state after setting PrimaryKey: %s", err))
		return
	}
	if keyData == nil {
		resp.Diagnostics.AddError("F5OS Client Error", "Failed to fetch state after setting PrimaryKey: empty response from device")
		return
	}

	r.primaryKeyResourceModelToState(keyData, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PrimaryKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *PrimaryKeyResourceModel

	// Load the current state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "[READ] Reading F5OS Primary Key Configuration")

	// Fetch the primary key state from the device
	keyData, err := r.client.GetPrimaryKey()
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Failed to fetch Primary Key configuration: %s", err))
		return
	}
	if keyData == nil {
		resp.Diagnostics.AddError("F5OS Client Error", "Failed to fetch Primary Key configuration: empty response from device")
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("PrimaryKey Response: %+v", keyData))

	// Update the state model with fetched data
	r.primaryKeyResourceModelToState(keyData, data)

	// Save back into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PrimaryKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *PrimaryKeyResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "[UPDATE] Updating Primary Key configuration")

	if data.ForceUpdate.ValueBool() {
		// Only re-key when force_update=true. Changing force_update alone
		// (false → true) is the user's explicit signal to rotate the key.
		keyReqConfig := getPrimaryKeyConfig(data)
		tflog.Debug(ctx, fmt.Sprintf("PrimaryKey Update Payload: %+v", keyReqConfig))

		_, err := r.client.SetPrimaryKey(keyReqConfig)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Failed to update Primary Key: %s", err))
			return
		}

		if err := r.waitForPrimaryKeyMigration(ctx); err != nil {
			resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Primary key migration failed: %s", err))
			return
		}
	} else {
		tflog.Info(ctx, "[UPDATE] force_update=false — skipping SetPrimaryKey, refreshing state only")
	}

	// Fetch the latest status after update
	keyData, err := r.client.GetPrimaryKey()
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Failed to retrieve Primary Key after update: %s", err))
		return
	}
	if keyData == nil {
		resp.Diagnostics.AddError("F5OS Client Error", "Failed to retrieve Primary Key after update: empty response from device")
		return
	}

	// Map response to Terraform model
	r.primaryKeyResourceModelToState(keyData, data)

	// Save new state to Terraform
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PrimaryKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *PrimaryKeyResourceModel

	// Load the state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "[DELETE] Attempting to delete F5OS Primary Key (noop)")

	// According to Ansible module, primary key deletion is not supported — return gracefully
	resp.Diagnostics.AddWarning(
		"Unsupported Operation",
		"The primary key cannot be deleted on F5OS devices. This operation will be a no-op.",
	)

	// Optionally, you could still attempt a reset endpoint if one exists in F5OS APIs
}

func (r *PrimaryKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func getPrimaryKeyConfig(data *PrimaryKeyResourceModel) *f5ossdk.F5ReqPrimaryKey {
	passphrase := data.Passphrase.ValueString()
	salt := data.Salt.ValueString()

	return &f5ossdk.F5ReqPrimaryKey{
		PrimaryKey: f5ossdk.PrimaryKeyConfig{
			Passphrase:        passphrase,
			ConfirmPassphrase: passphrase,
			Salt:              salt,
			ConfirmSalt:       salt,
		},
	}
}

func (r *PrimaryKeyResource) primaryKeyResourceModelToState(respData *f5ossdk.F5RespPrimaryKey, data *PrimaryKeyResourceModel) {
	if respData.PrimaryKey.State.Status != "" {
		data.Status = types.StringValue(respData.PrimaryKey.State.Status)
	} else {
		data.Status = types.StringNull()
	}

	if respData.PrimaryKey.State.Hash != "" {
		data.Hash = types.StringValue(respData.PrimaryKey.State.Hash)
	} else {
		data.Hash = types.StringNull()
	}

	data.Id = types.StringValue("primary-key")
}
