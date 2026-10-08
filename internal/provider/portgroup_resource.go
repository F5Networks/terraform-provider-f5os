package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

var _ resource.Resource = &PortGroupResource{}
var _ resource.ResourceWithImportState = &PortGroupResource{}

func NewPortGroupResource() resource.Resource { return &PortGroupResource{} }

// PortGroupResource manages the mode of a hardware-defined rSeries port group.
type PortGroupResource struct{ client *f5ossdk.F5os }

type PortGroupResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Mode             types.String `tfsdk:"mode"`
	DDMPollFrequency types.Int64  `tfsdk:"ddm_poll_frequency"`
}

func (r *PortGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_portgroup"
}

func (r *PortGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manage the mode of a hardware-defined port group on an F5OS rSeries appliance.", Attributes: map[string]schema.Attribute{
		"id":                 schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"name":               schema.StringAttribute{MarkdownDescription: "Hardware-defined port group name, for example `1/1`.", Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"mode":               schema.StringAttribute{MarkdownDescription: "Port group mode supported by the appliance.", Required: true, Validators: []validator.String{stringvalidator.OneOf("MODE_100GB", "MODE_4x25GB", "MODE_40GB", "MODE_4x10GB", "MODE_10GB", "MODE_25GB", "MODE_400GB", "MODE_4x100GB")}},
		"ddm_poll_frequency": schema.Int64Attribute{MarkdownDescription: "Digital diagnostic monitoring polling frequency.", Optional: true, Computed: true, Validators: []validator.Int64{int64validator.AtLeast(0)}},
	}}
}

func (r *PortGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (r *PortGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PortGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client.PlatformType == "Velos Controller" {
		resp.Diagnostics.AddError("Client Error", "`f5os_portgroup` resource is supported only on rSeries appliances.")
		return
	}
	if _, err := r.client.GetPortGroup(data.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to find port group %q: %s", data.Name.ValueString(), err))
		return
	}
	r.updateAndRead(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := waitForPortGroupReboot(ctx, r.client); err != nil {
		resp.Diagnostics.AddWarning("Port group configuration may still be applying", err.Error())
	}
	data.ID = data.Name
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PortGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PortGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	portGroup, err := r.client.GetPortGroup(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to read port group %q: %s", data.Name.ValueString(), err))
		return
	}
	data.Mode = types.StringValue(portGroup.Mode)
	r.setDDMPollFrequency(&data, portGroup)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PortGroupResource) setDDMPollFrequency(data *PortGroupResourceModel, portGroup *f5ossdk.PortGroupConfig) {
	if portGroup.DDM == nil || portGroup.DDM.PollFrequency == nil {
		data.DDMPollFrequency = types.Int64Null()
		return
	}
	data.DDMPollFrequency = types.Int64Value(*portGroup.DDM.PollFrequency)
}

func (r *PortGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state PortGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client.PlatformType == "Velos Controller" {
		resp.Diagnostics.AddError("Client Error", "`f5os_portgroup` resource is supported only on rSeries appliances.")
		return
	}
	// Warn if mode is changing (will trigger device reboot)
	if plan.Mode.ValueString() != state.Mode.ValueString() {
		resp.Diagnostics.AddWarning("Mode change will trigger device reboot",
			fmt.Sprintf("Changing port group %q mode from %s to %s will trigger a device reboot and may affect other running workloads",
				plan.Name.ValueString(), state.Mode.ValueString(), plan.Mode.ValueString()))
	}
	r.updateAndRead(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := waitForPortGroupReboot(ctx, r.client); err != nil {
		resp.Diagnostics.AddWarning("Port group configuration may still be applying", err.Error())
	}
	plan.ID = plan.Name
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PortGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PortGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Only clear DDM poll frequency on delete, do not change mode (mode persists in hardware)
	if err := r.client.ClearPortGroupDDM(data.Name.ValueString()); err != nil {
		resp.Diagnostics.AddWarning("Port group DDM could not be cleared",
			fmt.Sprintf("Resource removed from state but device DDM config persists: %s", err))
	}
}

func (r *PortGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

func (r *PortGroupResource) updateAndRead(_ context.Context, data *PortGroupResourceModel, diagnostics *diag.Diagnostics) {
	config := &f5ossdk.PortGroupConfig{Name: data.Name.ValueString(), Mode: data.Mode.ValueString()}
	if !data.DDMPollFrequency.IsNull() && !data.DDMPollFrequency.IsUnknown() {
		pollFrequency := data.DDMPollFrequency.ValueInt64()
		config.DDM = &f5ossdk.PortGroupDDMConfig{PollFrequency: &pollFrequency}
	}
	if err := r.client.SetPortGroupConfig(data.Name.ValueString(), config); err != nil {
		diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to configure port group %q: %s", data.Name.ValueString(), err))
		return
	}
	portGroup, err := r.client.GetPortGroup(data.Name.ValueString())
	if err != nil {
		diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Unable to read port group %q: %s", data.Name.ValueString(), err))
		return
	}
	data.Mode = types.StringValue(portGroup.Mode)
	r.setDDMPollFrequency(data, portGroup)
}

// waitForPortGroupReboot waits for the device to handle port group configuration
// changes. Port group mode changes trigger a system reboot; this helper sleeps
// for an initial grace period, then continuously tries to GET /f5-portgroup:portgroups
// until it succeeds, with a total timeout of 10 minutes.
//
// The wait is skipped when the client host is a loopback address (unit tests
// with httptest.Server) to avoid adding unnecessary latency.
func waitForPortGroupReboot(ctx context.Context, client *f5ossdk.F5os) error {
	// Skip the wait for unit tests targeting localhost/127.0.0.1.
	if strings.Contains(client.Host, "127.0.0.1") || strings.Contains(client.Host, "localhost") {
		return nil
	}

	// Initial grace period to let the device fully reboot if needed. Port group
	// mode changes can take several minutes to reboot and come back.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(30 * time.Second):
	}

	// Keep trying to GET /f5-portgroup:portgroups for up to 5 minutes
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Try to GET the portgroups endpoint - keep retrying until it succeeds
		resp, err := client.GetRequest("/f5-portgroup:portgroups")
		if err == nil && len(resp) > 0 {
			// Successfully got portgroups response, device has fully recovered
			return nil
		}

		// Failed, wait and retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return fmt.Errorf("RESTCONF API did not become available within 5 minutes after port group operation")
}
