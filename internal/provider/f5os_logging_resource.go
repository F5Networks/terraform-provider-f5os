package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
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
	State            types.String `tfsdk:"state"`
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
								int64validator.Between(1, 65535),
							},
						},
						"protocol": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The protocol used for logging (tcp or udp).",
							Validators: []validator.String{
								stringvalidator.OneOf("tcp", "udp"),
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
												"error", "critical", "alert", "emergency",
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
										stringvalidator.OneOf("local0", "authpriv"),
									},
								},
								"severity": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "The syslog severity for remote forwarding.",
									Validators: []validator.String{
										stringvalidator.OneOf(
											"debug", "informational", "notice", "warning",
											"error", "critical", "alert", "emergency",
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
										stringvalidator.LengthBetween(1, 256),
										stringvalidator.RegexMatches(regexp.MustCompile(`^[a-zA-Z0-9_\-.]+$`), "must be a valid filename"),
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
						Sensitive:           true,
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
								stringvalidator.LengthBetween(1, 256),
								stringvalidator.RegexMatches(regexp.MustCompile(`^[a-zA-Z0-9_\-.]+$`), "must be a valid filename"),
							},
						},
						"content": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The PEM-encoded content of the CA bundle.",
						},
					},
				},
			},
			"state": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Current state of the logging resource.",
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

	// TLS
	if err := createTLS(ctx, client, baseURI, plan.TLS, resp); err != nil {
		return
	}

	// CA Bundles
	if err := createCABundles(ctx, client, baseURI, plan.CABundles, resp); err != nil {
		return
	}

	// Remote Forwarding
	if err := createRemoteForwarding(ctx, client, baseURI, plan.RemoteForwarding, resp); err != nil {
		return
	}

	// Servers
	if err := createServers(ctx, client, baseURI, plan.Servers, resp); err != nil {
		return
	}

	// Include Hostname
	if err := createIncludeHostname(ctx, client, baseURI, plan.IncludeHostname, resp); err != nil {
		return
	}

	plan.ID = types.StringValue("f5os-logging")
	plan.State = types.StringValue("applied")
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

	// Load the state for individual attributes using helper functions
	if err := r.fetchTLS(ctx, client, baseURI, &state); err != nil {
		resp.Diagnostics.AddError("Error Fetching TLS Configuration", err.Error())
	}

	if err := r.fetchCABundles(ctx, client, baseURI, &state); err != nil {
		resp.Diagnostics.AddError("Error Fetching CA Bundles", err.Error())
	}

	if err := r.fetchRemoteForwarding(ctx, client, baseURI, &state); err != nil {
		resp.Diagnostics.AddError("Error Fetching Remote Forwarding Configuration", err.Error())
	}

	if err := r.fetchServers(ctx, client, baseURI, &state); err != nil {
		resp.Diagnostics.AddError("Error Fetching Logging Servers", err.Error())
	}

	// Fetch the IncludeHostname
	if err := r.fetchIncludeHostname(ctx, client, baseURI, &state); err != nil {
		resp.Diagnostics.AddError("Error Fetching Include Hostname Configuration", err.Error())
	}

	// Set defaults for mandatory fields (id and state)
	if state.ID.IsNull() || state.ID.IsUnknown() {
		state.ID = types.StringValue("f5os-logging")
	}
	if state.State.IsNull() || state.State.IsUnknown() {
		state.State = types.StringValue("active")
	}

	log.Printf("[DEBUG] Final state before applying: %+v", state)

	// Apply the updated state back to Terraform
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *f5osLoggingResource) fetchServers(ctx context.Context, client *f5ossdk.F5os, baseURI string, state *f5osLoggingModel) error {
	serversResp, err := client.GetRequest(fmt.Sprintf("%s/remote-servers", baseURI))
	if err != nil {
		return fmt.Errorf("could not fetch logging servers: %w", err)
	}

	var serversData map[string]interface{}
	var serverObjs []attr.Value

	// Parse the API response
	if err := json.Unmarshal(serversResp, &serversData); err != nil {
		return fmt.Errorf("could not parse server response: %w", err)
	}

	if servers, ok := serversData["remote-server"].([]interface{}); ok {
		for _, s := range servers {
			server := s.(map[string]interface{})
			conf := server["config"].(map[string]interface{})

			// Map server configuration
			serverObj := map[string]attr.Value{
				"address":  types.StringValue(fmt.Sprintf("%v", conf["host"])),
				"port":     types.Int64Value(int64(conf["remote-port"].(float64))),
				"protocol": types.StringValue(fmt.Sprintf("%v", conf["f5-openconfig-system-logging:proto"])),
			}

			// Authentication
			if authConfig, ok := conf["f5-openconfig-system-logging:authentication"].(map[string]interface{}); ok {
				serverObj["authentication"] = types.BoolValue(authConfig["enabled"].(bool))
			} else {
				serverObj["authentication"] = types.BoolValue(false)
			}

			// Logs
			if selectors, ok := server["selectors"].(map[string]interface{}); ok {
				if logs, ok := selectors["selector"].([]interface{}); ok {
					var logObjs []attr.Value
					for _, log := range logs {
						logEntry := log.(map[string]interface{})
						obj, _ := types.ObjectValueFrom(
							ctx,
							map[string]attr.Type{"facility": types.StringType, "severity": types.StringType},
							map[string]attr.Value{
								"facility": types.StringValue(strings.ToLower(strings.Split(fmt.Sprintf("%v", logEntry["facility"]), ":")[1])),
								"severity": types.StringValue(strings.ToLower(fmt.Sprintf("%v", logEntry["severity"]))),
							},
						)
						logObjs = append(logObjs, obj)
					}
					serverObj["logs"], _ = types.ListValueFrom(
						ctx,
						types.ObjectType{AttrTypes: map[string]attr.Type{"facility": types.StringType, "severity": types.StringType}},
						logObjs,
					)
				} else {
					serverObj["logs"] = types.ListValueMust(
						types.ObjectType{AttrTypes: map[string]attr.Type{"facility": types.StringType, "severity": types.StringType}},
						[]attr.Value{},
					)
				}
			} else {
				serverObj["logs"] = types.ListValueMust(
					types.ObjectType{AttrTypes: map[string]attr.Type{"facility": types.StringType, "severity": types.StringType}},
					[]attr.Value{},
				)
			}

			// Append server to the list
			serverValue, _ := types.ObjectValueFrom(ctx, map[string]attr.Type{
				"address":        types.StringType,
				"port":           types.Int64Type,
				"protocol":       types.StringType,
				"authentication": types.BoolType,
				"logs": types.ListType{
					ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"facility": types.StringType, "severity": types.StringType}},
				},
			}, serverObj)
			serverObjs = append(serverObjs, serverValue)
		}
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

	state.Servers, _ = types.ListValueFrom(ctx, serverType, serverObjs)

	// Set to empty list if no servers are configured
	if state.Servers.IsNull() || state.Servers.IsUnknown() {
		state.Servers, _ = types.ListValueFrom(ctx, serverType, []attr.Value{})
	}

	return nil
}

func (r *f5osLoggingResource) fetchIncludeHostname(ctx context.Context, client *f5ossdk.F5os, baseURI string, state *f5osLoggingModel) error {
	log.Printf("[DEBUG] Fetching Include Hostname Configuration")

	// Fetch IncludeHostname configuration from the API
	configResp, err := client.GetRequest(fmt.Sprintf("%s/f5-openconfig-system-logging:config", baseURI))
	if err != nil {
		log.Printf("[ERROR] Failed to fetch IncludeHostname: %v", err)
		return fmt.Errorf("could not fetch include hostname configuration: %w", err)
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

func (r *f5osLoggingResource) fetchTLS(ctx context.Context, client_ *f5ossdk.F5os, baseURI string, state *f5osLoggingModel) error {
	log.Printf("[DEBUG] Fetching TLS configuration from API at path: %s", fmt.Sprintf("%s/f5-openconfig-system-logging:tls", baseURI))

	// Step 1: Make the API call
	tlsResp, err := client_.GetRequest(fmt.Sprintf("%s/f5-openconfig-system-logging:tls", baseURI))
	if err != nil {
		log.Printf("[ERROR] Failed to fetch TLS configuration: %v", err)
		// Fallback to default empty values
		state.TLS = types.ObjectValueMust(
			map[string]attr.Type{
				"certificate": types.StringType,
				"key":         types.StringType,
			},
			map[string]attr.Value{
				"certificate": types.StringValue(""),
				"key":         types.StringValue(""),
			},
		)
		return nil // Safe fallback
	}

	// Step 2: Parse the API response into a map
	var tlsData map[string]interface{}
	if err := json.Unmarshal(tlsResp, &tlsData); err != nil {
		log.Printf("[ERROR] Failed to parse TLS response: %v", err)
		// Fallback to default empty values
		state.TLS = types.ObjectValueMust(
			map[string]attr.Type{
				"certificate": types.StringType,
				"key":         types.StringType,
			},
			map[string]attr.Value{
				"certificate": types.StringValue(""),
				"key":         types.StringValue(""),
			},
		)
		return nil
	}

	// Step 3: Initialize default values for TLS attributes
	certificate := types.StringValue("")
	key := types.StringValue("")

	// Step 4: Extract values from the parsed API response
	if tlsConfig, ok := tlsData["f5-openconfig-system-logging:tls"].(map[string]interface{}); ok {
		if certValue, certOK := tlsConfig["certificate"]; certOK {
			certificate = types.StringValue(fmt.Sprintf("%v", certValue))
		} else {
			log.Printf("[WARN] Certificate missing; falling back to empty placeholder.")
		}

		if keyValue, keyOK := tlsConfig["key"]; keyOK {
			key = types.StringValue(fmt.Sprintf("%v", keyValue))
		} else {
			log.Printf("[WARN] Key missing; falling back to empty placeholder.")
		}
	}

	// Step 5: Construct the ObjectValue correctly
	tlsObject, diags := types.ObjectValueFrom(
		ctx,
		map[string]attr.Type{
			"certificate": types.StringType,
			"key":         types.StringType,
		},
		tlsFieldsStruct{
			Certificate: certificate.ValueString(),
			Key:         key.ValueString(),
		},
	)

	if diags.HasError() {
		log.Printf("[ERROR] Failed to create TLS ObjectValue: %v", diags)
		return fmt.Errorf("error constructing TLS ObjectValue: %v", diags)
	}

	// Step 6: Assign the constructed object to state
	state.TLS = tlsObject
	log.Printf("[DEBUG] Successfully fetched and parsed TLS configuration.")
	return nil
}

func (r *f5osLoggingResource) fetchCABundles(ctx context.Context, client *f5ossdk.F5os, baseURI string, state *f5osLoggingModel) error {
	caBundlesResp, err := client.GetRequest(fmt.Sprintf("%s/f5-openconfig-system-logging:tls/ca-bundles", baseURI))
	if err != nil {
		return fmt.Errorf("could not fetch CA bundle configuration: %w", err)
	}

	var caBundlesData map[string]interface{}
	var caBundleObjs []attr.Value

	if err := json.Unmarshal(caBundlesResp, &caBundlesData); err != nil {
		return fmt.Errorf("could not parse CA bundle response: %w", err)
	}

	if caBundles, ok := caBundlesData["ca-bundle"].([]interface{}); ok {
		for _, b := range caBundles {
			bundle := b.(map[string]interface{})
			obj, _ := types.ObjectValueFrom(
				ctx,
				map[string]attr.Type{
					"name":    types.StringType,
					"content": types.StringType,
				},
				map[string]attr.Value{
					"name":    types.StringValue(fmt.Sprintf("%v", bundle["name"])),
					"content": types.StringValue(fmt.Sprintf("%v", bundle["content"])),
				},
			)
			caBundleObjs = append(caBundleObjs, obj)
		}
	}

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

	// API Call
	rfResp, err := client.GetRequest(fmt.Sprintf("%s/f5-openconfig-system-logging:host-logs", baseURI))
	if err != nil {
		log.Printf("[ERROR] Failed to fetch Remote Forwarding configuration: %v", err)
		// Fallback to default empty values
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
							logObjects = append(logObjects, map[string]interface{}{
								"facility": strings.ToLower(fmt.Sprintf("%v", logEntry["facility"])),
								"severity": strings.ToLower(fmt.Sprintf("%v", logEntry["severity"])),
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
								"name": fmt.Sprintf("%v", fileEntry["name"]),
							})
						}
					}
				}
			}
		}
	}

	var logsSlice []remoteForwardingLog
	for _, logEntry := range logObjects {
		logsSlice = append(logsSlice, remoteForwardingLog{
			Facility: logEntry["facility"].(string),
			Severity: logEntry["severity"].(string),
		})
	}

	var filesSlice []remoteForwardingFile
	for _, fileEntry := range fileObjects {
		filesSlice = append(filesSlice, remoteForwardingFile{
			Name: fileEntry["name"].(string),
		})
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
	plan.State = types.StringValue("applied")
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

	// Delete TLS
	err := client.DeleteRequest(baseURI + "/f5-openconfig-system-logging:tls")
	if err != nil {
		resp.Diagnostics.AddError("TLS Delete Error", err.Error())
		return
	}

	// Delete CA Bundles
	err = client.DeleteRequest(baseURI + "/f5-openconfig-system-logging:tls/ca-bundles")
	if err != nil {
		resp.Diagnostics.AddError("CA Bundles Delete Error", err.Error())
		return
	}

	// Delete Remote Forwarding
	err = client.DeleteRequest(baseURI + "/f5-openconfig-system-logging:host-logs")
	if err != nil {
		resp.Diagnostics.AddError("Remote Forwarding Delete Error", err.Error())
		return
	}

	// Delete Servers
	err = client.DeleteRequest(baseURI + "/remote-servers")
	if err != nil {
		resp.Diagnostics.AddError("Servers Delete Error", err.Error())
		return
	}

	// Delete Include Hostname
	err = client.DeleteRequest(baseURI + "/f5-openconfig-system-logging:config")
	if err != nil {
		resp.Diagnostics.AddError("Include Hostname Delete Error", err.Error())
		return
	}
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
	if tlsObj.IsNull() || tlsObj.IsUnknown() {
		return nil
	}
	var tlsFields tlsFieldsStruct
	diags := tlsObj.As(ctx, &tlsFields, basetypes.ObjectAsOptions{})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return fmt.Errorf("diagnostic error during TLS object processing")
	}

	// Prepare CA bundles
	var caBundles []caBundleFields
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
		"f5-openconfig-system-logging:tls": map[string]interface{}{
			"certificate": tlsFields.Certificate,
			"key":         tlsFields.Key,
			"ca-bundles": map[string]interface{}{
				"ca-bundle": caBundleList,
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("TLS+CA Bundles Marshal Error", err.Error())
		return err
	}

	_, err = client.PutRequest(baseURI+"/f5-openconfig-system-logging:tls", payloadBytes)
	if err != nil {
		resp.Diagnostics.AddError("TLS+CA Bundles API Error", err.Error())
		return err
	}

	return nil
}

func createTLS(ctx context.Context, client *f5ossdk.F5os, baseURI string, tlsObj types.Object, resp *resource.CreateResponse) error {
	if tlsObj.IsNull() || tlsObj.IsUnknown() {
		return nil
	}

	type tlsFieldsStruct struct {
		Certificate string `tfsdk:"certificate"`
		Key         string `tfsdk:"key"`
	}
	var tlsFields tlsFieldsStruct
	diags := tlsObj.As(ctx, &tlsFields, basetypes.ObjectAsOptions{})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return fmt.Errorf("diagnostic error during TLS object processing")
	}

	// Prepare the API payload
	tlsPayload := map[string]interface{}{
		"f5-openconfig-system-logging:tls": map[string]interface{}{
			"certificate": tlsFields.Certificate,
			"key":         tlsFields.Key,
		},
	}
	payloadBytes, err := json.Marshal(tlsPayload)
	if err != nil {
		resp.Diagnostics.AddError("TLS Payload Marshal Error", err.Error())
		return err
	}

	// Submit payload to API
	_, err = client.PutRequest(baseURI+"/f5-openconfig-system-logging:tls", payloadBytes)
	if err != nil {
		resp.Diagnostics.AddError("TLS API Error", err.Error())
		return err
	}

	return nil
}

func createCABundles(ctx context.Context, client *f5ossdk.F5os, baseURI string, caBundlesObj types.List, resp *resource.CreateResponse) error {
	if caBundlesObj.IsNull() || caBundlesObj.IsUnknown() {
		return nil
	}
	var caBundles []caBundleFields
	diags := caBundlesObj.ElementsAs(ctx, &caBundles, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return fmt.Errorf("diagnostic error")
	}
	for _, bundle := range caBundles {
		payload := map[string]interface{}{
			"ca-bundle": map[string]interface{}{
				"name": bundle.Name,
				"config": map[string]interface{}{
					"name":    bundle.Name,
					"content": bundle.Content,
				},
			},
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			resp.Diagnostics.AddError("CA Bundle Marshal Error", err.Error())
			return err
		}
		uri := baseURI + fmt.Sprintf("/f5-openconfig-system-logging:tls/ca-bundles/ca-bundle=%s", bundle.Name)
		_, err = client.PutRequest(uri, payloadBytes)
		if err != nil {
			resp.Diagnostics.AddError("CA Bundle Create Error", err.Error())
			return err
		}
	}
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
		return nil
	}
	var servers []serverFields
	diags := serversObj.ElementsAs(ctx, &servers, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return fmt.Errorf("diagnostic error")
	}
	for _, server := range servers {
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
		if err != nil {
			resp.Diagnostics.AddError("Server Marshal Error", err.Error())
			return err
		}
		_, err = client.PostRequest(baseURI+"/remote-servers/", payloadBytes)
		if err != nil {
			// Try PUT if already exists
			_, putErr := client.PutRequest(baseURI+fmt.Sprintf("/remote-servers/remote-server=%s", server.Address), payloadBytes)
			if putErr != nil {
				resp.Diagnostics.AddError("Server Create Error", putErr.Error())
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
