package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5os "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// emptyListWhenConfigNull is a plan modifier for the Optional+Computed
// dns_domains attribute. For an Optional+Computed list, when the user removes
// the attribute from configuration the framework's default behavior is to
// preserve the prior state value — which means removing all search domains
// from the config would never plan a change, and Update would never run to
// delete them from the device. This modifier forces the planned value to an
// empty list when the config value is null (and prior state was non-null), so
// removing domains is correctly planned and applied. When the config is set,
// the configured value is used as normal.
type emptyListWhenConfigNull struct{}

func (m emptyListWhenConfigNull) Description(_ context.Context) string {
	return "When the attribute is omitted from configuration, plan it as an empty list instead of preserving the prior state value."
}

func (m emptyListWhenConfigNull) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m emptyListWhenConfigNull) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	// Only act when the config omits the attribute. If the user set a value
	// (including an explicit empty list), respect it.
	if !req.ConfigValue.IsNull() {
		return
	}
	// On create (no prior state) leave the plan as-is; Create normalizes a
	// null value to an empty list in state.
	if req.StateValue.IsNull() {
		return
	}
	resp.PlanValue = types.ListValueMust(types.StringType, []attr.Value{})
}

// recomputeIDOnChange is a plan modifier for the Computed id attribute. The id
// is a content hash derived from dns_servers and dns_domains. By default a
// Computed attribute carries its prior state value into the plan, but since the
// id changes whenever servers/domains change, Update would then produce an id
// that differs from the planned value and Terraform would report "provider
// produced inconsistent result after apply". This modifier marks id as unknown
// whenever the planned servers/domains differ from state, so Update can set the
// recomputed value.
type recomputeIDOnChange struct{}

func (m recomputeIDOnChange) Description(_ context.Context) string {
	return "Recompute id (mark unknown) when dns_servers or dns_domains change."
}

func (m recomputeIDOnChange) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m recomputeIDOnChange) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// On create the id is already unknown; nothing to do.
	if req.StateValue.IsNull() {
		return
	}

	// Compare the user's configuration to prior state. Using the config (not
	// the plan) avoids depending on plan-modifier execution order for the
	// dns_domains attribute. A null domains config means "no domains", which
	// is equivalent to an empty list in state.
	var cfgServers, cfgDomains, stateServers, stateDomains types.List
	req.Config.GetAttribute(ctx, path.Root("dns_servers"), &cfgServers)
	req.Config.GetAttribute(ctx, path.Root("dns_domains"), &cfgDomains)
	req.State.GetAttribute(ctx, path.Root("dns_servers"), &stateServers)
	req.State.GetAttribute(ctx, path.Root("dns_domains"), &stateDomains)

	// Normalize a null domains config to an empty list for comparison.
	if cfgDomains.IsNull() {
		cfgDomains = types.ListValueMust(types.StringType, []attr.Value{})
	}
	stateDomainsNorm := stateDomains
	if stateDomainsNorm.IsNull() {
		stateDomainsNorm = types.ListValueMust(types.StringType, []attr.Value{})
	}

	if !cfgServers.Equal(stateServers) || !cfgDomains.Equal(stateDomainsNorm) {
		resp.PlanValue = types.StringUnknown()
	}
}

// Ensure the implementation satisfies the expected interfaces
var (
	_ resource.Resource              = &DNSResource{}
	_ resource.ResourceWithConfigure = &DNSResource{}
)

// DNSResourceModel represents the schema model
type DNSResourceModel struct {
	Id         types.String `tfsdk:"id"`
	DNSServers types.List   `tfsdk:"dns_servers"`
	DNSDomains types.List   `tfsdk:"dns_domains"`
}

// DNSResource defines the resource implementation
type DNSResource struct {
	client *f5os.F5os
}

// NewDNSResource creates a new instance of the resource
func NewDNSResource() resource.Resource {
	return &DNSResource{}
}

// Metadata returns the resource type name
func (r *DNSResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns"
}

// Schema defines the schema for the resource
func (r *DNSResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Resource used to configure DNS settings (servers and domains) on F5OS systems (VELOS or rSeries).\n\n" +
			"~> **NOTE:** The `f5os_dns` resource updates DNS servers and search domains on the F5OS platforms using Open API. " +
			"When updating, any servers or domains removed from the configuration are also deleted from the device before the new values are applied.\n\n" +
			"~> **IMPORTANT:** Running `terraform destroy` will remove this resource from Terraform state but will **not** delete the DNS configuration from the device. DNS is a critical system service and removing it could make the device unreachable.",
		Attributes: map[string]schema.Attribute{
			"dns_servers": schema.ListAttribute{
				ElementType:         types.StringType,
				Required:            true,
				MarkdownDescription: "List of DNS server IP addresses. Example: `[\"8.8.8.8\", \"1.1.1.1\"]`",
			},
			"dns_domains": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "List of DNS search domains. Example: `[\"internal.domain\"]`",
				PlanModifiers: []planmodifier.List{
					emptyListWhenConfigNull{},
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for the resource, computed from the DNS server configuration.",
				PlanModifiers: []planmodifier.String{
					recomputeIDOnChange{},
				},
			},
		},
	}
}

// Configure configures the resource with the provider client
func (r *DNSResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = client
}

// extractStringList safely extracts a string list from a types.List.
// Always returns a non-nil slice so callers never send JSON null to the
// API and types.ListValueFrom never produces a null Terraform list.
func extractStringList(ctx context.Context, list types.List) ([]string, diag.Diagnostics) {
	var result []string
	var diags diag.Diagnostics

	if !list.IsNull() && !list.IsUnknown() {
		diags = list.ElementsAs(ctx, &result, false)
	}

	if result == nil {
		result = []string{}
	}

	return result, diags
}

// computeResourceID generates a hash-based ID from DNS servers and domains
func computeResourceID(servers []string, domains []string) string {
	// Sort the slices to ensure consistent hash generation
	sortedServers := make([]string, len(servers))
	copy(sortedServers, servers)
	sort.Strings(sortedServers)

	sortedDomains := make([]string, len(domains))
	copy(sortedDomains, domains)
	sort.Strings(sortedDomains)

	// Create a string representation of the configuration
	configStr := fmt.Sprintf("servers:%s;domains:%s",
		strings.Join(sortedServers, ","),
		strings.Join(sortedDomains, ","))

	// Generate SHA-256 hash
	hash := sha256.Sum256([]byte(configStr))
	return hex.EncodeToString(hash[:])
}

// Create creates the resource and sets the initial Terraform state
func (r *DNSResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DNSResourceModel

	// Deserialize plan into Go struct
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Creating DNS configuration")

	// Extract dns_servers and dns_domains
	dnsServers, diags := extractStringList(ctx, plan.DNSServers)
	resp.Diagnostics.Append(diags...)

	dnsDomains, diags := extractStringList(ctx, plan.DNSDomains)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read the current device state so we can remove pre-existing entries
	// that are not in the Terraform config. The F5OS DNS PATCH API is
	// additive — it merges with existing config rather than replacing it.
	// Without this step, pre-existing servers/domains would persist on the
	// device and cause drift on the next refresh.
	existing, err := r.client.ReadDNSConfig()
	if err != nil {
		resp.Diagnostics.AddError(
			"DNS Configuration Error",
			fmt.Sprintf("Failed to read existing DNS configuration: %s", err),
		)
		return
	}

	var existingServers, existingDomains []string
	for _, s := range existing.DNS.Servers.Server {
		existingServers = append(existingServers, s.Address)
	}
	existingDomains = append(existingDomains, existing.DNS.Config.Search...)

	staleServers := removedEntries(existingServers, dnsServers)
	staleDomains := removedEntries(existingDomains, dnsDomains)
	if len(staleServers) > 0 || len(staleDomains) > 0 {
		tflog.Debug(ctx, "Removing pre-existing DNS entries not in config", map[string]interface{}{
			"stale_servers": staleServers,
			"stale_domains": staleDomains,
		})
		if err := r.client.DeleteDNSConfig(staleServers, staleDomains); err != nil {
			resp.Diagnostics.AddError(
				"DNS Configuration Error",
				fmt.Sprintf("Failed to remove pre-existing DNS entries: %s", err),
			)
			return
		}
	}

	// Call client to PATCH DNS config
	if err := r.client.PatchDNSConfig(dnsServers, dnsDomains); err != nil {
		resp.Diagnostics.AddError(
			"DNS Configuration Error",
			fmt.Sprintf("Failed to configure DNS: %s", err),
		)
		return
	}

	// Compute resource ID based on configuration
	resourceID := computeResourceID(dnsServers, dnsDomains)
	plan.Id = types.StringValue(resourceID)

	// Ensure dns_domains is never null in state — when omitted from the
	// HCL config, the plan value is null, but Computed attributes must
	// be known after apply. Set it to an empty list.
	if plan.DNSDomains.IsNull() || plan.DNSDomains.IsUnknown() {
		plan.DNSDomains, diags = types.ListValueFrom(ctx, types.StringType, dnsDomains)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	tflog.Debug(ctx, "DNS configuration created successfully", map[string]interface{}{
		"servers": dnsServers,
		"domains": dnsDomains,
		"id":      resourceID,
	})

	// Save final state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state with the latest data
func (r *DNSResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DNSResourceModel

	// Load current state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Reading DNS configuration from F5OS")

	// Fetch DNS config from F5OS API
	config, err := r.client.ReadDNSConfig()
	if err != nil {
		resp.Diagnostics.AddError(
			"DNS Read Error",
			fmt.Sprintf("Failed to fetch DNS configuration: %s", err),
		)
		return
	}

	// Normalize nil slices to empty slices
	if config.DNS.Servers.Server == nil {
		config.DNS.Servers.Server = []f5os.DNSServer{}
	}
	if config.DNS.Config.Search == nil {
		config.DNS.Config.Search = []string{}
	}

	// Extract values — use non-nil empty slices so types.ListValueFrom
	// always produces an empty list rather than a null Terraform value.
	servers := make([]string, 0, len(config.DNS.Servers.Server))
	for _, s := range config.DNS.Servers.Server {
		servers = append(servers, s.Address)
	}
	domains := make([]string, 0, len(config.DNS.Config.Search))
	domains = append(domains, config.DNS.Config.Search...)

	// Set Terraform types
	serversTF, diags := types.ListValueFrom(ctx, types.StringType, servers)
	resp.Diagnostics.Append(diags...)

	domainsTF, diags := types.ListValueFrom(ctx, types.StringType, domains)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Update state from device
	state.DNSServers = serversTF
	state.DNSDomains = domainsTF

	// Recompute resource ID based on current device configuration
	resourceID := computeResourceID(servers, domains)
	state.Id = types.StringValue(resourceID)

	tflog.Debug(ctx, "Setting DNS state", map[string]interface{}{
		"servers": servers,
		"domains": domains,
		"id":      state.Id.ValueString(),
	})

	// Save updated state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state
func (r *DNSResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior DNSResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updating DNS configuration")

	// Extract planned (new) values
	dnsServers, diags := extractStringList(ctx, plan.DNSServers)
	resp.Diagnostics.Append(diags...)

	dnsDomains, diags := extractStringList(ctx, plan.DNSDomains)
	resp.Diagnostics.Append(diags...)

	// Extract prior (old) values
	oldServers, diags := extractStringList(ctx, prior.DNSServers)
	resp.Diagnostics.Append(diags...)

	oldDomains, diags := extractStringList(ctx, prior.DNSDomains)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete servers and domains that were removed from the config
	removed := removedEntries(oldServers, dnsServers)
	removedDoms := removedEntries(oldDomains, dnsDomains)
	if err := r.client.DeleteDNSConfig(removed, removedDoms); err != nil {
		resp.Diagnostics.AddError("DNS Update Error",
			fmt.Sprintf("Failed to remove stale DNS entries: %s", err))
		return
	}

	// Patch remaining / newly added entries
	if err := r.client.PatchDNSConfig(dnsServers, dnsDomains); err != nil {
		resp.Diagnostics.AddError(
			"DNS Update Error",
			fmt.Sprintf("Failed to update DNS configuration: %s", err),
		)
		return
	}

	// Compute resource ID based on updated configuration
	resourceID := computeResourceID(dnsServers, dnsDomains)
	plan.Id = types.StringValue(resourceID)

	// Ensure dns_domains is never null/unknown in state after Update.
	if plan.DNSDomains.IsNull() || plan.DNSDomains.IsUnknown() {
		plan.DNSDomains, diags = types.ListValueFrom(ctx, types.StringType, dnsDomains)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	tflog.Debug(ctx, "DNS configuration updated successfully", map[string]interface{}{
		"servers": dnsServers,
		"domains": dnsDomains,
		"id":      resourceID,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// removedEntries returns elements present in old but absent from new.
func removedEntries(old, new []string) []string {
	set := make(map[string]struct{}, len(new))
	for _, v := range new {
		set[v] = struct{}{}
	}
	var removed []string
	for _, v := range old {
		if _, ok := set[v]; !ok {
			removed = append(removed, v)
		}
	}
	return removed
}

// Delete removes the DNS resource from Terraform state without modifying the
// device. DNS is a singleton system setting — removing the managed entries
// would break name resolution and could make the device unreachable.
func (r *DNSResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Removing DNS resource from Terraform state (device configuration is preserved)")

	resp.Diagnostics.AddWarning(
		"DNS Configuration Preserved",
		"The DNS resource has been removed from Terraform state, but the DNS configuration on the device has been left intact. "+
			"To modify DNS settings, re-import the resource or create a new f5os_dns resource.",
	)
}
