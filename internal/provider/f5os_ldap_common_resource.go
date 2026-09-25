package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitswarm.f5net.com/terraform-providers/f5osclient"
)

var (
	_ resource.Resource                = &LdapCommonResource{}
	_ resource.ResourceWithImportState = &LdapCommonResource{}
)

func NewLdapCommonResource() resource.Resource {
	return &LdapCommonResource{}
}

type LdapCommonResource struct {
	client *f5os.F5os
}

type LdapCommonResourceModel struct {
	ID               types.String `tfsdk:"id"`
	BaseDN           types.String `tfsdk:"base_dn"`
	BindDN           types.String `tfsdk:"bind_dn"`
	BindPW           types.String `tfsdk:"bind_pw"`
	BindTimeout      types.Int64  `tfsdk:"bind_timeout"`
	ReadTimeout      types.Int64  `tfsdk:"read_timeout"`
	IdleTimeout      types.Int64  `tfsdk:"idle_timeout"`
	LDAPVersion      types.Int64  `tfsdk:"ldap_version"`
	ChaseReferrals   types.Bool   `tfsdk:"chase_referrals"`
	SSL              types.Bool   `tfsdk:"ssl"`
	ActiveDirectory  types.Bool   `tfsdk:"active_directory"`
	UserObjectClass  types.List   `tfsdk:"user_object_class"`
	GroupObjectClass types.List   `tfsdk:"group_object_class"`
	UnixAttributes   types.Bool   `tfsdk:"unix_attributes"`
	IgnoreCase       types.Bool   `tfsdk:"ignore_case"`
}

func (r *LdapCommonResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ldap_common"
}

func (r *LdapCommonResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages LDAP common configuration.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"base_dn":          schema.StringAttribute{Optional: true},
			"bind_dn":          schema.StringAttribute{Optional: true},
			"bind_pw":          schema.StringAttribute{Optional: true, Sensitive: true},
			"bind_timeout":     schema.Int64Attribute{Optional: true},
			"read_timeout":     schema.Int64Attribute{Optional: true},
			"idle_timeout":     schema.Int64Attribute{Optional: true},
			"ldap_version":     schema.Int64Attribute{Optional: true},
			"chase_referrals":  schema.BoolAttribute{Optional: true},
			"ssl":              schema.BoolAttribute{Optional: true},
			"active_directory": schema.BoolAttribute{Optional: true},
			"user_object_class": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			"group_object_class": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			"unix_attributes": schema.BoolAttribute{Optional: true},
			"ignore_case":     schema.BoolAttribute{Optional: true},
		},
	}
}

func (r *LdapCommonResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*f5os.F5os)
}

func (r *LdapCommonResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan LdapCommonResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Simplified update logic to just reuse Create
	// Create() takes (ctx, CreateRequest, *CreateResponse)
	// But Update takes (ctx, UpdateRequest, *UpdateResponse)
	// I cannot directly call Create.
	// Implementing Update logic.

	// Re-implement or factor out creation logic.
	r.applyConfig(ctx, &plan, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LdapCommonResource) applyConfig(ctx context.Context, plan *LdapCommonResourceModel, diags *diag.Diagnostics) {
	config := &f5os.LdapConfig{}

	// Convert string to interface{} for the F5OS client
	if !plan.BaseDN.IsNull() {
		v := plan.BaseDN.ValueString()
		config.BaseDN = v
	}
	if !plan.BindDN.IsNull() {
		v := plan.BindDN.ValueString()
		config.BindDN = v
	}
	if !plan.BindPW.IsNull() {
		v := plan.BindPW.ValueString()
		config.BindPW = v
	}
	if !plan.BindTimeout.IsNull() {
		v := plan.BindTimeout.ValueInt64()
		config.BindTimelimit = v
	}
	if !plan.IdleTimeout.IsNull() {
		v := plan.IdleTimeout.ValueInt64()
		config.IdleTimelimit = v
	}
	if !plan.ReadTimeout.IsNull() {
		v := plan.ReadTimeout.ValueInt64()
		config.Timelimit = v
	}
	if !plan.LDAPVersion.IsNull() {
		v := plan.LDAPVersion.ValueInt64()
		config.LDAPVersion = v
	}
	if !plan.ChaseReferrals.IsNull() {
		v := plan.ChaseReferrals.ValueBool()
		config.ChaseReferrals = v
	}
	if !plan.SSL.IsNull() {
		v := plan.SSL.ValueBool()
		if v {
			config.SSL = &v
		} else {
			config.SSL = nil
		}
	}

	if !plan.ActiveDirectory.IsNull() {
		v := plan.ActiveDirectory.ValueBool()
		config.ActiveDirectory = v
	}
	// Version check: only set fields if device is 2.0.0 or greater
	if r.client.PlatformVersion >= "2.0.0" {
		if !plan.UserObjectClass.IsNull() {
			var userClasses []string
			*diags = append(*diags, plan.UserObjectClass.ElementsAs(ctx, &userClasses, false)...)
			config.UserObjectClass = userClasses
		}
		if !plan.GroupObjectClass.IsNull() {
			var groupClasses []string
			*diags = append(*diags, plan.GroupObjectClass.ElementsAs(ctx, &groupClasses, false)...)
			config.GroupObjectClass = groupClasses
		}
	}
	if !plan.UnixAttributes.IsNull() {
		v := plan.UnixAttributes.ValueBool()
		config.UnixAttributes = v
	}
	if !plan.IgnoreCase.IsNull() {
		v := plan.IgnoreCase.ValueBool()
		config.IgnoreCase = v
	}

	if err := r.client.SetLdapConfig(config); err != nil {
		diags.AddError("Error setting LDAP common config", err.Error())
	}
}

func (r *LdapCommonResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan LdapCommonResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyConfig(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = types.StringValue("ldap-common")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LdapCommonResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state LdapCommonResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.GetLdapConfig()
	if err != nil {
		resp.Diagnostics.AddError("Error reading LDAP common config", err.Error())
		return
	}

	// Set state only if value is present in API response
	if v, ok := config.BaseDN.(string); ok {
		state.BaseDN = types.StringValue(v)
	}
	if v, ok := config.BindDN.(string); ok {
		state.BindDN = types.StringValue(v)
	}
	if v, ok := config.BindTimelimit.(float64); ok {
		state.BindTimeout = types.Int64Value(int64(v))
	}
	if v, ok := config.IdleTimelimit.(float64); ok {
		state.IdleTimeout = types.Int64Value(int64(v))
	}
	if v, ok := config.Timelimit.(float64); ok {
		state.ReadTimeout = types.Int64Value(int64(v))
	}
	if v, ok := config.LDAPVersion.(float64); ok {
		state.LDAPVersion = types.Int64Value(int64(v))
	}
	if v, ok := config.ChaseReferrals.(bool); ok {
		state.ChaseReferrals = types.BoolValue(v)
	}
	if v, ok := config.SSL.(bool); ok {
		state.SSL = types.BoolValue(v)
	}
	if v, ok := config.ActiveDirectory.(bool); ok {
		state.ActiveDirectory = types.BoolValue(v)
	}
	if v, ok := config.UnixAttributes.(bool); ok {
		state.UnixAttributes = types.BoolValue(v)
	}
	if v, ok := config.IgnoreCase.(bool); ok {
		state.IgnoreCase = types.BoolValue(v)
	}

	if config.UserObjectClass != nil {
		state.UserObjectClass, _ = types.ListValueFrom(ctx, types.StringType, config.UserObjectClass)
	}
	if config.GroupObjectClass != nil {
		state.GroupObjectClass, _ = types.ListValueFrom(ctx, types.StringType, config.GroupObjectClass)
	}

	state.ID = types.StringValue("ldap-common")
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LdapCommonResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *LdapCommonResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.State.RemoveResource(ctx)
}
