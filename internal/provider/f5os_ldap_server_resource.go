package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gitswarm.f5net.com/terraform-providers/f5osclient"
)

// Ensure the implementation satisfies the expected interfaces
var (
	_ resource.Resource                = &LdapServerResource{}
	_ resource.ResourceWithImportState = &LdapServerResource{}
)

// NewLdapServerResource is a helper function to simplify the provider implementation.
func NewLdapServerResource() resource.Resource {
	return &LdapServerResource{}
}

// LdapServerResource is the resource implementation.
type LdapServerResource struct {
	client *f5os.F5os
}

// LdapServerResourceModel describes the resource data model.
type LdapServerResourceModel struct {
	ID          types.String `tfsdk:"id"`
	ServerGroup types.String `tfsdk:"server_group"`
	Address     types.String `tfsdk:"address"`
	AuthPort    types.Int64  `tfsdk:"auth_port"`
	Type        types.String `tfsdk:"type"`
}

// Metadata returns the resource type name.
func (r *LdapServerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ldap_server"
}

// Schema defines the schema for the resource.
func (r *LdapServerResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an individual LDAP server within an LDAP-type server group on F5OS. " +
			"LDAP servers must be defined within a named server group before they can be referenced by authentication policies.",
		MarkdownDescription: "Manages an individual LDAP server within an LDAP-type server group on F5OS. " +
			"LDAP servers must be defined within a named server group before they can be referenced by authentication policies.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:         "Server group and address identifier (computed)",
				MarkdownDescription: "Server group and address identifier (computed)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"server_group": schema.StringAttribute{
				Description:         "Name of the LDAP-type server group containing this server.",
				MarkdownDescription: "Name of the LDAP-type server group containing this server.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"address": schema.StringAttribute{
				Description:         "IP address or hostname of the LDAP server.",
				MarkdownDescription: "IP address or hostname of the LDAP server.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"auth_port": schema.Int64Attribute{
				Description:         "LDAP server port (default: 389 for ldap, 636 for ldaps).",
				MarkdownDescription: "LDAP server port (default: 389 for ldap, 636 for ldaps).",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"type": schema.StringAttribute{
				Description:         "Connection type: 'ldap' or 'ldaps'.",
				MarkdownDescription: "Connection type: 'ldap' or 'ldaps'.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *LdapServerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*f5os.F5os)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *f5os.F5os, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

// Create creates the resource and sets the initial Terraform state.
func (r *LdapServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan LdapServerResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serverGroup := plan.ServerGroup.ValueString()
	address := plan.Address.ValueString()

	// Convert auth_port if provided
	var port *int64
	if !plan.AuthPort.IsNull() && !plan.AuthPort.IsUnknown() {
		p := plan.AuthPort.ValueInt64()
		port = &p
	}

	// Get server type (ldap or ldaps)
	serverType := plan.Type.ValueString()

	// Create the LDAP server
	err := r.client.CreateLdapServer(serverGroup, address, port, serverType)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating LDAP server",
			fmt.Sprintf("Could not create LDAP server %s in group %s: %s", address, serverGroup, err.Error()),
		)
		return
	}

	// Set the ID
	plan.ID = types.StringValue(fmt.Sprintf("%s:%s", serverGroup, address))

	tflog.Trace(ctx, fmt.Sprintf("Created LDAP server %s in group %s", address, serverGroup))

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *LdapServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state LdapServerResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serverGroup := state.ServerGroup.ValueString()
	address := state.Address.ValueString()

	// Get the LDAP server config
	serverConfig, err := r.client.GetLdapServer(serverGroup, address)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading LDAP server",
			fmt.Sprintf("Could not read LDAP server %s in group %s: %s", address, serverGroup, err.Error()),
		)
		return
	}

	// Update state with retrieved values
	state.Address = types.StringValue(serverConfig.Address)
	state.ID = types.StringValue(fmt.Sprintf("%s:%s", serverGroup, address))

	if serverConfig.AuthPort != nil {
		state.AuthPort = types.Int64Value(*serverConfig.AuthPort)
	} else {
		state.AuthPort = types.Int64Null()
	}

	if serverConfig.Type != "" {
		state.Type = types.StringValue(serverConfig.Type)
	} else {
		state.Type = types.StringNull()
	}

	tflog.Trace(ctx, fmt.Sprintf("Read LDAP server %s from group %s", address, serverGroup))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state.
func (r *LdapServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan LdapServerResourceModel
	var state LdapServerResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serverGroup := plan.ServerGroup.ValueString()
	address := plan.Address.ValueString()

	// Convert auth_port if provided
	var port *int64
	if !plan.AuthPort.IsNull() && !plan.AuthPort.IsUnknown() {
		p := plan.AuthPort.ValueInt64()
		port = &p
	}

	// Get server type (ldap or ldaps)
	serverType := plan.Type.ValueString()

	// Update the LDAP server
	err := r.client.UpdateLdapServer(serverGroup, address, port, serverType)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating LDAP server",
			fmt.Sprintf("Could not update LDAP server %s in group %s: %s", address, serverGroup, err.Error()),
		)
		return
	}

	// Set the ID
	plan.ID = types.StringValue(fmt.Sprintf("%s:%s", serverGroup, address))

	tflog.Trace(ctx, fmt.Sprintf("Updated LDAP server %s in group %s", address, serverGroup))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the resource and removes the Terraform state.
func (r *LdapServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state LdapServerResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serverGroup := state.ServerGroup.ValueString()
	address := state.Address.ValueString()

	// Delete the LDAP server
	err := r.client.DeleteLdapServer(serverGroup, address)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting LDAP server",
			fmt.Sprintf("Could not delete LDAP server %s in group %s: %s", address, serverGroup, err.Error()),
		)
		return
	}

	tflog.Trace(ctx, fmt.Sprintf("Deleted LDAP server %s from group %s", address, serverGroup))
}

// ImportState imports the resource state.
func (r *LdapServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Parse the import ID as "server_group:address"
	// For now, we'll require the full ID format
	// In a more advanced implementation, you could parse more flexibly

	var state LdapServerResourceModel
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			fmt.Sprintf("Expected 'server_group:address', got '%s'", req.ID),
		)
		return
	}
	state.ServerGroup = types.StringValue(parts[0])
	state.Address = types.StringValue(parts[1])

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
