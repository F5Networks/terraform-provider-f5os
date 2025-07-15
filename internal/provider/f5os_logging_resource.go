package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// In your provider code:
type F5osAPI interface {
	GetRequest(string) ([]byte, error)
}

// Custom validator for facility attribute
type facilityValidator struct{}

// Ensure facilityValidator implements validator.String
var _ validator.String = facilityValidator{}

// Description provides a human-readable description of the validator
func (v facilityValidator) Description(ctx context.Context) string {
	return "Ensures the facility value is one of 'local0' or 'authpriv'."
}

// MarkdownDescription provides a markdown description of the validator
func (v facilityValidator) MarkdownDescription(ctx context.Context) string {
	return "Ensures the facility value is one of `local0` or `authpriv`."
}

// ValidateString performs the validation
func (v facilityValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	validFacilities := []string{"local0", "authpriv"}

	input := req.ConfigValue.ValueString()
	for _, valid := range validFacilities {
		if input == valid {
			return // Valid, no diagnostics needed
		}
	}

	resp.Diagnostics.AddError(
		"Invalid Facility Value",
		fmt.Sprintf("The value '%s' is not valid. Allowed values are: %v", input, validFacilities),
	)
}

// Add this to your struct definition:
type f5osLoggingResource struct {
	client *f5ossdk.F5os
}

// Ensure `f5osLoggingResource` implements necessary interfaces
var _ resource.Resource = &f5osLoggingResource{}
var _ resource.ResourceWithConfigure = &f5osLoggingResource{}

// Resource schema definition
func NewF5osLoggingResource() resource.Resource {
	return &f5osLoggingResource{}
}

type caBundleFields struct {
	Name    string `tfsdk:"name"`
	Content string `tfsdk:"content"`
}

type tlsFieldsStruct struct {
	Certificate string `tfsdk:"certificate"`
	Key         string `tfsdk:"key"`
}

type remoteForwardingLog struct {
	Facility string `tfsdk:"facility"`
	Severity string `tfsdk:"severity"`
}
type remoteForwardingFile struct {
	Name string `tfsdk:"name"`
}
type remoteForwardingFields struct {
	Enabled bool                   `tfsdk:"enabled"`
	Logs    []remoteForwardingLog  `tfsdk:"logs"`
	Files   []remoteForwardingFile `tfsdk:"files"`
}

type f5osLoggingModel struct {
	ID               types.String `tfsdk:"id"`
	Servers          types.List   `tfsdk:"servers"`
	RemoteForwarding types.Object `tfsdk:"remote_forwarding"`
	IncludeHostname  types.Bool   `tfsdk:"include_hostname"`
	TLS              types.Object `tfsdk:"tls"`
	CABundles        types.List   `tfsdk:"ca_bundles"`
	// State            types.String `tfsdk:"state"`
}

// func (r *f5osLoggingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
// 	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
// }

func (r *f5osLoggingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Log initialization attempt (optional)
	tflog.Debug(ctx, "Initializing client for f5osLoggingResource")

	// Retrieve the provider's client instance
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)

	// Log success or failure
	if r.client != nil {
		tflog.Debug(ctx, "Client successfully configured for f5osLoggingResource")
	} else {
		tflog.Warn(ctx, "Client not configured. Check Diagnostics for errors.")
	}
}

func (r *f5osLoggingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_logging"
}

func (r *f5osLoggingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The `f5os_logging` resource manages logging configuration on F5OS devices, including remote servers, TLS, CA bundles, remote forwarding, and hostname inclusion.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for the logging resource.",
			},
			"servers": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "A list of remote logging servers. Each server can specify address, port, protocol, authentication, and log selectors.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"address": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The IP address or hostname of the remote logging server.",
						},
						"port": schema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "The port number for the remote logging server (1-65535).",
							Validators: []validator.Int64{
								int64validator.Between(1, 65535), // Ensure port is between 1 and 65535
							},
						},
						"protocol": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The protocol used for logging (tcp or udp).",
							Validators: []validator.String{
								stringvalidator.OneOf("tcp", "udp"), // Enforce protocol to be either tcp or udp
							},
						},
						"authentication": schema.BoolAttribute{
							Optional:            true,
							MarkdownDescription: "Whether authentication is enabled for TCP protocol.",
						},
						"logs": schema.ListNestedAttribute{
							Optional:            true,
							MarkdownDescription: "Log selectors for this server, specifying facility and severity.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"facility": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "The syslog facility (e.g., local0, authpriv).",
										Validators: []validator.String{
											facilityValidator{},
										},
									},

									"severity": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "The syslog severity (e.g., debug, informational, notice, warning, error, critical, alert, emergency).",
										Validators: []validator.String{
											stringvalidator.OneOf(
												"debug", "informational", "notice", "warning",
												"error", "critical", "alert", "emergency", // Enforce valid severity levels
											),
										},
									},
								},
							},
						},
					},
				},
			},
			"remote_forwarding": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Remote forwarding configuration for host logs, including enablement, log selectors, and file outputs.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Required:            true,
						MarkdownDescription: "Whether remote forwarding is enabled.",
					},
					"logs": schema.ListNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Log selectors for remote forwarding, specifying facility and severity.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"facility": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "The syslog facility for remote forwarding (local0 or authpriv).",
									Validators: []validator.String{
										stringvalidator.OneOf("local0", "authpriv"), // Enforce valid facility values for remote_forwarding logs
									},
								},
								"severity": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "The syslog severity for remote forwarding.",
									Validators: []validator.String{
										stringvalidator.OneOf(
											"debug", "informational", "notice", "warning",
											"error", "critical", "alert", "emergency", // Enforce valid severity levels
										),
									},
								},
							},
						},
					},
					"files": schema.ListNestedAttribute{
						Optional:            true,
						MarkdownDescription: "List of files for remote forwarding output.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"name": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "The name of the file for log output.",
									Validators: []validator.String{
										stringvalidator.LengthBetween(1, 256), // Ensure name is a reasonable length
										stringvalidator.RegexMatches(regexp.MustCompile(`^[a-zA-Z0-9_\-.]+$`), "must be a valid filename"), // Validate filename format
									},
								},
							},
						},
					},
				},
			},
			"include_hostname": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether to include the hostname in log messages.",
			},
			"tls": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "TLS configuration for secure logging.",
				Attributes: map[string]schema.Attribute{
					"certificate": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "TLS certificate for secure logging.",
					},
					"key": schema.StringAttribute{
						Required:            true,
						Sensitive:           true, // Mark as sensitive so it doesn't show up in plan or state files
						MarkdownDescription: "TLS private key for secure logging (sensitive).",
					},
				},
			},
			"ca_bundles": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "List of CA bundles for TLS validation.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The name of the CA bundle.",
							Validators: []validator.String{
								stringvalidator.LengthBetween(1, 256), // Ensure name length is reasonable
								stringvalidator.RegexMatches(regexp.MustCompile(`^[a-zA-Z0-9_\-.]+$`), "must be a valid filename"), // Validate name format
							},
						},
						"content": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The PEM-encoded content of the CA bundle.",
						},
					},
				},
			},
		},
	}
}

func (r *f5osLoggingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan f5osLoggingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use the embedded client (set in Configure)
	client := r.client
	baseURI := "/openconfig-system:system/logging"

	// TLS and CA Bundles must be created together first (before servers)
	if err := putTLSWithCABundles(ctx, client, baseURI, plan.TLS, plan.CABundles, resp); err != nil {
		return
	}

	// Remote Forwarding
	if err := createRemoteForwarding(ctx, client, baseURI, plan.RemoteForwarding, resp); err != nil {
		return
	}

	// Servers (after TLS/CA bundles are in place)
	if err := createServers(ctx, client, baseURI, plan.Servers, resp); err != nil {
		return
	}

	// Include Hostname
	if err := createIncludeHostname(ctx, client, baseURI, plan.IncludeHostname, resp); err != nil {
		return
	}

	plan.ID = types.StringValue("f5os-logging")

	// Before setting state, print the plan for debugging
	planJson, _ := json.MarshalIndent(plan, "", "  ")
	log.Printf("[DEBUG] Plan before setting state: %s", string(planJson))

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *f5osLoggingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Initialize the state object
	var state f5osLoggingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	// Abort if there are issues with the request state
	if resp.Diagnostics.HasError() {
		return
	}

	client := r.client
	baseURI := "/openconfig-system:system/logging"

	// Check if servers are configured in the current state
	serversConfigured := !state.Servers.IsNull() && !state.Servers.IsUnknown()
	log.Printf("[DEBUG] Servers configured in current state: %v", serversConfigured)

	// Only fetch servers if they are explicitly configured
	if serversConfigured {
		if err := r.fetchServers(ctx, client, baseURI, &state); err != nil {
			resp.Diagnostics.AddError("Error Fetching Logging Servers", err.Error())
			return
		}
	} else {
		// Set servers to null when not configured
		state.Servers = types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"address":        types.StringType,
				"port":           types.Int64Type,
				"protocol":       types.StringType,
				"authentication": types.BoolType,
				"logs": types.ListType{
					ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"facility": types.StringType, "severity": types.StringType}},
				},
			},
		})
		log.Printf("[DEBUG] Servers not configured, setting to null")
	}

	// Check if any server requires authentication (TLS/CA bundles)
	authRequired := r.isAuthenticationRequired(&state)

	// Also check the original state/plan for TLS/CA bundles configuration
	tlsConfigured := !state.TLS.IsNull() && !state.TLS.IsUnknown()
	caBundlesConfigured := !state.CABundles.IsNull() && !state.CABundles.IsUnknown()

	log.Printf("[DEBUG] Authentication required: %v, TLS configured: %v, CA bundles configured: %v",
		authRequired, tlsConfigured, caBundlesConfigured)

	// Fetch TLS/CA bundles if either authentication is required OR they are explicitly configured
	if authRequired || tlsConfigured || caBundlesConfigured {
		if tlsConfigured {
			if err := r.fetchTLS(ctx, client, baseURI, &state); err != nil {
				resp.Diagnostics.AddError("Error Fetching TLS Configuration", err.Error())
				return
			}
		} else {
			// Set TLS to null when not configured
			state.TLS = types.ObjectNull(map[string]attr.Type{
				"certificate": types.StringType,
				"key":         types.StringType,
			})
		}

		if caBundlesConfigured {
			if err := r.fetchCABundles(ctx, client, baseURI, &state); err != nil {
				resp.Diagnostics.AddError("Error Fetching CA Bundles", err.Error())
				return
			}
		} else {
			// Set CA bundles to null when not configured
			state.CABundles = types.ListNull(types.ObjectType{
				AttrTypes: map[string]attr.Type{"name": types.StringType, "content": types.StringType},
			})
		}
	} else {
		// Set TLS and CA bundles to null when not needed and not configured
		state.TLS = types.ObjectNull(map[string]attr.Type{
			"certificate": types.StringType,
			"key":         types.StringType,
		})
		state.CABundles = types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{"name": types.StringType, "content": types.StringType},
		})
		log.Printf("[DEBUG] No authentication required and TLS/CA bundles not configured, setting to null")
	}

	// Check if remote forwarding is configured in the current state
	remoteForwardingConfigured := !state.RemoteForwarding.IsNull() && !state.RemoteForwarding.IsUnknown()
	log.Printf("[DEBUG] Remote forwarding configured in current state: %v", remoteForwardingConfigured)

	// Only fetch remote forwarding if it's explicitly configured
	if remoteForwardingConfigured {
		if err := r.fetchRemoteForwarding(ctx, client, baseURI, &state); err != nil {
			resp.Diagnostics.AddError("Error Fetching Remote Forwarding Configuration", err.Error())
			return
		}
	} else {
		// Set remote forwarding to null when not configured
		state.RemoteForwarding = types.ObjectNull(map[string]attr.Type{
			"enabled": types.BoolType,
			"logs": types.ListType{ElemType: types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"facility": types.StringType,
					"severity": types.StringType,
				},
			}},
			"files": types.ListType{ElemType: types.ObjectType{
				AttrTypes: map[string]attr.Type{"name": types.StringType},
			}},
		})
		log.Printf("[DEBUG] Remote forwarding not configured, setting to null")
	}

	// Check if include hostname is configured in the current state
	includeHostnameConfigured := !state.IncludeHostname.IsNull() && !state.IncludeHostname.IsUnknown()
	log.Printf("[DEBUG] Include hostname configured in current state: %v", includeHostnameConfigured)

	// Only fetch include hostname if it's explicitly configured
	if includeHostnameConfigured {
		if err := r.fetchIncludeHostname(ctx, client, baseURI, &state); err != nil {
			resp.Diagnostics.AddError("Error Fetching Include Hostname Configuration", err.Error())
			return
		}
	} else {
		// Set include hostname to null when not configured
		state.IncludeHostname = types.BoolNull()
		log.Printf("[DEBUG] Include hostname not configured, setting to null")
	}

	// Set defaults for mandatory fields (id and state)
	if state.ID.IsNull() || state.ID.IsUnknown() {
		state.ID = types.StringValue("f5os-logging")
	}

	log.Printf("[DEBUG] Final state before applying: %+v", state)

	// Apply the updated state back to Terraform
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// isAuthenticationRequired checks if any server in the state requires authentication
func (r *f5osLoggingResource) isAuthenticationRequired(state *f5osLoggingModel) bool {
	if state.Servers.IsNull() || state.Servers.IsUnknown() {
		return false
	}

	var servers []serverFields
	diags := state.Servers.ElementsAs(context.Background(), &servers, false)
	if diags.HasError() {
		log.Printf("[WARN] Failed to parse servers for authentication check: %v", diags)
		return false
	}

	for _, server := range servers {
		if server.Authentication && strings.ToLower(server.Protocol) == "tcp" {
			log.Printf("[DEBUG] Server %s requires authentication", server.Address)
			return true
		}
	}

	log.Printf("[DEBUG] No servers require authentication")
	return false
}

func (r *f5osLoggingResource) fetchServers(ctx context.Context, client *f5ossdk.F5os, baseURI string, state *f5osLoggingModel) error {
	tflog.Debug(ctx, fmt.Sprintf("[DEBUG] fetchServers: Fetching remote servers from API: %s/remote-servers", baseURI))
	serversResp, err := client.GetRequest(fmt.Sprintf("%s/remote-servers", baseURI))
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("[WARN] fetchServers: Failed to fetch servers (this is expected when no servers are configured): %v", err))
		// Set to null when no servers are configured and not in plan
		state.Servers = types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"address":        types.StringType,
				"port":           types.Int64Type,
				"protocol":       types.StringType,
				"authentication": types.BoolType,
				"logs": types.ListType{
					ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"facility": types.StringType, "severity": types.StringType}},
				},
			},
		})
		return nil
	}

	// Handle empty response (204 No Content) - means no servers are configured
	if len(serversResp) == 0 || strings.TrimSpace(string(serversResp)) == "" {
		tflog.Debug(ctx, "[DEBUG] fetchServers: Empty response from API, no servers configured")
		// Set to null when no servers are configured and not in plan
		state.Servers = types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"address":        types.StringType,
				"port":           types.Int64Type,
				"protocol":       types.StringType,
				"authentication": types.BoolType,
				"logs": types.ListType{
					ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"facility": types.StringType, "severity": types.StringType}},
				},
			},
		})
		return nil
	}

	tflog.Debug(ctx, fmt.Sprintf("[DEBUG] fetchServers: Raw API response: %s", string(serversResp)))

	var serversData map[string]interface{}
	var serverObjs []attr.Value

	// Parse the API response
	if err := json.Unmarshal(serversResp, &serversData); err != nil {
		tflog.Error(ctx, fmt.Sprintf("[ERROR] fetchServers: Failed to unmarshal API response: %v", err))
		return fmt.Errorf("could not parse server response: %w", err)
	}
	tflog.Debug(ctx, fmt.Sprintf("[DEBUG] fetchServers: Unmarshaled serversData: %+v", serversData))

	// Extract the nested map first!
	var remoteServers []interface{}
	if outer, ok := serversData["openconfig-system:remote-servers"].(map[string]interface{}); ok {
		if rs, ok := outer["remote-server"].([]interface{}); ok {
			remoteServers = rs
		}
	}

	if remoteServers != nil {
		tflog.Debug(ctx, fmt.Sprintf("[DEBUG] fetchServers: Found %d servers", len(remoteServers)))
		for idx, s := range remoteServers {
			tflog.Debug(ctx, fmt.Sprintf("[DEBUG] fetchServers: Processing server #%d: %+v", idx, s))
			server := s.(map[string]interface{})
			conf := server["config"].(map[string]interface{})
			tflog.Debug(ctx, fmt.Sprintf("[DEBUG] fetchServers: Server config: %+v", conf))

			address := strings.TrimSpace(fmt.Sprintf("%v", conf["host"]))
			port := int64(conf["remote-port"].(float64))
			protocol := strings.TrimSpace(fmt.Sprintf("%v", conf["f5-openconfig-system-logging:proto"]))

			authEnabled := false
			if authConfig, ok := conf["f5-openconfig-system-logging:authentication"].(map[string]interface{}); ok {
				if enabled, ok := authConfig["enabled"].(bool); ok {
					authEnabled = enabled
				}
			}

			// Logs as []serverLog
			var logObjs []serverLog
			if selectors, ok := server["selectors"].(map[string]interface{}); ok {
				if logsList, ok := selectors["selector"].([]interface{}); ok {
					for _, logItem := range logsList {
						logEntry := logItem.(map[string]interface{})
						facilityVal := fmt.Sprintf("%v", logEntry["facility"])
						if strings.Contains(facilityVal, ":") {
							facilityVal = strings.Split(facilityVal, ":")[1]
						}
						facilityVal = strings.ToLower(strings.TrimSpace(facilityVal))
						severityVal := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", logEntry["severity"])))
						logObjs = append(logObjs, serverLog{
							Facility: facilityVal,
							Severity: severityVal,
						})
					}
					// Sort logs by facility, then severity for idempotency
					sort.Slice(logObjs, func(i, j int) bool {
						if logObjs[i].Facility == logObjs[j].Facility {
							return logObjs[i].Severity < logObjs[j].Severity
						}
						return logObjs[i].Facility < logObjs[j].Facility
					})
				}
			}

			// Build serverFields struct
			serverStruct := serverFields{
				Address:        address,
				Port:           port,
				Protocol:       protocol,
				Authentication: authEnabled,
				Logs:           logObjs,
			}

			// Convert to ObjectValueFrom using struct
			serverValue, diags := types.ObjectValueFrom(ctx, map[string]attr.Type{
				"address":        types.StringType,
				"port":           types.Int64Type,
				"protocol":       types.StringType,
				"authentication": types.BoolType,
				"logs": types.ListType{
					ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"facility": types.StringType, "severity": types.StringType}},
				},
			}, serverStruct)
			if diags.HasError() {
				tflog.Error(ctx, fmt.Sprintf("[ERROR] fetchServers: Failed to build server object for %s: %v", address, diags))
				continue // skip this server
			}
			tflog.Debug(ctx, fmt.Sprintf("[DEBUG] fetchServers: Final server object for %s: %+v", address, serverValue))
			serverObjs = append(serverObjs, serverValue)
		}
	} else {
		tflog.Debug(ctx, "[DEBUG] fetchServers: No remote-server key found or not a list")
	}

	// Set the Terraform state
	serverType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"address":        types.StringType,
			"port":           types.Int64Type,
			"protocol":       types.StringType,
			"authentication": types.BoolType,
			"logs": types.ListType{
				ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"facility": types.StringType, "severity": types.StringType}},
			},
		},
	}

	// Sort servers by address for idempotency
	sort.Slice(serverObjs, func(i, j int) bool {
		a := serverObjs[i].(types.Object)
		b := serverObjs[j].(types.Object)
		addrA := a.Attributes()["address"].(types.String).ValueString()
		addrB := b.Attributes()["address"].(types.String).ValueString()
		return addrA < addrB
	})
	tflog.Debug(ctx, fmt.Sprintf("[DEBUG] fetchServers: Sorted serverObjs: %+v", serverObjs))

	// Only set servers if we actually found some, otherwise keep as null
	if len(serverObjs) > 0 {
		state.Servers, _ = types.ListValueFrom(ctx, serverType, serverObjs)
	} else {
		// Keep as null when no servers are configured
		state.Servers = types.ListNull(serverType)
	}

	tflog.Debug(ctx, fmt.Sprintf("[DEBUG] fetchServers: Final state.Servers: %+v", state.Servers))
	return nil
}

func (r *f5osLoggingResource) fetchIncludeHostname(ctx context.Context, client *f5ossdk.F5os, baseURI string, state *f5osLoggingModel) error {
	log.Printf("[DEBUG] Fetching Include Hostname Configuration")

	// Fetch IncludeHostname configuration from the API
	configResp, err := client.GetRequest(fmt.Sprintf("%s/f5-openconfig-system-logging:config", baseURI))
	if err != nil {
		log.Printf("[WARN] Failed to fetch IncludeHostname (this is expected when include hostname is not configured): %v", err)
		// Default to false when not configured
		state.IncludeHostname = types.BoolValue(false)
		return nil
	}

	log.Printf("[DEBUG] Include Hostname API Response: %s", string(configResp))

	var configData map[string]interface{}
	includeHostname := false // Default to false if not defined

	// Parse the API Response
	if err := json.Unmarshal(configResp, &configData); err != nil {
		log.Printf("[ERROR] Failed to parse IncludeHostname response: %v", err)
		return fmt.Errorf("could not parse include hostname response: %w", err)
	}

	if config, ok := configData["f5-openconfig-system-logging:config"].(map[string]interface{}); ok {
		log.Printf("[DEBUG] Include Hostname Config: %v", config)
		if v, ok := config["include-hostname"].(bool); ok {
			includeHostname = v
		}
	}

	// Set the Terraform state
	log.Printf("[DEBUG] Setting Include Hostname to: %v", includeHostname)
	state.IncludeHostname = types.BoolValue(includeHostname)

	if state.IncludeHostname.IsNull() || state.IncludeHostname.IsUnknown() {
		state.IncludeHostname = types.BoolValue(false)
		log.Printf("[WARN] Include Hostname set to default (false)")
	}

	return nil
}
func (r *f5osLoggingResource) fetchTLS(ctx context.Context, client *f5ossdk.F5os, baseURI string, state *f5osLoggingModel) error {
	log.Printf("[DEBUG] Fetching TLS configuration from API at path: %s", fmt.Sprintf("%s/f5-openconfig-system-logging:tls", baseURI))

	tlsResp, err := client.GetRequest(fmt.Sprintf("%s/f5-openconfig-system-logging:tls", baseURI))
	if err != nil {
		log.Printf("[WARN] Failed to fetch TLS configuration (this is expected when TLS is not configured): %v", err)
		// Set to null when TLS is not configured
		state.TLS = types.ObjectNull(map[string]attr.Type{
			"certificate": types.StringType,
			"key":         types.StringType,
		})
		return nil
	}

	var tlsData map[string]interface{}
	if err := json.Unmarshal(tlsResp, &tlsData); err != nil {
		log.Printf("[ERROR] Failed to parse TLS response: %v", err)
		state.TLS = types.ObjectNull(map[string]attr.Type{
			"certificate": types.StringType,
			"key":         types.StringType,
		})
		return nil
	}

	certificate := ""
	key := ""
	if tlsConfig, ok := tlsData["f5-openconfig-system-logging:tls"].(map[string]interface{}); ok {
		if certValue, certOK := tlsConfig["certificate"]; certOK {
			certificate = normalizeNewlines(fmt.Sprintf("%v", certValue))
			log.Printf("[DEBUG] TLS Certificate length after normalization: %d", len(certificate))
		}
		if keyValue, keyOK := tlsConfig["key"]; keyOK {
			rawKey := fmt.Sprintf("%v", keyValue)
			log.Printf("[DEBUG] TLS Key from F5OS starts with: %s", func() string {
				if len(rawKey) > 20 {
					return rawKey[:20]
				}
				return rawKey
			}())

			// If F5OS returns an encrypted key ($8$ format), we need to preserve the original
			// PEM format from the current state to avoid drift
			if strings.HasPrefix(rawKey, "$8$") && !state.TLS.IsNull() && !state.TLS.IsUnknown() {
				// Extract the current key from state to preserve the original PEM format
				var currentTLS tlsFieldsStruct
				if diags := state.TLS.As(ctx, &currentTLS, basetypes.ObjectAsOptions{}); !diags.HasError() {
					if strings.Contains(currentTLS.Key, "-----BEGIN") {
						log.Printf("[DEBUG] Preserving original PEM key format to avoid drift with F5OS encrypted format")
						key = currentTLS.Key
					} else {
						key = normalizeNewlines(rawKey)
					}
				} else {
					key = normalizeNewlines(rawKey)
				}
			} else {
				key = normalizeNewlines(rawKey)
			}
			log.Printf("[DEBUG] TLS Key length before normalization: %d, after: %d", len(rawKey), len(key))
		}
	}

	tlsObject, diags := types.ObjectValueFrom(
		ctx,
		map[string]attr.Type{
			"certificate": types.StringType,
			"key":         types.StringType,
		},
		tlsFieldsStruct{
			Certificate: certificate,
			Key:         key,
		},
	)
	if diags.HasError() {
		log.Printf("[ERROR] Failed to create TLS ObjectValue: %v", diags)
		return fmt.Errorf("error constructing TLS ObjectValue: %v", diags)
	}

	state.TLS = tlsObject
	log.Printf("[DEBUG] Successfully fetched and parsed TLS configuration.")
	return nil
}

func (r *f5osLoggingResource) fetchCABundles(ctx context.Context, client *f5ossdk.F5os, baseURI string, state *f5osLoggingModel) error {
	log.Printf("[DEBUG] Fetching CA Bundles configuration from API")
	caBundlesResp, err := client.GetRequest(fmt.Sprintf("%s/f5-openconfig-system-logging:tls/ca-bundles", baseURI))
	log.Printf("[DEBUG] CA Bundles API Response: %s", string(caBundlesResp))
	if err != nil {
		log.Printf("[WARN] Failed to fetch CA bundle configuration (this is expected when CA bundles are not configured): %v", err)
		// Set to null when CA bundles are not configured
		state.CABundles = types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{"name": types.StringType, "content": types.StringType},
		})
		return nil
	}

	var caBundlesData map[string]interface{}
	var caBundleObjs []attr.Value

	if len(caBundlesResp) == 0 || strings.TrimSpace(string(caBundlesResp)) == "" {
		// No CA bundles configured, treat as empty list
		caBundlesData = map[string]interface{}{"ca-bundle": []interface{}{}}
	} else {
		if err := json.Unmarshal(caBundlesResp, &caBundlesData); err != nil {
			return fmt.Errorf("could not parse CA bundle response: %w", err)
		}
	}

	// Extract CA bundles from the nested structure
	var caBundles []interface{}
	if caBundlesConfig, ok := caBundlesData["f5-openconfig-system-logging:ca-bundles"].(map[string]interface{}); ok {
		if bundleList, ok := caBundlesConfig["ca-bundle"].([]interface{}); ok {
			caBundles = bundleList
		}
	} else if bundleList, ok := caBundlesData["ca-bundle"].([]interface{}); ok {
		// Fallback for direct ca-bundle key
		caBundles = bundleList
	}

	if caBundles != nil {
		log.Printf("[DEBUG] Found %d CA bundles", len(caBundles))
		for _, b := range caBundles {
			bundle := b.(map[string]interface{})
			// Extract name and content from bundle structure
			var name, content string

			// Name is at the top level
			if bundleName, ok := bundle["name"]; ok {
				name = strings.TrimSpace(fmt.Sprintf("%v", bundleName))
			}

			// Content is inside the config object
			if config, hasConfig := bundle["config"].(map[string]interface{}); hasConfig {
				if bundleContent, ok := config["content"]; ok {
					content = normalizeNewlines(fmt.Sprintf("%v", bundleContent))
				}
			}

			log.Printf("[DEBUG] Processing CA bundle: name=%s, content_length=%d", name, len(content))

			obj, _ := types.ObjectValueFrom(
				ctx,
				map[string]attr.Type{
					"name":    types.StringType,
					"content": types.StringType,
				},
				caBundleFields{
					Name:    name,
					Content: content,
				},
			)
			caBundleObjs = append(caBundleObjs, obj)
		}
	} else {
		log.Printf("[DEBUG] No CA bundles found in response")
	}

	sort.Slice(caBundleObjs, func(i, j int) bool {
		a := caBundleObjs[i].(types.Object)
		b := caBundleObjs[j].(types.Object)
		return a.Attributes()["name"].(types.String).ValueString() < b.Attributes()["name"].(types.String).ValueString()
	})

	state.CABundles, _ = types.ListValueFrom(
		ctx,
		types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "content": types.StringType}},
		caBundleObjs,
	)

	if state.CABundles.IsNull() || state.CABundles.IsUnknown() {
		state.CABundles, _ = types.ListValueFrom(
			ctx,
			types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "content": types.StringType}},
			[]attr.Value{},
		)
	}

	return nil
}
func (r *f5osLoggingResource) fetchRemoteForwarding(ctx context.Context, client *f5ossdk.F5os, baseURI string, state *f5osLoggingModel) error {
	log.Printf("[DEBUG] Fetching Remote Forwarding configuration from API at path: %s", fmt.Sprintf("%s/f5-openconfig-system-logging:host-logs", baseURI))

	// Get the current state to preserve the original order from config
	var currentRemoteForwarding remoteForwardingFields
	if !state.RemoteForwarding.IsNull() && !state.RemoteForwarding.IsUnknown() {
		diags := state.RemoteForwarding.As(ctx, &currentRemoteForwarding, basetypes.ObjectAsOptions{})
		if diags.HasError() {
			log.Printf("[WARN] Failed to parse current remote forwarding state: %v", diags)
		}
	}

	// API Call
	rfResp, err := client.GetRequest(fmt.Sprintf("%s/f5-openconfig-system-logging:host-logs", baseURI))
	if err != nil {
		log.Printf("[WARN] Failed to fetch Remote Forwarding configuration (this is expected when remote forwarding is not configured): %v", err)
		// Set to disabled/empty when remote forwarding is not configured
		state.RemoteForwarding = types.ObjectValueMust(
			map[string]attr.Type{
				"enabled": types.BoolType,
				"logs": types.ListType{ElemType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"facility": types.StringType,
						"severity": types.StringType,
					},
				}},
				"files": types.ListType{ElemType: types.ObjectType{
					AttrTypes: map[string]attr.Type{"name": types.StringType},
				}},
			},
			map[string]attr.Value{
				"enabled": types.BoolValue(false),
				"logs":    types.ListValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{"facility": types.StringType, "severity": types.StringType}}, []attr.Value{}),
				"files":   types.ListValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType}}, []attr.Value{}),
			},
		)
		return nil
	}

	var rfData map[string]interface{}
	if err := json.Unmarshal(rfResp, &rfData); err != nil {
		log.Printf("[ERROR] Failed to parse Remote Forwarding response: %v", err)
		state.RemoteForwarding = types.ObjectValueMust(
			map[string]attr.Type{
				"enabled": types.BoolType,
				"logs": types.ListType{ElemType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"facility": types.StringType,
						"severity": types.StringType,
					},
				}},
				"files": types.ListType{ElemType: types.ObjectType{
					AttrTypes: map[string]attr.Type{"name": types.StringType},
				}},
			},
			map[string]attr.Value{
				"enabled": types.BoolValue(false),                                                                                                                             // Default for disabled state
				"logs":    types.ListValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{"facility": types.StringType, "severity": types.StringType}}, []attr.Value{}), // Empty list
				"files":   types.ListValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType}}, []attr.Value{}),                                   // Empty list
			},
		)
		return nil
	}

	// Default fallback values
	enabled := false
	logObjects := []map[string]interface{}{}
	fileObjects := []map[string]interface{}{}

	// Parse Remote Forwarding data
	if hostLogs, ok := rfData["f5-openconfig-system-logging:host-logs"].(map[string]interface{}); ok {
		if config, ok := hostLogs["config"].(map[string]interface{}); ok {
			// Extract `enabled`
			if remoteForwarding, rfOk := config["remote-forwarding"].(map[string]interface{}); rfOk {
				if enabledVal, eOk := remoteForwarding["enabled"].(bool); eOk {
					enabled = enabledVal
				}
			}

			// Extract `logs`
			if selectors, selOk := config["selectors"].(map[string]interface{}); selOk {
				if logEntries, logsOk := selectors["selector"].([]interface{}); logsOk {
					for _, logEntryRaw := range logEntries {
						logEntry, entryOk := logEntryRaw.(map[string]interface{})
						if entryOk {
							facilityVal := fmt.Sprintf("%v", logEntry["facility"])
							if strings.Contains(facilityVal, ":") {
								facilityVal = strings.Split(facilityVal, ":")[1]
							}
							logObjects = append(logObjects, map[string]interface{}{
								"facility": strings.ToLower(strings.TrimSpace(facilityVal)),
								"severity": strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", logEntry["severity"]))),
							})
						}
					}
				}
			}

			// Extract `files`
			if fileConfig, filesOk := config["files"].(map[string]interface{}); filesOk {
				if fileEntries, entriesOk := fileConfig["file"].([]interface{}); entriesOk {
					for _, fileEntryRaw := range fileEntries {
						fileEntry, entryOk := fileEntryRaw.(map[string]interface{})
						if entryOk {
							fileObjects = append(fileObjects, map[string]interface{}{
								"name": strings.TrimSpace(fmt.Sprintf("%v", fileEntry["name"])),
							})
						}
					}
				}
			}
		}
	}

	// Create maps for quick lookup from API response
	apiLogMap := make(map[string]bool)
	apiFileMap := make(map[string]bool)

	// Build lookup maps from API response
	for _, logEntry := range logObjects {
		key := fmt.Sprintf("%s|%s", logEntry["facility"].(string), logEntry["severity"].(string))
		apiLogMap[key] = true
	}
	for _, fileEntry := range fileObjects {
		apiFileMap[fileEntry["name"].(string)] = true
	}

	// Preserve original order from current state where possible, then add any new items
	var logsSlice []remoteForwardingLog
	var filesSlice []remoteForwardingFile

	// First, preserve the order from current state for items that still exist in API
	for _, currentLog := range currentRemoteForwarding.Logs {
		key := fmt.Sprintf("%s|%s", currentLog.Facility, currentLog.Severity)
		if apiLogMap[key] {
			logsSlice = append(logsSlice, currentLog)
			delete(apiLogMap, key) // Remove so we don't add it again
		}
	}

	// Add any new logs from API that weren't in current state
	for _, logEntry := range logObjects {
		key := fmt.Sprintf("%s|%s", logEntry["facility"].(string), logEntry["severity"].(string))
		if apiLogMap[key] {
			logsSlice = append(logsSlice, remoteForwardingLog{
				Facility: logEntry["facility"].(string),
				Severity: logEntry["severity"].(string),
			})
		}
	}

	// Same approach for files - preserve current state order
	for _, currentFile := range currentRemoteForwarding.Files {
		if apiFileMap[currentFile.Name] {
			filesSlice = append(filesSlice, currentFile)
			delete(apiFileMap, currentFile.Name) // Remove so we don't add it again
		}
	}

	// Add any new files from API that weren't in current state
	for _, fileEntry := range fileObjects {
		if apiFileMap[fileEntry["name"].(string)] {
			filesSlice = append(filesSlice, remoteForwardingFile{
				Name: fileEntry["name"].(string),
			})
		}
	}
	// Construct Remote Forwarding ObjectValue
	remoteForwardingObject, diags := types.ObjectValueFrom(
		ctx,
		map[string]attr.Type{
			"enabled": types.BoolType,
			"logs": types.ListType{ElemType: types.ObjectType{
				AttrTypes: map[string]attr.Type{"facility": types.StringType, "severity": types.StringType},
			}},
			"files": types.ListType{ElemType: types.ObjectType{
				AttrTypes: map[string]attr.Type{"name": types.StringType},
			}},
		},
		remoteForwardingFields{
			Enabled: enabled,
			Logs:    logsSlice,  // []remoteForwardingLog
			Files:   filesSlice, // []remoteForwardingFile
		},
	)

	if diags.HasError() {
		log.Printf("[ERROR] Failed to create RemoteForwarding ObjectValue: %v", diags)
		return fmt.Errorf("error constructing RemoteForwarding ObjectValue: %v", diags)
	}

	// Assign constructed value to state
	state.RemoteForwarding = remoteForwardingObject
	log.Printf("[DEBUG] Successfully fetched and parsed Remote Forwarding configuration.")
	return nil
}

func (r *f5osLoggingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan f5osLoggingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		log.Printf("[ERROR] Diagnostics error after getting plan: %v", resp.Diagnostics)
		return
	}

	client := r.client
	baseURI := "/openconfig-system:system/logging"

	if err := putTLSWithCABundles(ctx, client, baseURI, plan.TLS, plan.CABundles, (*resource.CreateResponse)(resp)); err != nil {
		log.Printf("[ERROR] TLS+CA Bundles update failed: %v", err)
		return
	}

	// Update Remote Forwarding
	log.Printf("[DEBUG] Updating Remote Forwarding...")
	if err := createRemoteForwarding(ctx, client, baseURI, plan.RemoteForwarding, (*resource.CreateResponse)(resp)); err != nil {
		log.Printf("[ERROR] Remote Forwarding update failed: %v", err)
		return
	}
	log.Printf("[DEBUG] Remote Forwarding update complete.")

	// Update Servers
	log.Printf("[DEBUG] Updating Servers...")
	if err := createServers(ctx, client, baseURI, plan.Servers, (*resource.CreateResponse)(resp)); err != nil {
		log.Printf("[ERROR] Servers update failed: %v", err)
		return
	}
	log.Printf("[DEBUG] Servers update complete.")

	// Update Include Hostname
	log.Printf("[DEBUG] Updating Include Hostname...")
	if err := createIncludeHostname(ctx, client, baseURI, plan.IncludeHostname, (*resource.CreateResponse)(resp)); err != nil {
		log.Printf("[ERROR] Include Hostname update failed: %v", err)
		return
	}
	log.Printf("[DEBUG] Include Hostname update complete.")

	plan.ID = types.StringValue("f5os-logging")
	// plan.State = types.StringValue("applied")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	log.Printf("[DEBUG] Update complete. State set: %+v", plan)
}

func (r *f5osLoggingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state f5osLoggingModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := r.client
	baseURI := "/openconfig-system:system/logging"

	log.Printf("[DEBUG] Starting delete process for logging configuration")

	// Delete in the correct order to avoid dependency issues:
	// 1. First delete servers (which may have authentication dependencies on TLS)
	// 2. Then delete remote forwarding
	// 3. Then delete include hostname
	// 4. Finally delete TLS and CA bundles (last because servers depend on them)

	// Delete Servers first (they depend on TLS/CA bundles)
	log.Printf("[DEBUG] Deleting servers configuration")
	if !state.Servers.IsNull() && !state.Servers.IsUnknown() {
		err := client.DeleteRequest(baseURI + "/remote-servers")
		if err != nil {
			log.Printf("[WARN] Failed to delete servers configuration (may not exist): %v", err)
			// Don't treat this as a fatal error - servers might not exist
		} else {
			log.Printf("[DEBUG] Successfully deleted servers configuration")
		}
	}

	// Delete Remote Forwarding
	log.Printf("[DEBUG] Deleting remote forwarding configuration")
	if !state.RemoteForwarding.IsNull() && !state.RemoteForwarding.IsUnknown() {
		err := client.DeleteRequest(baseURI + "/f5-openconfig-system-logging:host-logs")
		if err != nil {
			log.Printf("[WARN] Failed to delete remote forwarding configuration (may not exist): %v", err)
			// Don't treat this as a fatal error
		} else {
			log.Printf("[DEBUG] Successfully deleted remote forwarding configuration")
		}
	}

	// Delete Include Hostname
	log.Printf("[DEBUG] Deleting include hostname configuration")
	if !state.IncludeHostname.IsNull() && !state.IncludeHostname.IsUnknown() {
		err := client.DeleteRequest(baseURI + "/f5-openconfig-system-logging:config")
		if err != nil {
			log.Printf("[WARN] Failed to delete include hostname configuration (may not exist): %v", err)
			// Don't treat this as a fatal error
		} else {
			log.Printf("[DEBUG] Successfully deleted include hostname configuration")
		}
	}

	// Delete TLS configuration last (after all dependent resources are removed)
	log.Printf("[DEBUG] Deleting TLS configuration")
	if !state.TLS.IsNull() && !state.TLS.IsUnknown() || !state.CABundles.IsNull() && !state.CABundles.IsUnknown() {
		err := client.DeleteRequest(baseURI + "/f5-openconfig-system-logging:tls")
		if err != nil {
			log.Printf("[WARN] Failed to delete TLS configuration (may not exist): %v", err)
			// Check if the error is due to servers still having authentication enabled
			errStr := strings.ToLower(err.Error())
			if strings.Contains(errStr, "cannot allow remote server authentication") ||
				strings.Contains(errStr, "ca-bundle") ||
				strings.Contains(errStr, "cert & key") {
				resp.Diagnostics.AddWarning("TLS Delete Warning",
					"TLS configuration could not be deleted because there may still be servers with authentication enabled. "+
						"This is usually not a problem as the configuration will be cleaned up when the servers are fully removed. "+
						"Original error: "+err.Error())
			} else {
				log.Printf("[ERROR] Unexpected error deleting TLS configuration: %v", err)
				resp.Diagnostics.AddError("TLS Delete Error", err.Error())
				return
			}
		} else {
			log.Printf("[DEBUG] Successfully deleted TLS configuration")
		}
	}

	log.Printf("[DEBUG] Delete process completed for logging configuration")
}

// --- Helper Functions ---
func putTLSWithCABundles(
	ctx context.Context,
	client *f5ossdk.F5os,
	baseURI string,
	tlsObj types.Object,
	caBundlesObj types.List,
	resp *resource.CreateResponse,
) error {
	// Skip if both TLS and CA bundles are null/unknown
	if (tlsObj.IsNull() || tlsObj.IsUnknown()) && (caBundlesObj.IsNull() || caBundlesObj.IsUnknown()) {
		log.Printf("[DEBUG] Both TLS and CA bundles are null/unknown, skipping TLS+CA bundles creation.")
		return nil
	}

	var tlsFields tlsFieldsStruct
	var caBundles []caBundleFields

	// Handle TLS fields
	if !tlsObj.IsNull() && !tlsObj.IsUnknown() {
		diags := tlsObj.As(ctx, &tlsFields, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return fmt.Errorf("diagnostic error during TLS object processing")
		}
	}

	// Handle CA bundles
	if !caBundlesObj.IsNull() && !caBundlesObj.IsUnknown() {
		diags := caBundlesObj.ElementsAs(ctx, &caBundles, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return fmt.Errorf("diagnostic error during CA bundles processing")
		}
	}

	// Build ca-bundle list
	caBundleList := make([]map[string]interface{}, 0, len(caBundles))
	for _, bundle := range caBundles {
		caBundleList = append(caBundleList, map[string]interface{}{
			"name": bundle.Name,
			"config": map[string]interface{}{
				"name":    bundle.Name,
				"content": bundle.Content,
			},
		})
	}

	// Build the full payload
	payload := map[string]interface{}{
		"f5-openconfig-system-logging:tls": map[string]interface{}{},
	}

	// Add certificate and key if TLS is configured
	if !tlsObj.IsNull() && !tlsObj.IsUnknown() {
		payload["f5-openconfig-system-logging:tls"].(map[string]interface{})["certificate"] = tlsFields.Certificate
		payload["f5-openconfig-system-logging:tls"].(map[string]interface{})["key"] = tlsFields.Key
		log.Printf("[DEBUG] Added TLS certificate and key to payload")
	}

	// Add CA bundles if configured
	if len(caBundleList) > 0 {
		payload["f5-openconfig-system-logging:tls"].(map[string]interface{})["ca-bundles"] = map[string]interface{}{
			"ca-bundle": caBundleList,
		}
		log.Printf("[DEBUG] Added %d CA bundles to payload", len(caBundleList))
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("TLS+CA Bundles Marshal Error", err.Error())
		return err
	}

	log.Printf("[DEBUG] TLS+CA Bundles payload: %s", string(payloadBytes))

	_, err = client.PutRequest(baseURI+"/f5-openconfig-system-logging:tls", payloadBytes)
	if err != nil {
		resp.Diagnostics.AddError("TLS+CA Bundles API Error", err.Error())
		return err
	}

	log.Printf("[DEBUG] Successfully created TLS configuration with CA bundles")
	return nil
}

func createRemoteForwarding(ctx context.Context, client *f5ossdk.F5os, baseURI string, rfObj types.Object, resp *resource.CreateResponse) error {
	if rfObj.IsNull() || rfObj.IsUnknown() {
		return nil
	}
	var rf remoteForwardingFields
	diags := rfObj.As(ctx, &rf, basetypes.ObjectAsOptions{})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return fmt.Errorf("diagnostic error")
	}

	// --- Sort logs by facility, then severity ---
	sort.Slice(rf.Logs, func(i, j int) bool {
		if rf.Logs[i].Facility == rf.Logs[j].Facility {
			return rf.Logs[i].Severity < rf.Logs[j].Severity
		}
		return rf.Logs[i].Facility < rf.Logs[j].Facility
	})
	// --- Sort files by name ---
	sort.Slice(rf.Files, func(i, j int) bool {
		return rf.Files[i].Name < rf.Files[j].Name
	})

	payload := map[string]interface{}{
		"f5-openconfig-system-logging:config": map[string]interface{}{
			"remote-forwarding": map[string]interface{}{
				"enabled": rf.Enabled,
			},
		},
	}
	// logs
	if len(rf.Logs) > 0 {
		selectors := make([]map[string]interface{}, 0)
		for _, l := range rf.Logs {
			selectors = append(selectors, map[string]interface{}{
				"facility": fmt.Sprintf("openconfig-system-logging:%s", strings.ToUpper(l.Facility)),
				"severity": strings.ToUpper(l.Severity),
			})
		}
		payload["f5-openconfig-system-logging:config"].(map[string]interface{})["selectors"] = map[string]interface{}{
			"selector": selectors,
		}
	}
	// files
	if len(rf.Files) > 0 {
		fileList := make([]map[string]interface{}, 0)
		for _, f := range rf.Files {
			fileList = append(fileList, map[string]interface{}{
				"name": f.Name,
			})
		}
		payload["f5-openconfig-system-logging:config"].(map[string]interface{})["files"] = map[string]interface{}{
			"file": fileList,
		}
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Remote Forwarding Marshal Error", err.Error())
		return err
	}
	_, err = client.PutRequest(baseURI+"/f5-openconfig-system-logging:host-logs/config", payloadBytes)
	if err != nil {
		resp.Diagnostics.AddError("Remote Forwarding Create Error", err.Error())
		return err
	}
	return nil
}

type serverLog struct {
	Facility string `tfsdk:"facility"`
	Severity string `tfsdk:"severity"`
}
type serverFields struct {
	Address        string      `tfsdk:"address"`
	Port           int64       `tfsdk:"port"`
	Protocol       string      `tfsdk:"protocol"`
	Authentication bool        `tfsdk:"authentication"`
	Logs           []serverLog `tfsdk:"logs"`
}

func createServers(ctx context.Context, client *f5ossdk.F5os, baseURI string, serversObj types.List, resp *resource.CreateResponse) error {
	if serversObj.IsNull() || serversObj.IsUnknown() {
		log.Printf("[DEBUG] Servers object is null or unknown, skipping server creation.")
		return nil
	}
	var servers []serverFields
	diags := serversObj.ElementsAs(ctx, &servers, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		log.Printf("[ERROR] Diagnostics error in createServers: %v", resp.Diagnostics)
		return fmt.Errorf("diagnostic error")
	}

	// Pre-validate: check if any server requires authentication
	hasAuthenticatedServer := false
	for _, server := range servers {
		if server.Authentication && strings.ToLower(server.Protocol) == "tcp" {
			hasAuthenticatedServer = true
			log.Printf("[DEBUG] Server %s requires authentication", server.Address)
			break
		}
	}

	// If we have authenticated servers, verify TLS configuration exists on the F5OS device
	if hasAuthenticatedServer {
		log.Printf("[DEBUG] Found servers with authentication enabled, verifying TLS configuration is available")

		// Try to read the current TLS configuration to ensure it exists
		tlsResponse, err := client.GetRequest(baseURI + "/f5-openconfig-system-logging:tls")
		if err != nil || tlsResponse == nil {
			resp.Diagnostics.AddError("TLS Configuration Required",
				"One or more servers have authentication enabled, but TLS configuration is not available on the F5OS device. "+
					"Please ensure TLS certificates and CA bundles are properly configured before enabling server authentication.")
			return fmt.Errorf("TLS configuration required for authenticated servers")
		}
		log.Printf("[DEBUG] TLS configuration verified for authenticated servers")
	}

	// Sort servers by address for consistent ordering
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Address < servers[j].Address
	})

	for _, server := range servers {
		// Sort logs for this server
		sort.Slice(server.Logs, func(i, j int) bool {
			if server.Logs[i].Facility == server.Logs[j].Facility {
				return server.Logs[i].Severity < server.Logs[j].Severity
			}
			return server.Logs[i].Facility < server.Logs[j].Facility
		})

		log.Printf("[DEBUG] Preparing server payload for address: %s, protocol: %s, authentication: %v", server.Address, server.Protocol, server.Authentication)

		// Validate authentication requirements
		if server.Authentication && strings.ToLower(server.Protocol) == "tcp" {
			log.Printf("[INFO] Server %s has authentication enabled - ensuring TLS/CA bundle configuration exists", server.Address)
		}

		payload := map[string]interface{}{
			"remote-server": []map[string]interface{}{},
		}
		serverConf := map[string]interface{}{
			"host": server.Address,
			"config": map[string]interface{}{
				"host":                               server.Address,
				"remote-port":                        server.Port,
				"f5-openconfig-system-logging:proto": server.Protocol,
			},
		}
		// Only include authentication if protocol is tcp
		if strings.ToLower(server.Protocol) == "tcp" {
			serverConf["config"].(map[string]interface{})["f5-openconfig-system-logging:authentication"] = map[string]interface{}{
				"enabled": server.Authentication,
			}
		}
		// logs (unchanged)
		if len(server.Logs) > 0 {
			selectors := make([]map[string]interface{}, 0)
			for _, l := range server.Logs {
				selectors = append(selectors, map[string]interface{}{
					"facility": fmt.Sprintf("f5-system-logging-types:%s", strings.ToUpper(l.Facility)),
					"severity": strings.ToUpper(l.Severity),
					"config": map[string]interface{}{
						"facility": fmt.Sprintf("f5-system-logging-types:%s", strings.ToUpper(l.Facility)),
						"severity": strings.ToUpper(l.Severity),
					},
				})
			}
			serverConf["selectors"] = map[string]interface{}{
				"selector": selectors,
			}
		}
		payload["remote-server"] = append(payload["remote-server"].([]map[string]interface{}), serverConf)
		payloadBytes, err := json.Marshal(payload)
		log.Printf("[DEBUG] Server payload for %s: %s", server.Address, string(payloadBytes))
		if err != nil {
			resp.Diagnostics.AddError("Server Marshal Error", err.Error())
			return err
		}
		_, err = client.PostRequest(baseURI+"/remote-servers/", payloadBytes)
		if err != nil {
			log.Printf("[WARN] POST failed for %s, trying PUT. Error: %v", server.Address, err)
			_, putErr := client.PutRequest(baseURI+fmt.Sprintf("/remote-servers/remote-server=%s", server.Address), payloadBytes)
			if putErr != nil {
				log.Printf("[ERROR] PUT failed for %s: %v", server.Address, putErr) // Provide more helpful error message for authentication issues
				errStr := strings.ToLower(putErr.Error())
				if strings.Contains(errStr, "cannot allow remote server authentication") ||
					strings.Contains(errStr, "ca-bundle") ||
					strings.Contains(errStr, "cert & key") ||
					strings.Contains(errStr, "configuration does not comply") {
					resp.Diagnostics.AddError("Server Authentication Configuration Error",
						fmt.Sprintf("Server %s has authentication enabled but required TLS certificates and CA bundles are not properly configured. "+
							"F5OS requires that when authentication is enabled, both TLS certificate/key and CA bundles must be configured. "+
							"Error from F5OS: %s", server.Address, putErr.Error()))
				} else {
					resp.Diagnostics.AddError("Server Create Error", fmt.Sprintf("Failed to create server %s: %s", server.Address, putErr.Error()))
				}
				return putErr
			}
		}
	}

	return nil
}

func createIncludeHostname(ctx context.Context, client *f5ossdk.F5os, baseURI string, includeHostname types.Bool, resp *resource.CreateResponse) error {
	if includeHostname.IsNull() || includeHostname.IsUnknown() {
		return nil
	}
	payload := map[string]interface{}{
		"f5-openconfig-system-logging:config": map[string]interface{}{
			"include-hostname": includeHostname.ValueBool(),
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Include Hostname Marshal Error", err.Error())
		return err
	}
	_, err = client.PutRequest(baseURI+"/f5-openconfig-system-logging:config", payloadBytes)
	if err != nil {
		resp.Diagnostics.AddError("Include Hostname Create Error", err.Error())
		return err
	}
	return nil
}
func normalizeNewlines(s string) string {
	// Trim whitespace and normalize line endings
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// For PEM formatted content (certificates and keys), ensure it ends with a newline
	if strings.Contains(s, "-----BEGIN") && strings.Contains(s, "-----END") {
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
	} else if strings.HasPrefix(s, "$8$") {
		// F5-specific encrypted key format - ensure it ends with a newline
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
	}

	return s
}
