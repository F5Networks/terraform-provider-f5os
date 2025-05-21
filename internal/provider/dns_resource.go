package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5os "gitswarm.f5net.com/terraform-providers/f5osclient"
)

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
			"~> **NOTE:** The `f5os_dns` resource updates DNS servers and search domains on the F5OS platforms using Open API",
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
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for the resource, computed from the DNS server configuration.",
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

// extractStringList safely extracts a string list from a types.List
func extractStringList(ctx context.Context, list types.List) ([]string, diag.Diagnostics) {
	var result []string
	var diags diag.Diagnostics

	if !list.IsNull() && !list.IsUnknown() {
		diags = list.ElementsAs(ctx, &result, false)
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
	rawResp, err := r.client.GetRequest("/openconfig-system:system/dns")
	if err != nil {
		resp.Diagnostics.AddError(
			"DNS Read Error",
			fmt.Sprintf("Failed to fetch DNS configuration: %s", err),
		)
		return
	}

	tflog.Debug(ctx, "Received DNS configuration", map[string]interface{}{
		"response": string(rawResp),
	})

	// Parse the response
	var config f5os.DNSConfigPayload
	if err := json.Unmarshal(rawResp, &config); err != nil {
		resp.Diagnostics.AddError(
			"DNS Parse Error",
			fmt.Sprintf("Failed to parse DNS configuration: %s\nRaw response: %s", err, string(rawResp)),
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

	// Extract values
	var servers, domains []string
	for _, s := range config.DNS.Servers.Server {
		servers = append(servers, s.Address)
	}
	domains = append(domains, config.DNS.Config.Search...)

	// Set Terraform types
	_, diags := types.ListValueFrom(ctx, types.StringType, servers)
	resp.Diagnostics.Append(diags...)

	_, diags = types.ListValueFrom(ctx, types.StringType, domains)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Update state
	// state.DNSServers = serversTF
	// state.DNSDomains = domainsTF

	// Compute resource ID based on current configuration
	// resourceID := computeResourceID(servers, domains)
	// state.Id = types.StringValue(resourceID)

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
	var plan DNSResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updating DNS configuration")

	// Extract DNS Servers and Domains
	dnsServers, diags := extractStringList(ctx, plan.DNSServers)
	resp.Diagnostics.Append(diags...)

	dnsDomains, diags := extractStringList(ctx, plan.DNSDomains)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Call PATCH operation via client SDK
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

	tflog.Debug(ctx, "DNS configuration updated successfully", map[string]interface{}{
		"servers": dnsServers,
		"domains": dnsDomains,
		"id":      resourceID,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the resource and removes the Terraform state
func (r *DNSResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DNSResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Deleting DNS configuration")

	// Extract DNS servers and domains
	dnsServers, diags := extractStringList(ctx, state.DNSServers)
	resp.Diagnostics.Append(diags...)

	dnsDomains, diags := extractStringList(ctx, state.DNSDomains)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete DNS servers individually
	for _, server := range dnsServers {
		if err := r.client.DeleteDNSServer(server); err != nil {
			resp.Diagnostics.AddWarning(
				"DNS Server Deletion Warning",
				fmt.Sprintf("Failed to delete DNS server [%s]: %s", server, err),
			)
		}
	}

	// Delete DNS search domains individually
	for _, domain := range dnsDomains {
		if err := r.client.DeleteSearchDomain(domain); err != nil {
			resp.Diagnostics.AddWarning(
				"DNS Domain Deletion Warning",
				fmt.Sprintf("Failed to delete DNS domain [%s]: %s", domain, err),
			)
		}
	}

	tflog.Debug(ctx, "DNS configuration deleted", map[string]interface{}{
		"servers": dnsServers,
		"domains": dnsDomains,
		"id":      state.Id.ValueString(),
	})
}
