package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5os "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// Ensure the implementation satisfies the expected interfaces
var (
	_ resource.Resource                = &SnmpResource{}
	_ resource.ResourceWithConfigure   = &SnmpResource{}
	_ resource.ResourceWithImportState = &SnmpResource{}
)

// SnmpResourceModel represents the schema model
type SnmpResourceModel struct {
	Id            types.String `tfsdk:"id"`
	SnmpCommunity types.List   `tfsdk:"snmp_community"`
	SnmpTarget    types.List   `tfsdk:"snmp_target"`
	SnmpUser      types.List   `tfsdk:"snmp_user"`
	SnmpMib       types.Object `tfsdk:"snmp_mib"`
	State         types.String `tfsdk:"state"`
}

// SnmpCommunityModel represents the schema model for SNMP community
type SnmpCommunityModel struct {
	Name          types.String `tfsdk:"name"`
	SecurityModel types.List   `tfsdk:"security_model"`
}

// SnmpTargetModel represents the schema model for SNMP target
type SnmpTargetModel struct {
	Name          types.String `tfsdk:"name"`
	SecurityModel types.String `tfsdk:"security_model"`
	Community     types.String `tfsdk:"community"`
	User          types.String `tfsdk:"user"`
	Ipv4Address   types.String `tfsdk:"ipv4_address"`
	Ipv6Address   types.String `tfsdk:"ipv6_address"`
	Port          types.Int64  `tfsdk:"port"`
}

// SnmpUserModel represents the schema model for SNMP user
type SnmpUserModel struct {
	Name          types.String `tfsdk:"name"`
	AuthProto     types.String `tfsdk:"auth_proto"`
	AuthPasswd    types.String `tfsdk:"auth_passwd"`
	PrivacyProto  types.String `tfsdk:"privacy_proto"`
	PrivacyPasswd types.String `tfsdk:"privacy_passwd"`
}

// SnmpMibModel represents the schema model for SNMP MIB
type SnmpMibModel struct {
	SysName     types.String `tfsdk:"sysname"`
	SysContact  types.String `tfsdk:"syscontact"`
	SysLocation types.String `tfsdk:"syslocation"`
}

// SnmpResource defines the resource implementation
type SnmpResource struct {
	client snmpClient
}

// snmpClient defines the subset of F5OS client methods used by this resource.
// This enables mocking for unit tests.
type snmpClient interface {
	CreateSnmpCommunities(payload []byte) error
	CreateSnmpUsers(payload []byte) error
	CreateSnmpTargets(payload []byte) error
	UpdateSnmpMib(payload []byte) error
	UpdateSnmpCommunities(payload []byte) error
	UpdateSnmpUsers(payload []byte) error
	UpdateSnmpTargets(payload []byte) error
	DeleteSnmpTarget(name string) error
	DeleteSnmpCommunity(name string) error
	DeleteSnmpUser(name string) error
}

// f5osSnmpClient adapts the concrete SDK client to the snmpClient interface.
type f5osSnmpClient struct{ c *f5os.F5os }

func (a *f5osSnmpClient) CreateSnmpCommunities(payload []byte) error {
	return a.c.CreateSnmpCommunities(payload)
}
func (a *f5osSnmpClient) CreateSnmpUsers(payload []byte) error { return a.c.CreateSnmpUsers(payload) }
func (a *f5osSnmpClient) CreateSnmpTargets(payload []byte) error {
	return a.c.CreateSnmpTargets(payload)
}
func (a *f5osSnmpClient) UpdateSnmpMib(payload []byte) error { return a.c.UpdateSnmpMib(payload) }
func (a *f5osSnmpClient) UpdateSnmpCommunities(payload []byte) error {
	return a.c.UpdateSnmpCommunities(payload)
}
func (a *f5osSnmpClient) UpdateSnmpUsers(payload []byte) error { return a.c.UpdateSnmpUsers(payload) }
func (a *f5osSnmpClient) UpdateSnmpTargets(payload []byte) error {
	return a.c.UpdateSnmpTargets(payload)
}
func (a *f5osSnmpClient) DeleteSnmpTarget(name string) error    { return a.c.DeleteSnmpTarget(name) }
func (a *f5osSnmpClient) DeleteSnmpCommunity(name string) error { return a.c.DeleteSnmpCommunity(name) }
func (a *f5osSnmpClient) DeleteSnmpUser(name string) error      { return a.c.DeleteSnmpUser(name) }

// NewSnmpResource creates a new instance of the resource
func NewSnmpResource() resource.Resource {
	return &SnmpResource{}
}

// Metadata returns the resource type name
func (r *SnmpResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snmp"
}

// Schema defines the schema for the resource
func (r *SnmpResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Resource used to manage SNMP configuration (Communities, Users, Targets, and MIB settings) on F5OS systems (VELOS or rSeries).\n\n" +
			"~> **NOTE:** The `f5os_snmp` resource manages SNMP settings on F5OS platforms using Open API. " +
			"Due to API restrictions, passwords cannot be retrieved which may lead to Terraform detecting changes on every plan.",
		Attributes: map[string]schema.Attribute{
			"snmp_community": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "List of SNMP Community configurations. Each community represents a group with specific security models.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Unique name for the SNMP community.",
						},
						"security_model": schema.ListAttribute{
							ElementType:         types.StringType,
							Optional:            true,
							Computed:            true,
							Default:             listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{types.StringValue("v1")})),
							MarkdownDescription: "List of security models for the community. Valid options: `v1`, `v2c`. Default is `[\"v1\"]`.",
						},
					},
				},
			},
			"snmp_target": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "List of SNMP Target configurations. Targets define where SNMP notifications are sent.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Unique name for the SNMP target.",
						},
						"security_model": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Security model for the SNMP target. Valid options: `v1`, `v2c`. Note: `v3` is applied automatically when `user` is specified.",
						},
						"community": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "SNMP community name to use for this target. Cannot be used with `user`.",
						},
						"user": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "SNMP user for SNMPv3 targets. Cannot be used with `community` or `security_model`.",
						},
						"ipv4_address": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "IPv4 address for the SNMP target. Cannot be used with `ipv6_address`.",
						},
						"ipv6_address": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "IPv6 address for the SNMP target. Cannot be used with `ipv4_address`.",
						},
						"port": schema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "Port number for the SNMP target.",
						},
					},
				},
			},
			"snmp_user": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "List of SNMP User configurations for SNMPv3. Due to API restrictions, passwords cannot be retrieved which leads to Terraform always detecting changes.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Unique name for the SNMP user.",
						},
						"auth_proto": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Authentication protocol. Valid options: `sha`, `md5`.",
						},
						"auth_passwd": schema.StringAttribute{
							Optional:            true,
							Sensitive:           true,
							MarkdownDescription: "Password for authentication.",
						},
						"privacy_proto": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Privacy protocol. Valid options: `aes`, `des`. Requires authentication to be configured.",
						},
						"privacy_passwd": schema.StringAttribute{
							Optional:            true,
							Sensitive:           true,
							MarkdownDescription: "Password for encryption.",
						},
					},
				},
			},
			"snmp_mib": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Custom SNMP MIB entries for system information.",
				Attributes: map[string]schema.Attribute{
					"sysname": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "SNMPv2 sysName.",
					},
					"syscontact": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "SNMPv2 sysContact.",
					},
					"syslocation": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "SNMPv2 sysLocation.",
					},
				},
			},
			"state": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("present"),
				MarkdownDescription: "State of the SNMP configuration. Valid options: `present`, `absent`. Default is `present`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for the resource, computed from the SNMP configuration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Configure configures the resource with the provider client
func (r *SnmpResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*f5os.F5os)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected *f5os.F5os, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = &f5osSnmpClient{c: client}
}

// computeResourceID generates a hash-based ID from SNMP configuration
// func computeSnmpResourceID(communities []SnmpCommunityModel, targets []SnmpTargetModel, users []SnmpUserModel, mib *SnmpMibModel) string {
// 	var configParts []string

// 	// Add communities
// 	for _, community := range communities {
// 		configParts = append(configParts, fmt.Sprintf("community:%s", community.Name.ValueString()))
// 	}

// 	// Add targets
// 	for _, target := range targets {
// 		configParts = append(configParts, fmt.Sprintf("target:%s", target.Name.ValueString()))
// 	}

// 	// Add users
// 	for _, user := range users {
// 		configParts = append(configParts, fmt.Sprintf("user:%s", user.Name.ValueString()))
// 	}

// 	// Add MIB
// 	if mib != nil {
// 		if !mib.SysName.IsNull() {
// 			configParts = append(configParts, fmt.Sprintf("mib:sysname:%s", mib.SysName.ValueString()))
// 		}
// 		if !mib.SysContact.IsNull() {
// 			configParts = append(configParts, fmt.Sprintf("mib:syscontact:%s", mib.SysContact.ValueString()))
// 		}
// 		if !mib.SysLocation.IsNull() {
// 			configParts = append(configParts, fmt.Sprintf("mib:syslocation:%s", mib.SysLocation.ValueString()))
// 		}
// 	}

// 	// Sort to ensure consistent hash
// 	sort.Strings(configParts)
// 	configStr := strings.Join(configParts, ";")

// 	// Generate SHA-256 hash
// 	hash := sha256.Sum256([]byte(configStr))
// 	return hex.EncodeToString(hash[:])
// }

// extractSnmpCommunities safely extracts SNMP communities from a types.List
func extractSnmpCommunities(ctx context.Context, list types.List) ([]SnmpCommunityModel, diag.Diagnostics) {
	var result []SnmpCommunityModel
	var diags diag.Diagnostics

	if !list.IsNull() && !list.IsUnknown() {
		diags = list.ElementsAs(ctx, &result, false)
	}

	return result, diags
}

// extractSnmpTargets safely extracts SNMP targets from a types.List
func extractSnmpTargets(ctx context.Context, list types.List) ([]SnmpTargetModel, diag.Diagnostics) {
	var result []SnmpTargetModel
	var diags diag.Diagnostics

	if !list.IsNull() && !list.IsUnknown() {
		diags = list.ElementsAs(ctx, &result, false)
	}

	return result, diags
}

// extractSnmpUsers safely extracts SNMP users from a types.List
func extractSnmpUsers(ctx context.Context, list types.List) ([]SnmpUserModel, diag.Diagnostics) {
	var result []SnmpUserModel
	var diags diag.Diagnostics

	if !list.IsNull() && !list.IsUnknown() {
		diags = list.ElementsAs(ctx, &result, false)
	}

	return result, diags
}

// extractSnmpMib safely extracts SNMP MIB from a types.Object
func extractSnmpMib(ctx context.Context, obj types.Object) (*SnmpMibModel, diag.Diagnostics) {
	var result SnmpMibModel
	var diags diag.Diagnostics

	if !obj.IsNull() && !obj.IsUnknown() {
		diags = obj.As(ctx, &result, basetypes.ObjectAsOptions{})
		if diags.HasError() {
			return nil, diags
		}
		return &result, diags
	}

	return nil, diags
}

// Create creates the resource and sets the initial Terraform state
func (r *SnmpResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SnmpResourceModel

	// Deserialize plan into Go struct
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Creating SNMP configuration")

	// Extract SNMP components
	communities, diags := extractSnmpCommunities(ctx, plan.SnmpCommunity)
	resp.Diagnostics.Append(diags...)

	targets, diags := extractSnmpTargets(ctx, plan.SnmpTarget)
	resp.Diagnostics.Append(diags...)

	users, diags := extractSnmpUsers(ctx, plan.SnmpUser)
	resp.Diagnostics.Append(diags...)

	mib, diags := extractSnmpMib(ctx, plan.SnmpMib)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Check state - if absent, we shouldn't be creating
	state := plan.State.ValueString()
	if state == "absent" {
		resp.Diagnostics.AddError(
			"Invalid State for Create",
			"Cannot create SNMP configuration with state 'absent'. Use state 'present' or omit the state parameter.",
		)
		return
	}

	// Call client to create SNMP config
	if err := r.createSnmpConfig(ctx, communities, targets, users, mib); err != nil {
		resp.Diagnostics.AddError(
			"SNMP Configuration Error",
			fmt.Sprintf("Failed to configure SNMP: %s", err),
		)
		return
	}

	// Set a stable, known ID for this singleton resource
	plan.Id = types.StringValue("snmp_config")
	plan.State = types.StringValue("present")

	tflog.Debug(ctx, "SNMP configuration created successfully", map[string]interface{}{
		"communities": len(communities),
		"targets":     len(targets),
		"users":       len(users),
		"mib":         mib != nil,
	})

	// Save final state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state with the latest data
func (r *SnmpResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SnmpResourceModel

	// Load current state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Reading SNMP configuration from F5OS")

	// IMPORTANT: The resource ID must never be changed in Read to prevent Terraform state drift and test failures.
	// During import, the state field might be empty, so we need to set it
	// For SNMP configuration resources, we assume the state is "present" if the resource exists
	if state.State.IsNull() || state.State.IsUnknown() {
		state.State = types.StringValue("present")
	}
	// Always use the static resource ID
	state.Id = types.StringValue("snmp_config")

	// For now, we'll keep the existing state as SNMP config reading is complex
	// This is a common pattern for configuration resources where the API doesn't
	// provide complete read capabilities, especially for sensitive data like passwords

	tflog.Debug(ctx, "SNMP configuration read completed", map[string]interface{}{
		"id":    state.Id.ValueString(),
		"state": state.State.ValueString(),
	})

	// Save current state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state
func (r *SnmpResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SnmpResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updating SNMP configuration")

	// Extract SNMP components
	communities, diags := extractSnmpCommunities(ctx, plan.SnmpCommunity)
	resp.Diagnostics.Append(diags...)

	targets, diags := extractSnmpTargets(ctx, plan.SnmpTarget)
	resp.Diagnostics.Append(diags...)

	users, diags := extractSnmpUsers(ctx, plan.SnmpUser)
	resp.Diagnostics.Append(diags...)

	mib, diags := extractSnmpMib(ctx, plan.SnmpMib)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Check state
	state := plan.State.ValueString()
	if state == "absent" {
		// If state is absent, we should remove the configuration
		if err := r.deleteSnmpConfig(ctx, communities, targets, users); err != nil {
			resp.Diagnostics.AddError(
				"SNMP Configuration Error",
				fmt.Sprintf("Failed to remove SNMP configuration: %s", err),
			)
			return
		}
	} else {
		// Call PATCH operation via client SDK
		if err := r.updateSnmpConfig(ctx, communities, targets, users, mib); err != nil {
			resp.Diagnostics.AddError(
				"SNMP Update Error",
				fmt.Sprintf("Failed to update SNMP configuration: %s", err),
			)
			return
		}
		// Ensure state remains known and present after update
		plan.State = types.StringValue("present")
	}

	// Use a static resource ID for SNMP config
	plan.Id = types.StringValue("snmp_config")

	tflog.Debug(ctx, "SNMP configuration updated successfully", map[string]interface{}{
		"communities": len(communities),
		"targets":     len(targets),
		"users":       len(users),
		"mib":         mib != nil,
		"state":       plan.State.ValueString(),
		"id":          plan.Id.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the resource and removes the Terraform state
func (r *SnmpResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SnmpResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Deleting SNMP configuration")

	// Extract SNMP components for deletion
	communities, diags := extractSnmpCommunities(ctx, state.SnmpCommunity)
	resp.Diagnostics.Append(diags...)

	targets, diags := extractSnmpTargets(ctx, state.SnmpTarget)
	resp.Diagnostics.Append(diags...)

	users, diags := extractSnmpUsers(ctx, state.SnmpUser)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete SNMP configuration
	if err := r.deleteSnmpConfig(ctx, communities, targets, users); err != nil {
		resp.Diagnostics.AddError(
			"SNMP Deletion Error",
			fmt.Sprintf("Failed to delete SNMP configuration: %s", err),
		)
		return
	}

	tflog.Debug(ctx, "SNMP configuration deleted", map[string]interface{}{
		"communities": len(communities),
		"targets":     len(targets),
		"users":       len(users),
		"id":          state.Id.ValueString(),
	})

}

// ImportState imports an existing SNMP configuration into Terraform state
func (r *SnmpResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *SnmpResource) createSnmpConfig(ctx context.Context, communities []SnmpCommunityModel, targets []SnmpTargetModel, users []SnmpUserModel, mib *SnmpMibModel) error {
	// Create communities first
	if len(communities) > 0 {
		communityPayload := r.buildCommunityPayload(communities)
		communityBytes, err := json.Marshal(communityPayload)
		if err != nil {
			return fmt.Errorf("failed to marshal community payload: %w", err)
		}

		err = r.client.CreateSnmpCommunities(communityBytes)
		if err != nil {
			return fmt.Errorf("failed to create SNMP communities: %w", err)
		}
	}

	// Create users before targets (targets may reference users)
	if len(users) > 0 {
		userPayload := r.buildUserPayload(users)
		userBytes, err := json.Marshal(userPayload)
		if err != nil {
			return fmt.Errorf("failed to marshal user payload: %w", err)
		}

		err = r.client.CreateSnmpUsers(userBytes)
		if err != nil {
			return fmt.Errorf("failed to create SNMP users: %w", err)
		}
	}

	// Create targets after users (so user references are valid)
	if len(targets) > 0 {
		targetPayload := r.buildTargetPayload(targets)
		targetBytes, err := json.Marshal(targetPayload)
		if err != nil {
			return fmt.Errorf("failed to marshal target payload: %w", err)
		}

		err = r.client.CreateSnmpTargets(targetBytes)
		if err != nil {
			return fmt.Errorf("failed to create SNMP targets: %w", err)
		}
	}

	// Create MIB settings last
	if mib != nil {
		mibPayload := r.buildMibPayload(mib)
		mibBytes, err := json.Marshal(mibPayload)
		if err != nil {
			return fmt.Errorf("failed to marshal MIB payload: %w", err)
		}

		err = r.client.UpdateSnmpMib(mibBytes)
		if err != nil {
			return fmt.Errorf("failed to configure SNMP MIB: %w", err)
		}
	}

	return nil
}

// updateSnmpConfig handles the update of SNMP configuration
func (r *SnmpResource) updateSnmpConfig(ctx context.Context, communities []SnmpCommunityModel, targets []SnmpTargetModel, users []SnmpUserModel, mib *SnmpMibModel) error {
	// Update communities first
	if len(communities) > 0 {
		communityPayload := map[string]interface{}{
			"communities": map[string]interface{}{
				"community": r.buildCommunityList(communities),
			},
		}
		communityBytes, err := json.Marshal(communityPayload)
		if err != nil {
			return fmt.Errorf("failed to marshal community payload: %w", err)
		}

		err = r.client.UpdateSnmpCommunities(communityBytes)
		if err != nil {
			return fmt.Errorf("failed to update SNMP communities: %w", err)
		}
	}

	// Update users before targets (targets may reference users)
	if len(users) > 0 {
		userPayload := map[string]interface{}{
			"users": map[string]interface{}{
				"user": r.buildUserList(users),
			},
		}
		userBytes, err := json.Marshal(userPayload)
		if err != nil {
			return fmt.Errorf("failed to marshal user payload: %w", err)
		}

		err = r.client.UpdateSnmpUsers(userBytes)
		if err != nil {
			return fmt.Errorf("failed to update SNMP users: %w", err)
		}
	}

	// Update targets after users (so user references are valid)
	if len(targets) > 0 {
		targetPayload := map[string]interface{}{
			"targets": map[string]interface{}{
				"target": r.buildTargetList(targets),
			},
		}
		targetBytes, err := json.Marshal(targetPayload)
		if err != nil {
			return fmt.Errorf("failed to marshal target payload: %w", err)
		}

		err = r.client.UpdateSnmpTargets(targetBytes)
		if err != nil {
			return fmt.Errorf("failed to update SNMP targets: %w", err)
		}
	}

	// Update MIB settings last
	if mib != nil {
		mibPayload := r.buildMibPayload(mib)
		mibBytes, err := json.Marshal(mibPayload)
		if err != nil {
			return fmt.Errorf("failed to marshal MIB payload: %w", err)
		}

		err = r.client.UpdateSnmpMib(mibBytes)
		if err != nil {
			return fmt.Errorf("failed to update SNMP MIB: %w", err)
		}
	}

	return nil
}

// deleteSnmpConfig handles the deletion of SNMP configuration
func (r *SnmpResource) deleteSnmpConfig(ctx context.Context, communities []SnmpCommunityModel, targets []SnmpTargetModel, users []SnmpUserModel) error {
	// Delete targets first (they may depend on communities/users)
	for _, target := range targets {
		err := r.client.DeleteSnmpTarget(target.Name.ValueString())
		if err != nil {
			tflog.Warn(ctx, "Failed to delete SNMP target", map[string]interface{}{
				"target": target.Name.ValueString(),
				"error":  err.Error(),
			})
		}
	}

	// Delete communities
	for _, community := range communities {
		err := r.client.DeleteSnmpCommunity(community.Name.ValueString())
		if err != nil {
			tflog.Warn(ctx, "Failed to delete SNMP community", map[string]interface{}{
				"community": community.Name.ValueString(),
				"error":     err.Error(),
			})
		}
	}

	// Delete users
	for _, user := range users {
		err := r.client.DeleteSnmpUser(user.Name.ValueString())
		if err != nil {
			tflog.Warn(ctx, "Failed to delete SNMP user", map[string]interface{}{
				"user":  user.Name.ValueString(),
				"error": err.Error(),
			})
		}
	}

	return nil
}

// Helper methods to build payloads

func (r *SnmpResource) buildCommunityPayload(communities []SnmpCommunityModel) map[string]interface{} {
	return map[string]interface{}{
		"communities": map[string]interface{}{
			"community": r.buildCommunityList(communities),
		},
	}
}

func (r *SnmpResource) buildCommunityList(communities []SnmpCommunityModel) []map[string]interface{} {
	var communityList []map[string]interface{}
	for _, community := range communities {
		var securityModels []string
		if !community.SecurityModel.IsNull() {
			var models []types.String
			community.SecurityModel.ElementsAs(context.Background(), &models, false)
			for _, model := range models {
				securityModels = append(securityModels, model.ValueString())
			}
		} else {
			securityModels = []string{"v1"}
		}

		communityItem := map[string]interface{}{
			"name": community.Name.ValueString(),
			"config": map[string]interface{}{
				"name":           community.Name.ValueString(),
				"security-model": securityModels,
			},
		}
		communityList = append(communityList, communityItem)
	}
	return communityList
}

func (r *SnmpResource) buildTargetPayload(targets []SnmpTargetModel) map[string]interface{} {
	return map[string]interface{}{
		"targets": map[string]interface{}{
			"target": r.buildTargetList(targets),
		},
	}
}

func (r *SnmpResource) buildTargetList(targets []SnmpTargetModel) []map[string]interface{} {
	var targetList []map[string]interface{}
	for _, target := range targets {
		config := map[string]interface{}{
			"name": target.Name.ValueString(),
		}

		if !target.SecurityModel.IsNull() {
			config["security-model"] = target.SecurityModel.ValueString()
		}

		if !target.Community.IsNull() {
			config["community"] = target.Community.ValueString()
		}

		if !target.User.IsNull() {
			config["user"] = target.User.ValueString()
		}

		if !target.Ipv4Address.IsNull() {
			config["ipv4"] = map[string]interface{}{
				"address": target.Ipv4Address.ValueString(),
				"port":    target.Port.ValueInt64(),
			}
		} else if !target.Ipv6Address.IsNull() {
			config["ipv6"] = map[string]interface{}{
				"address": target.Ipv6Address.ValueString(),
				"port":    target.Port.ValueInt64(),
			}
		}

		targetItem := map[string]interface{}{
			"name":   target.Name.ValueString(),
			"config": config,
		}
		targetList = append(targetList, targetItem)
	}
	return targetList
}

func (r *SnmpResource) buildUserPayload(users []SnmpUserModel) map[string]interface{} {
	return map[string]interface{}{
		"users": map[string]interface{}{
			"user": r.buildUserList(users),
		},
	}
}

func (r *SnmpResource) buildUserList(users []SnmpUserModel) []map[string]interface{} {
	var userList []map[string]interface{}
	for _, user := range users {
		config := map[string]interface{}{
			"name": user.Name.ValueString(),
		}

		if !user.AuthProto.IsNull() {
			config["authentication-protocol"] = user.AuthProto.ValueString()
		}

		if !user.AuthPasswd.IsNull() {
			config["authentication-password"] = user.AuthPasswd.ValueString()
		}

		if !user.PrivacyProto.IsNull() {
			config["privacy-protocol"] = user.PrivacyProto.ValueString()
		}

		if !user.PrivacyPasswd.IsNull() {
			config["privacy-password"] = user.PrivacyPasswd.ValueString()
		}

		userItem := map[string]interface{}{
			"name":   user.Name.ValueString(),
			"config": config,
		}
		userList = append(userList, userItem)
	}
	return userList
}

func (r *SnmpResource) buildMibPayload(mib *SnmpMibModel) map[string]interface{} {
	config := make(map[string]interface{})

	if !mib.SysContact.IsNull() {
		config["SNMPv2-MIB:sysContact"] = mib.SysContact.ValueString()
	}

	if !mib.SysName.IsNull() {
		config["SNMPv2-MIB:sysName"] = mib.SysName.ValueString()
	}

	if !mib.SysLocation.IsNull() {
		config["SNMPv2-MIB:sysLocation"] = mib.SysLocation.ValueString()
	}

	return map[string]interface{}{
		"SNMPv2-MIB:system": config,
	}
}
