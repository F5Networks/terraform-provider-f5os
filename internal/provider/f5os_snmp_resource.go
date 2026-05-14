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
	GetSnmpConfig() ([]byte, error)
	GetSnmpMib() ([]byte, error)
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
func (a *f5osSnmpClient) GetSnmpConfig() ([]byte, error)        { return a.c.GetSnmpConfig() }
func (a *f5osSnmpClient) GetSnmpMib() ([]byte, error)           { return a.c.GetSnmpMib() }

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
			"Due to API restrictions, passwords cannot be retrieved which may lead to Terraform detecting changes on every plan.\n\n" +
			"~> **NOTE:** Running `terraform destroy` will reset MIB fields (sysContact, sysLocation, sysName) to their defaults and remove the resource from Terraform state.",
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
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"privacy_proto": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Privacy protocol. Valid options: `aes`, `des`. Requires authentication to be configured.",
						},
						"privacy_passwd": schema.StringAttribute{
							Optional:            true,
							Sensitive:           true,
							MarkdownDescription: "Password for encryption.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
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

// Read refreshes the Terraform state with the latest data from the device.
func (r *SnmpResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SnmpResourceModel

	// Load current state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Reading SNMP configuration from F5OS")

	// Always use the static resource ID and present state.
	state.Id = types.StringValue("snmp_config")
	state.State = types.StringValue("present")

	// ------------------------------------------------------------------
	// 1. Read SNMP communities, targets, and users from the device.
	// ------------------------------------------------------------------
	snmpData, err := r.client.GetSnmpConfig()
	if err != nil {
		resp.Diagnostics.AddError("SNMP Read Error", fmt.Sprintf("Failed to read SNMP config from device: %s", err))
		return
	}

	var snmpEnvelope struct {
		SNMP struct {
			Communities struct {
				Community []struct {
					Name   string `json:"name"`
					Config struct {
						Name          string   `json:"name"`
						SecurityModel []string `json:"security-model"`
					} `json:"config"`
				} `json:"community"`
			} `json:"communities"`
			Targets struct {
				Target []struct {
					Name   string `json:"name"`
					Config struct {
						Name          string `json:"name"`
						SecurityModel string `json:"security-model"`
						Community     string `json:"community"`
						User          string `json:"user"`
						IPv4          *struct {
							Address string `json:"address"`
							Port    int64  `json:"port"`
						} `json:"ipv4"`
						IPv6 *struct {
							Address string `json:"address"`
							Port    int64  `json:"port"`
						} `json:"ipv6"`
					} `json:"config"`
				} `json:"target"`
			} `json:"targets"`
			Users struct {
				User []struct {
					Name   string `json:"name"`
					Config struct {
						Name                   string `json:"name"`
						AuthenticationProtocol string `json:"authentication-protocol"`
						PrivacyProtocol        string `json:"privacy-protocol"`
					} `json:"config"`
				} `json:"user"`
			} `json:"users"`
		} `json:"f5-system-snmp:snmp"`
	}

	if err := json.Unmarshal(snmpData, &snmpEnvelope); err != nil {
		resp.Diagnostics.AddError("SNMP Parse Error", fmt.Sprintf("Failed to parse SNMP config JSON: %s", err))
		return
	}

	// Build lookups from the prior state so we know which entries are managed
	// by this resource and can preserve sensitive fields the API won't return.
	oldCommunities, diags := extractSnmpCommunities(ctx, state.SnmpCommunity)
	resp.Diagnostics.Append(diags...)
	oldTargets, diags := extractSnmpTargets(ctx, state.SnmpTarget)
	resp.Diagnostics.Append(diags...)
	oldUsers, diags := extractSnmpUsers(ctx, state.SnmpUser)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// When the prior state has entries, Read should only track the entries
	// that Terraform manages (i.e., those names present in state).
	// When the prior state is empty/null (e.g., after import), Read imports
	// everything from the device.
	managedCommunities := make(map[string]bool)
	for _, c := range oldCommunities {
		managedCommunities[c.Name.ValueString()] = true
	}
	managedTargets := make(map[string]bool)
	for _, t := range oldTargets {
		managedTargets[t.Name.ValueString()] = true
	}
	managedUsers := make(map[string]bool)
	for _, u := range oldUsers {
		managedUsers[u.Name.ValueString()] = true
	}
	userPasswords := make(map[string][2]types.String) // name -> {auth_passwd, privacy_passwd}
	for _, u := range oldUsers {
		userPasswords[u.Name.ValueString()] = [2]types.String{u.AuthPasswd, u.PrivacyPasswd}
	}

	filterCommunities := len(managedCommunities) > 0
	filterTargets := len(managedTargets) > 0
	filterUsers := len(managedUsers) > 0

	// --- Communities ---
	snmpCommunities := snmpEnvelope.SNMP.Communities.Community
	{
		communityObjects := make([]SnmpCommunityModel, 0, len(snmpCommunities))
		for _, c := range snmpCommunities {
			if filterCommunities && !managedCommunities[c.Config.Name] {
				continue
			}
			smVals := make([]attr.Value, 0, len(c.Config.SecurityModel))
			for _, sm := range c.Config.SecurityModel {
				smVals = append(smVals, types.StringValue(sm))
			}
			smList, listDiags := types.ListValue(types.StringType, smVals)
			resp.Diagnostics.Append(listDiags...)
			communityObjects = append(communityObjects, SnmpCommunityModel{
				Name:          types.StringValue(c.Config.Name),
				SecurityModel: smList,
			})
		}
		if len(communityObjects) > 0 {
			communityList, listDiags := types.ListValueFrom(ctx, state.SnmpCommunity.ElementType(ctx), communityObjects)
			resp.Diagnostics.Append(listDiags...)
			state.SnmpCommunity = communityList
		} else if !state.SnmpCommunity.IsNull() {
			state.SnmpCommunity = types.ListNull(state.SnmpCommunity.ElementType(ctx))
		}
	}

	// --- Targets ---
	snmpTargets := snmpEnvelope.SNMP.Targets.Target
	{
		targetObjects := make([]SnmpTargetModel, 0, len(snmpTargets))
		for _, t := range snmpTargets {
			if filterTargets && !managedTargets[t.Config.Name] {
				continue
			}
			tm := SnmpTargetModel{
				Name: types.StringValue(t.Config.Name),
			}
			if t.Config.SecurityModel != "" {
				tm.SecurityModel = types.StringValue(t.Config.SecurityModel)
			} else {
				tm.SecurityModel = types.StringNull()
			}
			if t.Config.Community != "" {
				tm.Community = types.StringValue(t.Config.Community)
			} else {
				tm.Community = types.StringNull()
			}
			if t.Config.User != "" {
				tm.User = types.StringValue(t.Config.User)
			} else {
				tm.User = types.StringNull()
			}
			switch {
			case t.Config.IPv4 != nil:
				tm.Ipv4Address = types.StringValue(t.Config.IPv4.Address)
				tm.Port = types.Int64Value(t.Config.IPv4.Port)
				tm.Ipv6Address = types.StringNull()
			case t.Config.IPv6 != nil:
				tm.Ipv6Address = types.StringValue(t.Config.IPv6.Address)
				tm.Port = types.Int64Value(t.Config.IPv6.Port)
				tm.Ipv4Address = types.StringNull()
			default:
				tm.Ipv4Address = types.StringNull()
				tm.Ipv6Address = types.StringNull()
				tm.Port = types.Int64Value(0)
			}
			targetObjects = append(targetObjects, tm)
		}
		if len(targetObjects) > 0 {
			targetList, listDiags := types.ListValueFrom(ctx, state.SnmpTarget.ElementType(ctx), targetObjects)
			resp.Diagnostics.Append(listDiags...)
			state.SnmpTarget = targetList
		} else if !state.SnmpTarget.IsNull() {
			state.SnmpTarget = types.ListNull(state.SnmpTarget.ElementType(ctx))
		}
	}

	// --- Users ---
	snmpUsers := snmpEnvelope.SNMP.Users.User
	{
		userObjects := make([]SnmpUserModel, 0, len(snmpUsers))
		for _, u := range snmpUsers {
			if filterUsers && !managedUsers[u.Config.Name] {
				continue
			}
			um := SnmpUserModel{
				Name: types.StringValue(u.Config.Name),
			}
			if u.Config.AuthenticationProtocol != "" {
				um.AuthProto = types.StringValue(u.Config.AuthenticationProtocol)
			} else {
				um.AuthProto = types.StringNull()
			}
			if u.Config.PrivacyProtocol != "" {
				um.PrivacyProto = types.StringValue(u.Config.PrivacyProtocol)
			} else {
				um.PrivacyProto = types.StringNull()
			}
			// Passwords are never returned by the API. Preserve prior state values.
			if pw, ok := userPasswords[u.Config.Name]; ok {
				um.AuthPasswd = pw[0]
				um.PrivacyPasswd = pw[1]
			} else {
				um.AuthPasswd = types.StringNull()
				um.PrivacyPasswd = types.StringNull()
			}
			userObjects = append(userObjects, um)
		}
		if len(userObjects) > 0 {
			userList, listDiags := types.ListValueFrom(ctx, state.SnmpUser.ElementType(ctx), userObjects)
			resp.Diagnostics.Append(listDiags...)
			state.SnmpUser = userList
		} else if !state.SnmpUser.IsNull() {
			state.SnmpUser = types.ListNull(state.SnmpUser.ElementType(ctx))
		}
	}

	// ------------------------------------------------------------------
	// 2. Read MIB settings from the device.
	// Only populate snmp_mib in state if the user is managing it (i.e.,
	// the prior state had snmp_mib set). If snmp_mib was null, the user
	// did not declare it and we should not pull device-global MIB values
	// into state — that would cause spurious diffs.
	// ------------------------------------------------------------------
	mibWasManaged := !state.SnmpMib.IsNull()

	mibData, err := r.client.GetSnmpMib()
	if err != nil {
		// MIB read failure is non-fatal; leave snmp_mib unchanged.
		tflog.Warn(ctx, "Failed to read SNMP MIB from device", map[string]interface{}{"error": err.Error()})
	} else if mibWasManaged {
		var mibEnvelope struct {
			System struct {
				SysName     string `json:"sysName"`
				SysContact  string `json:"sysContact"`
				SysLocation string `json:"sysLocation"`
			} `json:"SNMPv2-MIB:system"`
		}
		if err := json.Unmarshal(mibData, &mibEnvelope); err != nil {
			tflog.Warn(ctx, "Failed to parse SNMP MIB JSON", map[string]interface{}{"error": err.Error()})
		} else {
			mibModel := SnmpMibModel{
				SysName:     types.StringValue(mibEnvelope.System.SysName),
				SysContact:  types.StringValue(mibEnvelope.System.SysContact),
				SysLocation: types.StringValue(mibEnvelope.System.SysLocation),
			}
			mibObj, objDiags := types.ObjectValueFrom(ctx, map[string]attr.Type{
				"sysname":     types.StringType,
				"syscontact":  types.StringType,
				"syslocation": types.StringType,
			}, mibModel)
			resp.Diagnostics.Append(objDiags...)
			state.SnmpMib = mibObj
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "SNMP configuration read completed", map[string]interface{}{
		"id":    state.Id.ValueString(),
		"state": state.State.ValueString(),
	})

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
		if err := r.deleteSnmpConfig(ctx, communities, targets, users, mib != nil); err != nil {
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

	// Only reset MIB fields if the user actually managed them.
	resetMib := !state.SnmpMib.IsNull()

	// Delete SNMP configuration
	if err := r.deleteSnmpConfig(ctx, communities, targets, users, resetMib); err != nil {
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

// deleteSnmpConfig handles the deletion of SNMP configuration.
// When resetMib is true the MIB sysName/sysContact/sysLocation fields are
// reset to empty strings on the device. Pass false when the user never
// declared snmp_mib so we don't wipe device-level values they don't own.
func (r *SnmpResource) deleteSnmpConfig(ctx context.Context, communities []SnmpCommunityModel, targets []SnmpTargetModel, users []SnmpUserModel, resetMib bool) error {
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

	// Only reset MIB fields when the user declared snmp_mib in their config.
	// Otherwise we would wipe device-level values the user never claimed.
	if resetMib {
		emptyMib := map[string]interface{}{
			"SNMPv2-MIB:system": map[string]interface{}{
				"SNMPv2-MIB:sysContact":  "",
				"SNMPv2-MIB:sysName":     "",
				"SNMPv2-MIB:sysLocation": "",
			},
		}
		mibBytes, err := json.Marshal(emptyMib)
		if err != nil {
			tflog.Warn(ctx, "Failed to marshal empty MIB payload", map[string]interface{}{"error": err.Error()})
		} else if err := r.client.UpdateSnmpMib(mibBytes); err != nil {
			tflog.Warn(ctx, "Failed to reset SNMP MIB on delete", map[string]interface{}{"error": err.Error()})
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
