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
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
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

// passwordPolicyModel represents the password policy settings in Terraform state.
type passwordPolicyModel struct {
	MinLength           types.Int64 `tfsdk:"min_length"`
	RequiredNumeric     types.Int64 `tfsdk:"required_numeric"`
	RequiredUppercase   types.Int64 `tfsdk:"required_uppercase"`
	RequiredLowercase   types.Int64 `tfsdk:"required_lowercase"`
	RequiredSpecial     types.Int64 `tfsdk:"required_special"`
	RequiredDifferences types.Int64 `tfsdk:"required_differences"`
	RejectUsername      types.Bool  `tfsdk:"reject_username"`
	ApplyToRoot         types.Bool  `tfsdk:"apply_to_root"`
	Retries             types.Int64 `tfsdk:"retries"`
	MaxLoginFailures    types.Int64 `tfsdk:"max_login_failures"`
	UnlockTime          types.Int64 `tfsdk:"unlock_time"`
	RootLockout         types.Bool  `tfsdk:"root_lockout"`
	RootUnlockTime      types.Int64 `tfsdk:"root_unlock_time"`
	MaxAge              types.Int64 `tfsdk:"max_age"`
	// v1.7+ only fields
	MaxLetterRepeat   types.Int64 `tfsdk:"max_letter_repeat"`
	MaxSequenceRepeat types.Int64 `tfsdk:"max_sequence_repeat"`
	MaxClassRepeat    types.Int64 `tfsdk:"max_class_repeat"`
	// v2.0.0+ only fields
	MinDays  types.Int64 `tfsdk:"min_days"`
	Remember types.Int64 `tfsdk:"remember"`
	WarnAge  types.Int64 `tfsdk:"warn_age"`
}

// loginPolicyModel represents the AAA login-policy settings in Terraform state.
// Introduced in F5OS 2.0.0 (f5-openconfig-aaa-login-policy module).
type loginPolicyModel struct {
	AdminRoleLimit          types.Bool  `tfsdk:"admin_role_limit"`
	RestconfMaxSessionLimit types.Int64 `tfsdk:"restconf_max_session_limit"`
	SSHMaxSessionLimit      types.Int64 `tfsdk:"ssh_max_session_limit"`
}

// ldapConfigModel represents the LDAP server-config object classes in
// Terraform state. Introduced in F5OS 2.0.0 (f5-openconfig-aaa-ldap module).
type ldapConfigModel struct {
	UserObjectClass  types.List `tfsdk:"user_object_class"`
	GroupObjectClass types.List `tfsdk:"group_object_class"`
}

type AuthResourceModel struct {
	ID             types.String `tfsdk:"id"`
	AuthOrder      types.List   `tfsdk:"auth_order"`
	RemoteRoles    types.Set    `tfsdk:"remote_roles"`
	PasswordPolicy types.Object `tfsdk:"password_policy"`
	LoginPolicy    types.Object `tfsdk:"login_policy"`
	Ldap           types.Object `tfsdk:"ldap"`
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
		MarkdownDescription: "Manage AAA authentication on F5OS. Includes authentication method order, role GID mappings, and password policy.\n\n" +
			"~> **NOTE:** Running `terraform destroy` will restore the original authentication order and revert any role GID changes made by this resource.",
		Attributes: map[string]schema.Attribute{
			"auth_order": schema.ListAttribute{
				MarkdownDescription: "Ordered list of authentication methods. Allowed values: local, radius, tacacs, ldap.",
				Optional:            true,
				ElementType:         types.StringType,
				Validators:          []validator.List{listAuthOrderValidator{}},
			},
			"password_policy": schema.SingleNestedAttribute{
				MarkdownDescription: "Password policy settings. Only fields you specify are managed; unspecified fields are left at device defaults.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"min_length": schema.Int64Attribute{
						MarkdownDescription: "Minimum password length.",
						Optional:            true,
					},
					"required_numeric": schema.Int64Attribute{
						MarkdownDescription: "Required numeric digit count.",
						Optional:            true,
					},
					"required_uppercase": schema.Int64Attribute{
						MarkdownDescription: "Required uppercase character count.",
						Optional:            true,
					},
					"required_lowercase": schema.Int64Attribute{
						MarkdownDescription: "Required lowercase character count.",
						Optional:            true,
					},
					"required_special": schema.Int64Attribute{
						MarkdownDescription: "Required special character count.",
						Optional:            true,
					},
					"required_differences": schema.Int64Attribute{
						MarkdownDescription: "Characters that must differ from previous password.",
						Optional:            true,
					},
					"reject_username": schema.BoolAttribute{
						MarkdownDescription: "Reject passwords containing the username.",
						Optional:            true,
					},
					"apply_to_root": schema.BoolAttribute{
						MarkdownDescription: "Apply password restrictions to root accounts.",
						Optional:            true,
					},
					"retries": schema.Int64Attribute{
						MarkdownDescription: "Password entry retries before failure.",
						Optional:            true,
					},
					"max_login_failures": schema.Int64Attribute{
						MarkdownDescription: "Failed login attempts before lockout.",
						Optional:            true,
					},
					"unlock_time": schema.Int64Attribute{
						MarkdownDescription: "Account unlock time in seconds (0 = manual).",
						Optional:            true,
					},
					"root_lockout": schema.BoolAttribute{
						MarkdownDescription: "Enable lockout of root accounts.",
						Optional:            true,
					},
					"root_unlock_time": schema.Int64Attribute{
						MarkdownDescription: "Root account unlock time in seconds.",
						Optional:            true,
					},
					"max_age": schema.Int64Attribute{
						MarkdownDescription: "Password max age in days (0 = never expires).",
						Optional:            true,
					},
					// v1.7+ only fields
					"max_letter_repeat": schema.Int64Attribute{
						MarkdownDescription: "Max repeating lowercase letters allowed. Only supported on F5OS >= v1.7.",
						Optional:            true,
					},
					"max_sequence_repeat": schema.Int64Attribute{
						MarkdownDescription: "Max repeating letters/digits allowed. Only supported on F5OS >= v1.7.",
						Optional:            true,
					},
					"max_class_repeat": schema.Int64Attribute{
						MarkdownDescription: "Max repeating chars of any class allowed. Only supported on F5OS >= v1.7.",
						Optional:            true,
					},
					// v2.0.0+ only fields
					"min_days": schema.Int64Attribute{
						MarkdownDescription: "Minimum number of days between password changes. Only supported on F5OS >= 2.0.0.",
						Optional:            true,
					},
					"remember": schema.Int64Attribute{
						MarkdownDescription: "Number of previous passwords remembered to prevent reuse. Only supported on F5OS >= 2.0.0.",
						Optional:            true,
					},
					"warn_age": schema.Int64Attribute{
						MarkdownDescription: "Number of days before expiration to warn the user. Only supported on F5OS >= 2.0.0.",
						Optional:            true,
					},
				},
			},
			"login_policy": schema.SingleNestedAttribute{
				MarkdownDescription: "AAA login-policy settings. Only supported on F5OS >= 2.0.0. Only fields you specify are managed; unspecified fields are left at device defaults.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"admin_role_limit": schema.BoolAttribute{
						MarkdownDescription: "Whether to limit concurrent sessions for the admin role.",
						Optional:            true,
					},
					"restconf_max_session_limit": schema.Int64Attribute{
						MarkdownDescription: "Maximum number of concurrent RESTCONF sessions.",
						Optional:            true,
						Validators: []validator.Int64{
							int64validator.AtLeast(1),
						},
					},
					"ssh_max_session_limit": schema.Int64Attribute{
						MarkdownDescription: "Maximum number of concurrent SSH sessions.",
						Optional:            true,
						Validators: []validator.Int64{
							int64validator.AtLeast(1),
						},
					},
				},
			},
			"ldap": schema.SingleNestedAttribute{
				MarkdownDescription: "LDAP server configuration. Only supported on F5OS >= 2.0.0. " +
					"Only fields you specify are managed; unspecified fields are left at device defaults.",
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"user_object_class": schema.ListAttribute{
						MarkdownDescription: "Object classes used when searching for LDAP user objects (e.g. [\"posixAccount\"]).",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"group_object_class": schema.ListAttribute{
						MarkdownDescription: "Object classes used when searching for LDAP group objects (e.g. [\"posixGroup\"]).",
						Optional:            true,
						ElementType:         types.StringType,
					},
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

	// Handle password policy if provided
	if !plan.PasswordPolicy.IsNull() && !plan.PasswordPolicy.IsUnknown() {
		var ppModel passwordPolicyModel
		resp.Diagnostics.Append(plan.PasswordPolicy.As(ctx, &ppModel, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Version guard for v1.7+ fields
		resp.Diagnostics.Append(r.validateV17Fields(ctx, &ppModel)...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Version guard for v2.0.0+ fields
		resp.Diagnostics.Append(r.validateV20Fields(ctx, &ppModel)...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Write the password policy to the device
		resp.Diagnostics.Append(r.writePasswordPolicy(ctx, &ppModel)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Handle login policy if provided (F5OS 2.0.0+ only)
	if !plan.LoginPolicy.IsNull() && !plan.LoginPolicy.IsUnknown() {
		var lpModel loginPolicyModel
		resp.Diagnostics.Append(plan.LoginPolicy.As(ctx, &lpModel, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(r.writeLoginPolicy(ctx, &lpModel)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Handle LDAP config if provided (F5OS 2.0.0+ only)
	if !plan.Ldap.IsNull() && !plan.Ldap.IsUnknown() {
		var ldapModel ldapConfigModel
		resp.Diagnostics.Append(plan.Ldap.As(ctx, &ldapModel, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(r.writeLdapConfig(ctx, &ldapModel)...)
		if resp.Diagnostics.HasError() {
			return
		}
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
	if !plan.LoginPolicy.IsNull() {
		resp.Diagnostics.Append(r.readLoginPolicy(ctx, &plan, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	if !plan.Ldap.IsNull() {
		resp.Diagnostics.Append(r.readLdapConfig(ctx, &plan, false)...)
		if resp.Diagnostics.HasError() {
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
	isImport := state.AuthOrder.IsNull() && state.RemoteRoles.IsNull() && state.PasswordPolicy.IsNull() && state.LoginPolicy.IsNull() && state.Ldap.IsNull()

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

	// Read password_policy from device when managed or during import.
	if !state.PasswordPolicy.IsNull() || isImport {
		resp.Diagnostics.Append(r.readPasswordPolicy(ctx, &state, isImport)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Read login_policy from device when managed or during import. Login
	// policy is only available on F5OS 2.0.0+; on older devices the read is
	// skipped during import (nothing to import) and, when managed, the write
	// would already have failed at Create/Update time.
	if !state.LoginPolicy.IsNull() || (isImport && platformVersionAtLeast(r.client.PlatformVersion, "v2.0")) {
		resp.Diagnostics.Append(r.readLoginPolicy(ctx, &state, isImport)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Read ldap config from device when managed or during import. Like
	// login_policy, the LDAP object classes are only available on F5OS
	// 2.0.0+; on older devices the import read is skipped and a managed
	// write would already have failed at Create/Update time.
	if !state.Ldap.IsNull() || (isImport && platformVersionAtLeast(r.client.PlatformVersion, "v2.0")) {
		resp.Diagnostics.Append(r.readLdapConfig(ctx, &state, isImport)...)
		if resp.Diagnostics.HasError() {
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

	// Update password policy if specified
	if !plan.PasswordPolicy.IsNull() && !plan.PasswordPolicy.IsUnknown() {
		var ppModel passwordPolicyModel
		resp.Diagnostics.Append(plan.PasswordPolicy.As(ctx, &ppModel, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Version guard for v1.7+ fields
		resp.Diagnostics.Append(r.validateV17Fields(ctx, &ppModel)...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Version guard for v2.0.0+ fields
		resp.Diagnostics.Append(r.validateV20Fields(ctx, &ppModel)...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Write the password policy to the device
		resp.Diagnostics.Append(r.writePasswordPolicy(ctx, &ppModel)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Update login policy if specified (F5OS 2.0.0+ only)
	if !plan.LoginPolicy.IsNull() && !plan.LoginPolicy.IsUnknown() {
		var lpModel loginPolicyModel
		resp.Diagnostics.Append(plan.LoginPolicy.As(ctx, &lpModel, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(r.writeLoginPolicy(ctx, &lpModel)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Update LDAP config if specified (F5OS 2.0.0+ only)
	if !plan.Ldap.IsNull() && !plan.Ldap.IsUnknown() {
		var ldapModel ldapConfigModel
		resp.Diagnostics.Append(plan.Ldap.As(ctx, &ldapModel, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(r.writeLdapConfig(ctx, &ldapModel)...)
		if resp.Diagnostics.HasError() {
			return
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
	if !plan.LoginPolicy.IsNull() {
		resp.Diagnostics.Append(r.readLoginPolicy(ctx, &plan, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	if !plan.Ldap.IsNull() {
		resp.Diagnostics.Append(r.readLdapConfig(ctx, &plan, false)...)
		if resp.Diagnostics.HasError() {
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

	// cleanupErr captures the final error from the auth_order cleanup so it
	// can be surfaced as a diagnostic. Unlike role-GID restoration (which is
	// best-effort), failing to clear/restore auth_order must fail the destroy:
	// otherwise Terraform removes the resource from state while the device is
	// left with the Terraform-managed auth_order still applied (a dirty
	// device with no error surfaced). Keeping the resource in state lets the
	// user retry the destroy once connectivity/permissions are restored.
	var cleanupErr error

	if origData != nil {
		var origMethods []string
		if err := json.Unmarshal(origData, &origMethods); err != nil {
			// Corrupt private state: we cannot restore the original order, so
			// fall back to clearing the leaf entirely.
			tflog.Warn(ctx, "failed to deserialize original auth order from private state, falling back to delete",
				map[string]any{"error": err.Error()})
			if err := r.deleteAuthOrder(ctx); err != nil {
				cleanupErr = fmt.Errorf("failed deleting auth order: %w", err)
			}
		} else {
			if restoreErr := r.restoreAuthOrder(ctx, origMethods); restoreErr != nil {
				// Restore failed; fall back to clearing the leaf. Surface the
				// clear error if that also fails.
				tflog.Warn(ctx, "failed restoring original auth order, falling back to delete",
					map[string]any{"error": restoreErr.Error(), "methods": origMethods})
				if err := r.deleteAuthOrder(ctx); err != nil {
					cleanupErr = fmt.Errorf("failed restoring original auth order (%v) and fallback delete also failed: %w", restoreErr, err)
				}
			}
		}
	} else {
		// No original auth order saved (device had no auth_order configured
		// at Create time, or the resource predates snapshotting). Clear the
		// Terraform-managed auth_order to restore the unset baseline.
		tflog.Warn(ctx, "No original auth order in private state, falling back to delete")
		if err := r.deleteAuthOrder(ctx); err != nil {
			cleanupErr = fmt.Errorf("failed deleting auth order: %w", err)
		}
	}

	// If auth_order cleanup failed, surface it and abort the destroy so the
	// resource remains in state for a retry. Do not remove the resource.
	if cleanupErr != nil {
		resp.Diagnostics.AddError(
			"Failed to clean up auth_order during destroy",
			fmt.Sprintf("The device may still have the Terraform-managed authentication order applied. "+
				"The resource has been kept in state; re-run destroy once the device is reachable. Error: %v", cleanupErr),
		)
		return
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

	// Password policy is left as-is on destroy (no-op). The device always has
	// a password policy; reverting to weaker defaults would be a security risk.

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

// ---------------------------------------------------------------------------
// Password Policy helpers
// ---------------------------------------------------------------------------

// passwordPolicyAttrTypes returns the attr.Type map for the password_policy
// SingleNestedAttribute. Used when constructing types.ObjectValue.
func passwordPolicyAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"min_length":           types.Int64Type,
		"required_numeric":     types.Int64Type,
		"required_uppercase":   types.Int64Type,
		"required_lowercase":   types.Int64Type,
		"required_special":     types.Int64Type,
		"required_differences": types.Int64Type,
		"reject_username":      types.BoolType,
		"apply_to_root":        types.BoolType,
		"retries":              types.Int64Type,
		"max_login_failures":   types.Int64Type,
		"unlock_time":          types.Int64Type,
		"root_lockout":         types.BoolType,
		"root_unlock_time":     types.Int64Type,
		"max_age":              types.Int64Type,
		"max_letter_repeat":    types.Int64Type,
		"max_sequence_repeat":  types.Int64Type,
		"max_class_repeat":     types.Int64Type,
		"min_days":             types.Int64Type,
		"remember":             types.Int64Type,
		"warn_age":             types.Int64Type,
	}
}

// validateV17Fields checks whether the user configured v1.7+ fields on a
// device that doesn't support them. Returns diagnostics with errors if so.
func (r *AuthResource) validateV17Fields(ctx context.Context, pp *passwordPolicyModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if platformVersionAtLeast(r.client.PlatformVersion, "v1.7") {
		return diags
	}
	if !pp.MaxLetterRepeat.IsNull() && !pp.MaxLetterRepeat.IsUnknown() {
		diags.AddError("Unsupported attribute",
			"max_letter_repeat is not supported on F5OS versions below v1.7")
	}
	if !pp.MaxSequenceRepeat.IsNull() && !pp.MaxSequenceRepeat.IsUnknown() {
		diags.AddError("Unsupported attribute",
			"max_sequence_repeat is not supported on F5OS versions below v1.7")
	}
	if !pp.MaxClassRepeat.IsNull() && !pp.MaxClassRepeat.IsUnknown() {
		diags.AddError("Unsupported attribute",
			"max_class_repeat is not supported on F5OS versions below v1.7")
	}
	return diags
}

// validateV20Fields checks whether the user configured v2.0.0+ fields on a
// device that doesn't support them. Returns diagnostics with errors if so.
func (r *AuthResource) validateV20Fields(ctx context.Context, pp *passwordPolicyModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if platformVersionAtLeast(r.client.PlatformVersion, "v2.0") {
		return diags
	}
	if !pp.MinDays.IsNull() && !pp.MinDays.IsUnknown() {
		diags.AddError("Unsupported attribute",
			"min_days is not supported on F5OS versions below 2.0.0")
	}
	if !pp.Remember.IsNull() && !pp.Remember.IsUnknown() {
		diags.AddError("Unsupported attribute",
			"remember is not supported on F5OS versions below 2.0.0")
	}
	if !pp.WarnAge.IsNull() && !pp.WarnAge.IsUnknown() {
		diags.AddError("Unsupported attribute",
			"warn_age is not supported on F5OS versions below 2.0.0")
	}
	return diags
}

// readPasswordPolicy reads password policy from the device and refreshes
// the PasswordPolicy field in the model.
//
// When isImport is true, all fields are populated from the device (since
// there is no prior state). Otherwise, only fields already present in state
// are refreshed — this avoids adding fields the user didn't declare.
func (r *AuthResource) readPasswordPolicy(ctx context.Context, state *AuthResourceModel, isImport bool) diag.Diagnostics {
	var diags diag.Diagnostics
	policy, err := r.client.GetPasswordPolicy()
	if err != nil {
		diags.AddError("Failed to read password policy from device", err.Error())
		return diags
	}

	if isImport {
		// Import: populate all fields from device
		model := passwordPolicyConfigToModel(policy, r.client.PlatformVersion)
		obj, d := types.ObjectValueFrom(ctx, passwordPolicyAttrTypes(), model)
		diags.Append(d...)
		if !diags.HasError() {
			state.PasswordPolicy = obj
		}
		return diags
	}

	// Normal read: only refresh fields already in state
	var current passwordPolicyModel
	diags.Append(state.PasswordPolicy.As(ctx, &current, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return diags
	}

	if !current.MinLength.IsNull() && policy.MinLength != nil {
		current.MinLength = types.Int64Value(*policy.MinLength)
	}
	if !current.RequiredNumeric.IsNull() && policy.RequiredNumeric != nil {
		current.RequiredNumeric = types.Int64Value(*policy.RequiredNumeric)
	}
	if !current.RequiredUppercase.IsNull() && policy.RequiredUppercase != nil {
		current.RequiredUppercase = types.Int64Value(*policy.RequiredUppercase)
	}
	if !current.RequiredLowercase.IsNull() && policy.RequiredLowercase != nil {
		current.RequiredLowercase = types.Int64Value(*policy.RequiredLowercase)
	}
	if !current.RequiredSpecial.IsNull() && policy.RequiredSpecial != nil {
		current.RequiredSpecial = types.Int64Value(*policy.RequiredSpecial)
	}
	if !current.RequiredDifferences.IsNull() && policy.RequiredDifferences != nil {
		current.RequiredDifferences = types.Int64Value(*policy.RequiredDifferences)
	}
	if !current.RejectUsername.IsNull() && policy.RejectUsername != nil {
		current.RejectUsername = types.BoolValue(*policy.RejectUsername)
	}
	if !current.ApplyToRoot.IsNull() && policy.ApplyToRoot != nil {
		current.ApplyToRoot = types.BoolValue(*policy.ApplyToRoot)
	}
	if !current.Retries.IsNull() && policy.Retries != nil {
		current.Retries = types.Int64Value(*policy.Retries)
	}
	if !current.MaxLoginFailures.IsNull() && policy.MaxLoginFailures != nil {
		current.MaxLoginFailures = types.Int64Value(*policy.MaxLoginFailures)
	}
	if !current.UnlockTime.IsNull() && policy.UnlockTime != nil {
		current.UnlockTime = types.Int64Value(*policy.UnlockTime)
	}
	if !current.RootLockout.IsNull() && policy.RootLockout != nil {
		current.RootLockout = types.BoolValue(*policy.RootLockout)
	}
	if !current.RootUnlockTime.IsNull() && policy.RootUnlockTime != nil {
		current.RootUnlockTime = types.Int64Value(*policy.RootUnlockTime)
	}
	if !current.MaxAge.IsNull() && policy.MaxAge != nil {
		current.MaxAge = types.Int64Value(*policy.MaxAge)
	}
	if !current.MaxLetterRepeat.IsNull() && policy.MaxLetterRepeat != nil {
		current.MaxLetterRepeat = types.Int64Value(*policy.MaxLetterRepeat)
	}
	if !current.MaxSequenceRepeat.IsNull() && policy.MaxSequenceRepeat != nil {
		current.MaxSequenceRepeat = types.Int64Value(*policy.MaxSequenceRepeat)
	}
	if !current.MaxClassRepeat.IsNull() && policy.MaxClassRepeat != nil {
		current.MaxClassRepeat = types.Int64Value(*policy.MaxClassRepeat)
	}
	if !current.MinDays.IsNull() && policy.MinDays != nil {
		current.MinDays = types.Int64Value(*policy.MinDays)
	}
	if !current.Remember.IsNull() && policy.Remember != nil {
		current.Remember = types.Int64Value(*policy.Remember)
	}
	if !current.WarnAge.IsNull() && policy.WarnAge != nil {
		current.WarnAge = types.Int64Value(*policy.WarnAge)
	}

	obj, d := types.ObjectValueFrom(ctx, passwordPolicyAttrTypes(), current)
	diags.Append(d...)
	if !diags.HasError() {
		state.PasswordPolicy = obj
	}
	return diags
}

// writePasswordPolicy converts the Terraform model to an API config struct
// and sends it to the device via PATCH.
func (r *AuthResource) writePasswordPolicy(ctx context.Context, pp *passwordPolicyModel) diag.Diagnostics {
	var diags diag.Diagnostics
	config := passwordPolicyModelToConfig(pp, r.client.PlatformVersion)
	tflog.Debug(ctx, "Writing password policy to device")
	if err := r.client.SetPasswordPolicy(config); err != nil {
		diags.AddError("Failed to set password policy", err.Error())
	}
	return diags
}

// passwordPolicyModelToConfig converts a Terraform passwordPolicyModel to
// an f5osclient PasswordPolicyConfig struct. Only non-null fields are set.
// The deviceVersion parameter controls which version-specific fields are included.
func passwordPolicyModelToConfig(pp *passwordPolicyModel, deviceVersion string) *f5os.PasswordPolicyConfig {
	config := &f5os.PasswordPolicyConfig{}

	if !pp.MinLength.IsNull() && !pp.MinLength.IsUnknown() {
		v := pp.MinLength.ValueInt64()
		config.MinLength = &v
	}
	if !pp.RequiredNumeric.IsNull() && !pp.RequiredNumeric.IsUnknown() {
		v := pp.RequiredNumeric.ValueInt64()
		config.RequiredNumeric = &v
	}
	if !pp.RequiredUppercase.IsNull() && !pp.RequiredUppercase.IsUnknown() {
		v := pp.RequiredUppercase.ValueInt64()
		config.RequiredUppercase = &v
	}
	if !pp.RequiredLowercase.IsNull() && !pp.RequiredLowercase.IsUnknown() {
		v := pp.RequiredLowercase.ValueInt64()
		config.RequiredLowercase = &v
	}
	if !pp.RequiredSpecial.IsNull() && !pp.RequiredSpecial.IsUnknown() {
		v := pp.RequiredSpecial.ValueInt64()
		config.RequiredSpecial = &v
	}
	if !pp.RequiredDifferences.IsNull() && !pp.RequiredDifferences.IsUnknown() {
		v := pp.RequiredDifferences.ValueInt64()
		config.RequiredDifferences = &v
	}
	if !pp.RejectUsername.IsNull() && !pp.RejectUsername.IsUnknown() {
		v := pp.RejectUsername.ValueBool()
		config.RejectUsername = &v
	}
	if !pp.ApplyToRoot.IsNull() && !pp.ApplyToRoot.IsUnknown() {
		v := pp.ApplyToRoot.ValueBool()
		config.ApplyToRoot = &v
	}
	if !pp.Retries.IsNull() && !pp.Retries.IsUnknown() {
		v := pp.Retries.ValueInt64()
		config.Retries = &v
	}
	if !pp.MaxLoginFailures.IsNull() && !pp.MaxLoginFailures.IsUnknown() {
		v := pp.MaxLoginFailures.ValueInt64()
		config.MaxLoginFailures = &v
	}
	if !pp.UnlockTime.IsNull() && !pp.UnlockTime.IsUnknown() {
		v := pp.UnlockTime.ValueInt64()
		config.UnlockTime = &v
	}
	if !pp.RootLockout.IsNull() && !pp.RootLockout.IsUnknown() {
		v := pp.RootLockout.ValueBool()
		config.RootLockout = &v
	}
	if !pp.RootUnlockTime.IsNull() && !pp.RootUnlockTime.IsUnknown() {
		v := pp.RootUnlockTime.ValueInt64()
		config.RootUnlockTime = &v
	}
	if !pp.MaxAge.IsNull() && !pp.MaxAge.IsUnknown() {
		v := pp.MaxAge.ValueInt64()
		config.MaxAge = &v
	}

	// v1.7+ fields — only include if device supports them
	if platformVersionAtLeast(deviceVersion, "v1.7") {
		if !pp.MaxLetterRepeat.IsNull() && !pp.MaxLetterRepeat.IsUnknown() {
			v := pp.MaxLetterRepeat.ValueInt64()
			config.MaxLetterRepeat = &v
		}
		if !pp.MaxSequenceRepeat.IsNull() && !pp.MaxSequenceRepeat.IsUnknown() {
			v := pp.MaxSequenceRepeat.ValueInt64()
			config.MaxSequenceRepeat = &v
		}
		if !pp.MaxClassRepeat.IsNull() && !pp.MaxClassRepeat.IsUnknown() {
			v := pp.MaxClassRepeat.ValueInt64()
			config.MaxClassRepeat = &v
		}
	}

	// v2.0.0+ fields — only include if device supports them
	if platformVersionAtLeast(deviceVersion, "v2.0") {
		if !pp.MinDays.IsNull() && !pp.MinDays.IsUnknown() {
			v := pp.MinDays.ValueInt64()
			config.MinDays = &v
		}
		if !pp.Remember.IsNull() && !pp.Remember.IsUnknown() {
			v := pp.Remember.ValueInt64()
			config.Remember = &v
		}
		if !pp.WarnAge.IsNull() && !pp.WarnAge.IsUnknown() {
			v := pp.WarnAge.ValueInt64()
			config.WarnAge = &v
		}
	}

	return config
}

// passwordPolicyConfigToModel converts an f5osclient PasswordPolicyConfig
// to a Terraform passwordPolicyModel for populating state.
// The deviceVersion parameter controls which version-specific fields are populated.
func passwordPolicyConfigToModel(config *f5os.PasswordPolicyConfig, deviceVersion string) passwordPolicyModel {
	model := passwordPolicyModel{}

	if config.MinLength != nil {
		model.MinLength = types.Int64Value(*config.MinLength)
	} else {
		model.MinLength = types.Int64Null()
	}
	if config.RequiredNumeric != nil {
		model.RequiredNumeric = types.Int64Value(*config.RequiredNumeric)
	} else {
		model.RequiredNumeric = types.Int64Null()
	}
	if config.RequiredUppercase != nil {
		model.RequiredUppercase = types.Int64Value(*config.RequiredUppercase)
	} else {
		model.RequiredUppercase = types.Int64Null()
	}
	if config.RequiredLowercase != nil {
		model.RequiredLowercase = types.Int64Value(*config.RequiredLowercase)
	} else {
		model.RequiredLowercase = types.Int64Null()
	}
	if config.RequiredSpecial != nil {
		model.RequiredSpecial = types.Int64Value(*config.RequiredSpecial)
	} else {
		model.RequiredSpecial = types.Int64Null()
	}
	if config.RequiredDifferences != nil {
		model.RequiredDifferences = types.Int64Value(*config.RequiredDifferences)
	} else {
		model.RequiredDifferences = types.Int64Null()
	}
	if config.RejectUsername != nil {
		model.RejectUsername = types.BoolValue(*config.RejectUsername)
	} else {
		model.RejectUsername = types.BoolNull()
	}
	if config.ApplyToRoot != nil {
		model.ApplyToRoot = types.BoolValue(*config.ApplyToRoot)
	} else {
		model.ApplyToRoot = types.BoolNull()
	}
	if config.Retries != nil {
		model.Retries = types.Int64Value(*config.Retries)
	} else {
		model.Retries = types.Int64Null()
	}
	if config.MaxLoginFailures != nil {
		model.MaxLoginFailures = types.Int64Value(*config.MaxLoginFailures)
	} else {
		model.MaxLoginFailures = types.Int64Null()
	}
	if config.UnlockTime != nil {
		model.UnlockTime = types.Int64Value(*config.UnlockTime)
	} else {
		model.UnlockTime = types.Int64Null()
	}
	if config.RootLockout != nil {
		model.RootLockout = types.BoolValue(*config.RootLockout)
	} else {
		model.RootLockout = types.BoolNull()
	}
	if config.RootUnlockTime != nil {
		model.RootUnlockTime = types.Int64Value(*config.RootUnlockTime)
	} else {
		model.RootUnlockTime = types.Int64Null()
	}
	if config.MaxAge != nil {
		model.MaxAge = types.Int64Value(*config.MaxAge)
	} else {
		model.MaxAge = types.Int64Null()
	}

	// v1.7+ fields — only populate if device supports them
	if platformVersionAtLeast(deviceVersion, "v1.7") {
		if config.MaxLetterRepeat != nil {
			model.MaxLetterRepeat = types.Int64Value(*config.MaxLetterRepeat)
		} else {
			model.MaxLetterRepeat = types.Int64Null()
		}
		if config.MaxSequenceRepeat != nil {
			model.MaxSequenceRepeat = types.Int64Value(*config.MaxSequenceRepeat)
		} else {
			model.MaxSequenceRepeat = types.Int64Null()
		}
		if config.MaxClassRepeat != nil {
			model.MaxClassRepeat = types.Int64Value(*config.MaxClassRepeat)
		} else {
			model.MaxClassRepeat = types.Int64Null()
		}
	} else {
		// Pre-v1.7: these fields don't exist on the device
		model.MaxLetterRepeat = types.Int64Null()
		model.MaxSequenceRepeat = types.Int64Null()
		model.MaxClassRepeat = types.Int64Null()
	}

	// v2.0.0+ fields — only populate if device supports them
	if platformVersionAtLeast(deviceVersion, "v2.0") {
		if config.MinDays != nil {
			model.MinDays = types.Int64Value(*config.MinDays)
		} else {
			model.MinDays = types.Int64Null()
		}
		if config.Remember != nil {
			model.Remember = types.Int64Value(*config.Remember)
		} else {
			model.Remember = types.Int64Null()
		}
		if config.WarnAge != nil {
			model.WarnAge = types.Int64Value(*config.WarnAge)
		} else {
			model.WarnAge = types.Int64Null()
		}
	} else {
		// Pre-2.0.0: these fields don't exist on the device
		model.MinDays = types.Int64Null()
		model.Remember = types.Int64Null()
		model.WarnAge = types.Int64Null()
	}

	return model
}

// ---------------------------------------------------------------------------
// Login Policy helpers (F5OS 2.0.0+)
// ---------------------------------------------------------------------------

// loginPolicyAttrTypes returns the attr.Type map for the login_policy
// SingleNestedAttribute. Used when constructing types.ObjectValue.
func loginPolicyAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"admin_role_limit":           types.BoolType,
		"restconf_max_session_limit": types.Int64Type,
		"ssh_max_session_limit":      types.Int64Type,
	}
}

// writeLoginPolicy converts the Terraform model to an API config struct and
// sends it to the device via PATCH. Login policy is only available on F5OS
// 2.0.0+, so writing to an older device is rejected with a clear error.
func (r *AuthResource) writeLoginPolicy(ctx context.Context, lp *loginPolicyModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if !platformVersionAtLeast(r.client.PlatformVersion, "v2.0") {
		diags.AddError("Unsupported attribute",
			"login_policy is not supported on F5OS versions below 2.0.0")
		return diags
	}
	config := loginPolicyModelToConfig(lp)
	tflog.Debug(ctx, "Writing login policy to device")
	if err := r.client.SetLoginPolicy(config); err != nil {
		diags.AddError("Failed to set login policy", err.Error())
	}
	return diags
}

// readLoginPolicy reads login policy from the device and refreshes the
// LoginPolicy field in the model.
//
// When isImport is true, all fields are populated from the device (since there
// is no prior state). Otherwise, only fields already present in state are
// refreshed — this avoids adding fields the user didn't declare.
func (r *AuthResource) readLoginPolicy(ctx context.Context, state *AuthResourceModel, isImport bool) diag.Diagnostics {
	var diags diag.Diagnostics
	policy, err := r.client.GetLoginPolicy()
	if err != nil {
		diags.AddError("Failed to read login policy from device", err.Error())
		return diags
	}

	if isImport {
		model := loginPolicyConfigToModel(policy)
		obj, d := types.ObjectValueFrom(ctx, loginPolicyAttrTypes(), model)
		diags.Append(d...)
		if !diags.HasError() {
			state.LoginPolicy = obj
		}
		return diags
	}

	// Normal read: only refresh fields already in state.
	var current loginPolicyModel
	diags.Append(state.LoginPolicy.As(ctx, &current, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return diags
	}

	if !current.AdminRoleLimit.IsNull() && policy.AdminRoleLimit != nil {
		current.AdminRoleLimit = types.BoolValue(*policy.AdminRoleLimit)
	}
	if !current.RestconfMaxSessionLimit.IsNull() && policy.RestconfMaxSessionLimit != nil {
		current.RestconfMaxSessionLimit = types.Int64Value(*policy.RestconfMaxSessionLimit)
	}
	if !current.SSHMaxSessionLimit.IsNull() && policy.SSHMaxSessionLimit != nil {
		current.SSHMaxSessionLimit = types.Int64Value(*policy.SSHMaxSessionLimit)
	}

	obj, d := types.ObjectValueFrom(ctx, loginPolicyAttrTypes(), current)
	diags.Append(d...)
	if !diags.HasError() {
		state.LoginPolicy = obj
	}
	return diags
}

// loginPolicyModelToConfig converts a Terraform loginPolicyModel to an
// f5osclient LoginPolicyConfig struct. Only non-null fields are set.
func loginPolicyModelToConfig(lp *loginPolicyModel) *f5os.LoginPolicyConfig {
	config := &f5os.LoginPolicyConfig{}
	if !lp.AdminRoleLimit.IsNull() && !lp.AdminRoleLimit.IsUnknown() {
		v := lp.AdminRoleLimit.ValueBool()
		config.AdminRoleLimit = &v
	}
	if !lp.RestconfMaxSessionLimit.IsNull() && !lp.RestconfMaxSessionLimit.IsUnknown() {
		v := lp.RestconfMaxSessionLimit.ValueInt64()
		config.RestconfMaxSessionLimit = &v
	}
	if !lp.SSHMaxSessionLimit.IsNull() && !lp.SSHMaxSessionLimit.IsUnknown() {
		v := lp.SSHMaxSessionLimit.ValueInt64()
		config.SSHMaxSessionLimit = &v
	}
	return config
}

// loginPolicyConfigToModel converts an f5osclient LoginPolicyConfig to a
// Terraform loginPolicyModel for populating state.
func loginPolicyConfigToModel(config *f5os.LoginPolicyConfig) loginPolicyModel {
	model := loginPolicyModel{}
	if config.AdminRoleLimit != nil {
		model.AdminRoleLimit = types.BoolValue(*config.AdminRoleLimit)
	} else {
		model.AdminRoleLimit = types.BoolNull()
	}
	if config.RestconfMaxSessionLimit != nil {
		model.RestconfMaxSessionLimit = types.Int64Value(*config.RestconfMaxSessionLimit)
	} else {
		model.RestconfMaxSessionLimit = types.Int64Null()
	}
	if config.SSHMaxSessionLimit != nil {
		model.SSHMaxSessionLimit = types.Int64Value(*config.SSHMaxSessionLimit)
	} else {
		model.SSHMaxSessionLimit = types.Int64Null()
	}
	return model
}

// ---------------------------------------------------------------------------
// LDAP config helpers (F5OS 2.0.0+)
// ---------------------------------------------------------------------------

// ldapAttrTypes returns the attr.Type map for the ldap SingleNestedAttribute.
// Used when constructing types.ObjectValue.
func ldapAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"user_object_class":  types.ListType{ElemType: types.StringType},
		"group_object_class": types.ListType{ElemType: types.StringType},
	}
}

// writeLdapConfig converts the Terraform model to an API config struct and
// sends it to the device via PATCH. The LDAP object-class leaf-lists are only
// available on F5OS 2.0.0+, so writing to an older device is rejected with a
// clear error.
func (r *AuthResource) writeLdapConfig(ctx context.Context, lc *ldapConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if !platformVersionAtLeast(r.client.PlatformVersion, "v2.0") {
		diags.AddError("Unsupported attribute",
			"ldap configuration (user_object_class/group_object_class) is not supported on F5OS versions below 2.0.0")
		return diags
	}
	config, d := ldapModelToConfig(ctx, lc)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	tflog.Debug(ctx, "Writing ldap config to device")
	if err := r.client.SetLdapConfig(config); err != nil {
		diags.AddError("Failed to set ldap config", err.Error())
	}
	return diags
}

// readLdapConfig reads the LDAP container config from the device and refreshes
// the Ldap field in the model.
//
// When isImport is true, all fields are populated from the device (since there
// is no prior state). Otherwise, only fields already present in state are
// refreshed — this avoids adding fields the user didn't declare.
func (r *AuthResource) readLdapConfig(ctx context.Context, state *AuthResourceModel, isImport bool) diag.Diagnostics {
	var diags diag.Diagnostics
	config, err := r.client.GetLdapConfig()
	if err != nil {
		diags.AddError("Failed to read ldap config from device", err.Error())
		return diags
	}

	if isImport {
		model, d := ldapConfigToModel(ctx, config)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		obj, d := types.ObjectValueFrom(ctx, ldapAttrTypes(), model)
		diags.Append(d...)
		if !diags.HasError() {
			state.Ldap = obj
		}
		return diags
	}

	// Normal read: only refresh fields already in state.
	var current ldapConfigModel
	diags.Append(state.Ldap.As(ctx, &current, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return diags
	}

	// Refresh managed fields unconditionally from the device so out-of-band
	// drift is surfaced. Dropping the previous "config.X != nil" guard is
	// deliberate: if the device evicts the value (nil) while state still holds
	// a non-null list, we must write the device's value back so Terraform sees
	// the diff and re-applies. ListValueFrom preserves the nil/empty
	// distinction from GetLdapConfig: a nil slice becomes a null list
	// (leaf-list absent on device), a non-nil empty slice becomes an empty list
	// (leaf-list present but cleared).
	if !current.UserObjectClass.IsNull() {
		lv, d := types.ListValueFrom(ctx, types.StringType, config.UserObjectClass)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		current.UserObjectClass = lv
	}
	if !current.GroupObjectClass.IsNull() {
		lv, d := types.ListValueFrom(ctx, types.StringType, config.GroupObjectClass)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		current.GroupObjectClass = lv
	}

	obj, d := types.ObjectValueFrom(ctx, ldapAttrTypes(), current)
	diags.Append(d...)
	if !diags.HasError() {
		state.Ldap = obj
	}
	return diags
}

// ldapModelToConfig converts a Terraform ldapConfigModel to an f5osclient
// LdapConfig struct. Only non-null leaf-lists are set.
func ldapModelToConfig(ctx context.Context, lc *ldapConfigModel) (*f5os.LdapConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	config := &f5os.LdapConfig{}
	if !lc.UserObjectClass.IsNull() && !lc.UserObjectClass.IsUnknown() {
		var classes []string
		diags.Append(lc.UserObjectClass.ElementsAs(ctx, &classes, false)...)
		if diags.HasError() {
			return config, diags
		}
		config.UserObjectClass = classes
	}
	if !lc.GroupObjectClass.IsNull() && !lc.GroupObjectClass.IsUnknown() {
		var classes []string
		diags.Append(lc.GroupObjectClass.ElementsAs(ctx, &classes, false)...)
		if diags.HasError() {
			return config, diags
		}
		config.GroupObjectClass = classes
	}
	return config, diags
}

// ldapConfigToModel converts an f5osclient LdapConfig to a Terraform
// ldapConfigModel for populating state.
func ldapConfigToModel(ctx context.Context, config *f5os.LdapConfig) (ldapConfigModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := ldapConfigModel{
		UserObjectClass:  types.ListNull(types.StringType),
		GroupObjectClass: types.ListNull(types.StringType),
	}
	if config.UserObjectClass != nil {
		lv, d := types.ListValueFrom(ctx, types.StringType, config.UserObjectClass)
		diags.Append(d...)
		if diags.HasError() {
			return model, diags
		}
		model.UserObjectClass = lv
	}
	if config.GroupObjectClass != nil {
		lv, d := types.ListValueFrom(ctx, types.StringType, config.GroupObjectClass)
		diags.Append(d...)
		if diags.HasError() {
			return model, diags
		}
		model.GroupObjectClass = lv
	}
	return model, diags
}
