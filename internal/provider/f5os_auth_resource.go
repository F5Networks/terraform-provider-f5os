package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5os "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// privateKeyOriginalAuthOrder is the private state key used to store the
// device's pre-existing auth_order so Delete can restore it.
const privateKeyOriginalAuthOrder = "original_auth_order"

// privateKeyOriginalRoleGIDs is the private state key used to store the
// device's pre-existing role GIDs so Delete can restore them.
const privateKeyOriginalRoleGIDs = "original_role_gids"

// privateStateSetter is satisfied by resp.Private on CreateResponse,
// ReadResponse, and UpdateResponse.
type privateStateSetter interface {
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

// privateStateGetter is satisfied by req.Private on UpdateRequest and
// DeleteRequest.
type privateStateGetter interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
}

// Ensure interface satisfaction
var _ resource.Resource = &AuthResource{}
var _ resource.ResourceWithImportState = &AuthResource{}

func NewAuthResource() resource.Resource { return &AuthResource{} }

// AuthResource manages AAA authentication settings
// Focus: authentication order and role GID mappings. Server groups and password policy can be extended.
type AuthResource struct{ client *f5os.F5os }

type authRemoteRoleModel struct {
	Rolename  types.String `tfsdk:"rolename"`
	RemoteGID types.Int64  `tfsdk:"remote_gid"`
	LDAPGroup types.String `tfsdk:"ldap_group"`
}

type AuthResourceModel struct {
	ID             types.String `tfsdk:"id"`
	AuthOrder      types.List   `tfsdk:"auth_order"`
	RemoteRoles    types.Set    `tfsdk:"remote_roles"`
	PasswordPolicy types.Object `tfsdk:"password_policy"`
}

func (r *AuthResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_auth"
}

func (r *AuthResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*f5os.F5os)
	}
}

func (r *AuthResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage AAA authentication on F5OS. Includes authentication method order and role GID mappings.",
		Attributes: map[string]schema.Attribute{
			"auth_order": schema.ListAttribute{
				MarkdownDescription: "Ordered list of authentication methods. Allowed values: local, radius, tacacs, ldap.",
				Optional:            true,
				ElementType:         types.StringType,
				Validators:          []validator.List{listAuthOrderValidator{}},
			},
			"password_policy": schema.SingleNestedAttribute{
				MarkdownDescription: "Password policy settings (note: device enforces final policy).",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"min_length":        schema.Int64Attribute{Optional: true},
					"max_length":        schema.Int64Attribute{Optional: true},
					"history":           schema.Int64Attribute{Optional: true},
					"max_age_days":      schema.Int64Attribute{Optional: true},
					"min_classes":       schema.Int64Attribute{Optional: true},
					"require_upper":     schema.BoolAttribute{Optional: true},
					"require_lower":     schema.BoolAttribute{Optional: true},
					"require_digit":     schema.BoolAttribute{Optional: true},
					"require_special":   schema.BoolAttribute{Optional: true},
					"allow_username":    schema.BoolAttribute{Optional: true},
					"allow_consecutive": schema.BoolAttribute{Optional: true},
				},
			},
			"remote_roles": schema.SetNestedAttribute{
				MarkdownDescription: "Remote role mappings. Configure role GID (and optionally LDAP group association).",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"rolename": schema.StringAttribute{
							Required: true,
						},
						"remote_gid": schema.Int64Attribute{
							Optional: true,
							Computed: true,
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
						},
						"ldap_group": schema.StringAttribute{Optional: true},
					},
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Synthetic ID for the auth resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *AuthResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AuthResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Creating F5OS auth configuration")

	// Create authentication order if provided
	if !plan.AuthOrder.IsNull() && !plan.AuthOrder.IsUnknown() {
		// Save the original auth order from the device before overwriting,
		// so Delete can restore it instead of removing it entirely.
		resp.Diagnostics.Append(r.snapshotAuthOrder(ctx, resp.Private)...)
		if resp.Diagnostics.HasError() {
			return
		}

		var methods []string
		if diags := plan.AuthOrder.ElementsAs(ctx, &methods, false); !diags.HasError() {
			tflog.Debug(ctx, "Creating auth order", map[string]any{"methods": methods})
			if err := r.createAuthOrder(ctx, methods); err != nil {
				resp.Diagnostics.AddError("Failed to create auth order", err.Error())
				return
			}
		} else {
			resp.Diagnostics.Append(diags...)
			return
		}
	}

	// Create remote role configurations if provided
	if !plan.RemoteRoles.IsNull() && !plan.RemoteRoles.IsUnknown() {
		var roles []authRemoteRoleModel
		if diags := plan.RemoteRoles.ElementsAs(ctx, &roles, false); !diags.HasError() {
			// Collect role names and snapshot their current GIDs on the
			// device before overwriting, so Delete can restore them.
			var roleNames []string
			for _, rr := range roles {
				if !rr.Rolename.IsNull() && !rr.Rolename.IsUnknown() {
					roleNames = append(roleNames, rr.Rolename.ValueString())
				}
			}
			resp.Diagnostics.Append(r.snapshotRoleGIDs(ctx, resp.Private, roleNames)...)
			if resp.Diagnostics.HasError() {
				return
			}

			for _, rr := range roles {
				if rr.Rolename.IsNull() || rr.Rolename.IsUnknown() {
					continue
				}
				rolename := rr.Rolename.ValueString()
				var gidPtr *int64
				if !rr.RemoteGID.IsNull() && !rr.RemoteGID.IsUnknown() {
					v := rr.RemoteGID.ValueInt64()
					gidPtr = &v
				}
				tflog.Debug(ctx, "Creating role config", map[string]any{"rolename": rolename, "gid": gidPtr})
				if err := r.createRoleConfig(ctx, rolename, gidPtr); err != nil {
					resp.Diagnostics.AddError("Failed to create role config",
						fmt.Sprintf("Error configuring role %s: %v", rolename, err))
					return
				}
			}
		} else {
			resp.Diagnostics.Append(diags...)
			return
		}
	}

	// Handle password policy (placeholder for future implementation)
	if !plan.PasswordPolicy.IsNull() && !plan.PasswordPolicy.IsUnknown() {
		tflog.Warn(ctx, "Password policy configuration is not yet implemented")
	}

	plan.ID = types.StringValue("f5os-auth")

	// Read back actual device state so Terraform detects any discrepancies
	// between what was planned and what the device accepted.
	if !plan.AuthOrder.IsNull() {
		if err := r.readAuthOrder(ctx, &plan); err != nil {
			resp.Diagnostics.AddError("Failed to read back auth order after create", err.Error())
			return
		}
	}
	if !plan.RemoteRoles.IsNull() {
		if err := r.readRoleConfig(ctx, &plan); err != nil {
			resp.Diagnostics.AddError("Failed to read back role config after create", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AuthResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AuthResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading auth configuration from device")

	// Detect import: all user-configurable fields are null because
	// ImportStatePassthroughID only sets the ID.
	isImport := state.AuthOrder.IsNull() && state.RemoteRoles.IsNull() && state.PasswordPolicy.IsNull()

	// During import, snapshot the device's current auth_order into private
	// state so Delete can restore it. Create handles this for the normal
	// lifecycle, but import bypasses Create entirely.
	//
	// Role GIDs are NOT snapshotted here because import alone doesn't
	// modify any role GIDs. The first apply after import will call Update,
	// which uses ensureRoleGIDsSnapshotted to capture any roles before
	// modifying them.
	if isImport {
		existing, diags := req.Private.GetKey(ctx, privateKeyOriginalAuthOrder)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if existing == nil {
			resp.Diagnostics.Append(r.snapshotAuthOrder(ctx, resp.Private)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
	}

	// Read auth_order from device when managed or during import.
	if !state.AuthOrder.IsNull() || isImport {
		if err := r.readAuthOrder(ctx, &state); err != nil {
			resp.Diagnostics.AddError("Failed to read auth order from device", err.Error())
			return
		}
	}

	// Read remote_roles from device when managed or during import.
	if !state.RemoteRoles.IsNull() || isImport {
		if err := r.readRoleConfig(ctx, &state); err != nil {
			resp.Diagnostics.AddError("Failed to read role config from device", err.Error())
			return
		}
	}

	state.ID = types.StringValue("f5os-auth")
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AuthResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AuthResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update authentication order if specified
	if !plan.AuthOrder.IsNull() && !plan.AuthOrder.IsUnknown() {
		var methods []string
		resp.Diagnostics.Append(plan.AuthOrder.ElementsAs(ctx, &methods, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		if err := r.updateAuthOrder(ctx, methods); err != nil {
			resp.Diagnostics.AddError("Failed to update authentication order", err.Error())
			return
		}
	}

	// Update role configurations if specified
	if !plan.RemoteRoles.IsNull() && !plan.RemoteRoles.IsUnknown() {
		var roles []authRemoteRoleModel
		resp.Diagnostics.Append(plan.RemoteRoles.ElementsAs(ctx, &roles, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		// If the user added new roles that weren't in the original
		// Create snapshot, capture their device-side GIDs now before
		// we overwrite them, so Delete can restore them later.
		resp.Diagnostics.Append(r.ensureRoleGIDsSnapshotted(ctx, req.Private, resp.Private, roles)...)
		if resp.Diagnostics.HasError() {
			return
		}

		for _, role := range roles {
			rolename := role.Rolename.ValueString()
			var gidPtr *int64
			if !role.RemoteGID.IsNull() && !role.RemoteGID.IsUnknown() {
				gid := role.RemoteGID.ValueInt64()
				gidPtr = &gid
			}

			if err := r.updateRoleConfig(ctx, rolename, gidPtr); err != nil {
				resp.Diagnostics.AddError(fmt.Sprintf("Failed to update role %s", rolename), err.Error())
				return
			}
		}
	}

	plan.ID = types.StringValue("f5os-auth")

	// Read back actual device state so Terraform detects any discrepancies
	// between what was planned and what the device accepted.
	if !plan.AuthOrder.IsNull() {
		if err := r.readAuthOrder(ctx, &plan); err != nil {
			resp.Diagnostics.AddError("Failed to read back auth order after update", err.Error())
			return
		}
	}
	if !plan.RemoteRoles.IsNull() {
		if err := r.readRoleConfig(ctx, &plan); err != nil {
			resp.Diagnostics.AddError("Failed to read back role config after update", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AuthResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Restore the original auth order that was saved during Create, rather
	// than deleting the authentication-method array entirely. This avoids
	// nuking auth config that existed before Terraform managed it.
	origData, diags := req.Private.GetKey(ctx, privateKeyOriginalAuthOrder)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if origData != nil {
		var origMethods []string
		if err := json.Unmarshal(origData, &origMethods); err != nil {
			tflog.Warn(ctx, "failed to deserialize original auth order from private state, falling back to delete",
				map[string]any{"error": err.Error()})
			if err := r.deleteAuthOrder(ctx); err != nil {
				tflog.Warn(ctx, "failed deleting auth order", map[string]any{"error": err.Error()})
			}
		} else {
			if err := r.restoreAuthOrder(ctx, origMethods); err != nil {
				tflog.Warn(ctx, "failed restoring original auth order, falling back to delete",
					map[string]any{"error": err.Error(), "methods": origMethods})
				if err := r.deleteAuthOrder(ctx); err != nil {
					tflog.Warn(ctx, "failed deleting auth order", map[string]any{"error": err.Error()})
				}
			}
		}
	} else {
		// No original auth order saved (e.g., resource created before this fix).
		// Fall back to the old behavior of deleting the auth order.
		tflog.Warn(ctx, "No original auth order in private state, falling back to delete")
		if err := r.deleteAuthOrder(ctx); err != nil {
			tflog.Warn(ctx, "failed deleting auth order", map[string]any{"error": err.Error()})
		}
	}

	// Restore the original role GIDs that were saved during Create/Import,
	// so destroy does not leave modified GIDs on the device.
	origRoleData, diags := req.Private.GetKey(ctx, privateKeyOriginalRoleGIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if origRoleData != nil {
		var origGIDs map[string]int
		if err := json.Unmarshal(origRoleData, &origGIDs); err != nil {
			tflog.Warn(ctx, "failed to deserialize original role GIDs from private state, skipping role restoration",
				map[string]any{"error": err.Error()})
		} else {
			for rolename, gid := range origGIDs {
				if gid == 0 {
					// The role had no remote-gid before Terraform managed it;
					// delete the leaf to restore the unset state.
					tflog.Debug(ctx, "Clearing role remote-gid", map[string]any{"rolename": rolename})
					if err := r.client.ClearRoleRemoteGID(rolename); err != nil {
						tflog.Warn(ctx, "failed clearing role remote-gid",
							map[string]any{"rolename": rolename, "error": err.Error()})
					}
				} else {
					gid64 := int64(gid)
					tflog.Debug(ctx, "Restoring role config", map[string]any{"rolename": rolename, "gid": gid64})
					if err := r.client.SetRoleConfig(rolename, &gid64); err != nil {
						tflog.Warn(ctx, "failed restoring original role GID",
							map[string]any{"rolename": rolename, "gid": gid64, "error": err.Error()})
					}
				}
			}
		}
	} else {
		tflog.Warn(ctx, "No original role GIDs in private state, skipping role restoration")
	}

	resp.State.RemoveResource(ctx)
}

func (r *AuthResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

//go:cover ignore
func (r *AuthResource) getAuthOrder(ctx context.Context) ([]string, error) {
	openConfigMethods, err := r.client.GetAuthOrder()
	if err != nil {
		return nil, err
	}
	if openConfigMethods == nil {
		return nil, nil
	}

	// Map OpenConfig identifiers back to user-friendly names
	methodMap := map[string]string{
		"openconfig-aaa-types:LOCAL":      "local",
		"openconfig-aaa-types:RADIUS_ALL": "radius",
		"openconfig-aaa-types:TACACS_ALL": "tacacs",
		"f5-openconfig-aaa-ldap:LDAP_ALL": "ldap",
	}

	out := make([]string, 0, len(openConfigMethods))
	for _, method := range openConfigMethods {
		if mappedMethod, exists := methodMap[method]; exists {
			out = append(out, mappedMethod)
		} else {
			// Fallback to original value if mapping not found
			out = append(out, method)
		}
	}
	return out, nil
}

//go:cover ignore
func (r *AuthResource) listRoles(ctx context.Context) (map[string]int, error) {
	return r.client.GetRoles()
}

// snapshotAuthOrder reads the current auth_order from the device and saves it
// to private state so Delete can restore it later. Does nothing if the device
// has no auth_order configured.
func (r *AuthResource) snapshotAuthOrder(ctx context.Context, private privateStateSetter) diag.Diagnostics {
	var diags diag.Diagnostics
	methods, err := r.getAuthOrder(ctx)
	if err != nil {
		diags.AddError("Failed to read original auth order from device", err.Error())
		return diags
	}
	if methods == nil {
		return diags
	}
	data, err := json.Marshal(methods)
	if err != nil {
		diags.AddError("Failed to serialize original auth order", err.Error())
		return diags
	}
	diags.Append(private.SetKey(ctx, privateKeyOriginalAuthOrder, data)...)
	if !diags.HasError() {
		tflog.Debug(ctx, "Saved original auth order to private state", map[string]any{"original": methods})
	}
	return diags
}

// snapshotRoleGIDs reads the current role GIDs from the device for the
// specified roles and saves them to private state so Delete can restore
// them later.
func (r *AuthResource) snapshotRoleGIDs(ctx context.Context, private privateStateSetter, roleNames []string) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(roleNames) == 0 {
		return diags
	}

	allRoles, err := r.listRoles(ctx)
	if err != nil {
		diags.AddError("Failed to read original role GIDs from device", err.Error())
		return diags
	}

	// Build a map of only the roles the user declared, with their
	// current GIDs on the device. Roles that don't exist yet get
	// GID 0 (the device default / unset state).
	snapshot := make(map[string]int, len(roleNames))
	for _, name := range roleNames {
		snapshot[name] = allRoles[name] // 0 if not present
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		diags.AddError("Failed to serialize original role GIDs", err.Error())
		return diags
	}
	diags.Append(private.SetKey(ctx, privateKeyOriginalRoleGIDs, data)...)
	if !diags.HasError() {
		tflog.Debug(ctx, "Saved original role GIDs to private state", map[string]any{"original": snapshot})
	}
	return diags
}

// ensureRoleGIDsSnapshotted reads the existing role GID snapshot from
// private state and checks whether any of the planned roles are missing
// from it. For any new roles (added to the config after initial Create),
// it reads their current device-side GIDs and merges them into the
// snapshot so Delete can restore them.
func (r *AuthResource) ensureRoleGIDsSnapshotted(ctx context.Context, privateReader privateStateGetter, privateWriter privateStateSetter, roles []authRemoteRoleModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Read the existing snapshot.
	existingData, d := privateReader.GetKey(ctx, privateKeyOriginalRoleGIDs)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}

	existing := make(map[string]int)
	if existingData != nil {
		if err := json.Unmarshal(existingData, &existing); err != nil {
			diags.AddError("Failed to deserialize existing role GID snapshot", err.Error())
			return diags
		}
	}

	// Find role names in the plan that are not yet in the snapshot.
	var newNames []string
	for _, rr := range roles {
		if rr.Rolename.IsNull() || rr.Rolename.IsUnknown() {
			continue
		}
		name := rr.Rolename.ValueString()
		if _, ok := existing[name]; !ok {
			newNames = append(newNames, name)
		}
	}

	if len(newNames) == 0 {
		return diags
	}

	// Read the current device GIDs for the new roles.
	allRoles, err := r.listRoles(ctx)
	if err != nil {
		diags.AddError("Failed to read role GIDs from device for snapshot update", err.Error())
		return diags
	}
	for _, name := range newNames {
		existing[name] = allRoles[name] // 0 if not present on device
	}

	// Write the merged snapshot back.
	data, err := json.Marshal(existing)
	if err != nil {
		diags.AddError("Failed to serialize updated role GID snapshot", err.Error())
		return diags
	}
	diags.Append(privateWriter.SetKey(ctx, privateKeyOriginalRoleGIDs, data)...)
	if !diags.HasError() {
		tflog.Debug(ctx, "Merged new role GIDs into snapshot", map[string]any{"new_roles": newNames, "snapshot": existing})
	}
	return diags
}

// createAuthOrder sets the authentication method order
func (r *AuthResource) createAuthOrder(ctx context.Context, methods []string) error {
	tflog.Debug(ctx, "Creating auth order", map[string]any{"methods": methods})
	return r.client.SetAuthOrder(methods)
}

// createRoleConfig creates a role with specific gid
func (r *AuthResource) createRoleConfig(ctx context.Context, rolename string, gid *int64) error {
	tflog.Debug(ctx, "Creating role config", map[string]any{"rolename": rolename, "gid": gid})
	return r.client.SetRoleConfig(rolename, gid)
}

//go:cover ignore
func (r *AuthResource) readAuthOrder(ctx context.Context, state *AuthResourceModel) error {
	methods, err := r.getAuthOrder(ctx)
	if err != nil {
		return err
	}
	if methods != nil {
		list, _ := types.ListValueFrom(ctx, types.StringType, methods)
		state.AuthOrder = list
	}
	return nil
}

//go:cover ignore
func (r *AuthResource) readRoleConfig(ctx context.Context, state *AuthResourceModel) error {
	roles, err := r.listRoles(ctx)
	if err != nil {
		return err
	}

	// Build the set of role names the user configured so we can filter the
	// device response to only those roles.
	configuredNames := make(map[string]bool)
	if !state.RemoteRoles.IsNull() && !state.RemoteRoles.IsUnknown() {
		var existing []authRemoteRoleModel
		diags := state.RemoteRoles.ElementsAs(ctx, &existing, false)
		if diags.HasError() {
			return fmt.Errorf("failed to read configured roles from state: %s", diags.Errors()[0].Detail())
		}
		for _, er := range existing {
			if !er.Rolename.IsNull() && !er.Rolename.IsUnknown() {
				configuredNames[er.Rolename.ValueString()] = true
			}
		}
	}

	var roleModels []authRemoteRoleModel
	for name, gid := range roles {
		// Only include roles the user declared in their config. During import,
		// configuredNames is empty (no prior state), so include all roles.
		if len(configuredNames) > 0 && !configuredNames[name] {
			continue
		}
		item := authRemoteRoleModel{Rolename: types.StringValue(name)}
		if gid > 0 {
			item.RemoteGID = types.Int64Value(int64(gid))
		}
		roleModels = append(roleModels, item)
	}

	sv, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: map[string]attr.Type{
		"rolename":   types.StringType,
		"remote_gid": types.Int64Type,
		"ldap_group": types.StringType,
	}}, roleModels)
	if diags.HasError() {
		return fmt.Errorf("failed to convert role models to set: %s", diags.Errors()[0].Detail())
	}
	state.RemoteRoles = sv
	return nil
}

// updateAuthOrder updates the authentication method order
func (r *AuthResource) updateAuthOrder(ctx context.Context, methods []string) error {
	tflog.Debug(ctx, "Updating auth order", map[string]any{"methods": methods})
	return r.client.SetAuthOrder(methods)
}

// updateRoleConfig updates a role with specific gid
func (r *AuthResource) updateRoleConfig(ctx context.Context, rolename string, gid *int64) error {
	tflog.Debug(ctx, "Updating role config", map[string]any{"rolename": rolename, "gid": gid})
	return r.client.SetRoleConfig(rolename, gid)
}

// restoreAuthOrder restores the authentication method order to a previous state
func (r *AuthResource) restoreAuthOrder(ctx context.Context, methods []string) error {
	tflog.Debug(ctx, "Restoring auth order", map[string]any{"methods": methods})
	return r.client.SetAuthOrder(methods)
}

// deleteAuthOrder removes the authentication method order
func (r *AuthResource) deleteAuthOrder(ctx context.Context) error {
	tflog.Debug(ctx, "Deleting auth order")
	return r.client.ClearAuthOrder()
} // Simple validator to restrict auth_order values
type listAuthOrderValidator struct{}

var _ validator.List = listAuthOrderValidator{}

func (v listAuthOrderValidator) Description(ctx context.Context) string {
	return "Validates allowed auth methods"
}

//go:cover ignore
func (v listAuthOrderValidator) MarkdownDescription(ctx context.Context) string {
	return "Allowed values: local, radius, tacacs, ldap"
}
func (v listAuthOrderValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	var methods []string
	if diags := req.ConfigValue.ElementsAs(ctx, &methods, false); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Check for empty auth_order
	if len(methods) == 0 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid auth_order", "auth_order cannot be empty")
		return
	}

	// Check for valid auth methods and duplicates
	allowed := map[string]struct{}{"local": {}, "radius": {}, "tacacs": {}, "ldap": {}}
	seen := make(map[string]bool)

	for _, m := range methods {
		if _, ok := allowed[m]; !ok {
			resp.Diagnostics.AddAttributeError(req.Path, "Invalid authentication method", fmt.Sprintf("%q is not one of: local, radius, tacacs, ldap", m))
		}
		if seen[m] {
			resp.Diagnostics.AddAttributeError(req.Path, "Duplicate authentication method", fmt.Sprintf("duplicate authentication method %q found in auth_order", m))
		}
		seen[m] = true
	}
}
