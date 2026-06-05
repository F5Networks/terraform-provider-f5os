package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// Device stabilization timing constants. SSHD cipher/kex/mac/hkey changes
// cause the RESTCONF service to restart asynchronously, sometimes more than
// once. These values control the polling and cooldown behaviour used by
// waitForDeviceReady (provider) and waitForDeviceAvailable (tests).
const (
	// deviceStabilizeTimeout is how long to wait for the device after
	// Create/Update/Delete operations that modify SSHD settings.
	deviceStabilizeTimeout = 120 * time.Second

	// devicePollInterval is the sleep between consecutive availability checks.
	devicePollInterval = 5 * time.Second

	// deviceCooldown is the pause between two consecutive successful checks
	// required to confirm the device has truly stabilized.
	deviceCooldown = 30 * time.Second
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &SystemResource{}
var _ resource.ResourceWithImportState = &SystemResource{}

func NewSystemResource() resource.Resource {
	return &SystemResource{}
}

// SystemResource defines the resource implementation.
type SystemResource struct {
	client *f5ossdk.F5os
}

type SystemResourceModel struct {
	Hostname         types.String `tfsdk:"hostname"`
	Motd             types.String `tfsdk:"motd"`
	LoginBanner      types.String `tfsdk:"login_banner"`
	Timezone         types.String `tfsdk:"timezone"`
	CliTimeout       types.Int64  `tfsdk:"cli_timeout"`
	TokenLifetime    types.Int64  `tfsdk:"token_lifetime"`
	HttpdCipherSuite types.String `tfsdk:"httpd_ciphersuite"`
	SshdIdleTimeout  types.String `tfsdk:"sshd_idle_timeout"`
	SshdCiphers      types.List   `tfsdk:"sshd_ciphers"`
	SshdKeyAlg       types.List   `tfsdk:"sshd_kex_alg"`
	SshdMacAlg       types.List   `tfsdk:"sshd_mac_alg"`
	SshdHkeyAlg      types.List   `tfsdk:"sshd_hkey_alg"`
	Id               types.String `tfsdk:"id"`
}

func (r *SystemResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system"
}

func (r *SystemResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Resource to manage generic system settings",

		Attributes: map[string]schema.Attribute{
			"hostname": schema.StringAttribute{
				MarkdownDescription: "System Hostname",
				Required:            true,
			},
			"motd": schema.StringAttribute{
				MarkdownDescription: "Message of the day",
				Optional:            true,
			},
			"login_banner": schema.StringAttribute{
				MarkdownDescription: "Login Banner",
				Optional:            true,
			},
			"timezone": schema.StringAttribute{
				MarkdownDescription: "Timezone for the system per TZ database name",
				Optional:            true,
			},
			"cli_timeout": schema.Int64Attribute{
				MarkdownDescription: "CLI idle timeout",
				Optional:            true,
			},
			"token_lifetime": schema.Int64Attribute{
				MarkdownDescription: "Token lifetime length in minutes",
				Optional:            true,
			},
			"httpd_ciphersuite": schema.StringAttribute{
				MarkdownDescription: "HTTPS Ciphersuite in OpenSSL format",
				Optional:            true,
			},
			"sshd_idle_timeout": schema.StringAttribute{
				MarkdownDescription: "SSH Idle timeout",
				Optional:            true,
			},
			"sshd_ciphers": schema.ListAttribute{
				MarkdownDescription: "List of httpd ciphersuite in OpenSSL format",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"sshd_kex_alg": schema.ListAttribute{
				MarkdownDescription: "List of sshd key exchange algorithms in OpenSSH format",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"sshd_mac_alg": schema.ListAttribute{
				MarkdownDescription: "List of sshd Mac algorithms in OpenSSH format",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"sshd_hkey_alg": schema.ListAttribute{
				MarkdownDescription: "List of the sshd host key algorithms in OpenSSH format",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for Interface resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *SystemResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (r *SystemResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *SystemResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, fmt.Sprintf("[CREATE] Config System :%+v", data.Hostname.ValueString()))

	systemConfigReq := getSystemConfig(ctx, data)

	byteBody, err := json.Marshal(systemConfigReq)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating System payload, got error: %s", err))
		return
	}
	res, err := r.client.PatchRequest("/openconfig-system:system", byteBody)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while creating System, got error: %s", err))
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("[CREATE], System Resp:%+v", string(res)))

	if !data.TokenLifetime.IsNull() && !data.TokenLifetime.IsUnknown() {
		tokenLifetimeReq := getTokenLifetimeConfig(ctx, data)

		byteBody, err := json.Marshal(tokenLifetimeReq)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating Token Lifetime payload, got error: %s", err))
			return
		}

		res, err := r.client.PatchRequest("/openconfig-system:system/aaa", byteBody)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while Patching Token Lifetime, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[CREATE], Token Lifetime Resp:%+v", string(res)))
	}

	if (!data.CliTimeout.IsNull() && !data.CliTimeout.IsUnknown()) || (!data.SshdIdleTimeout.IsNull() && !data.SshdIdleTimeout.IsUnknown()) {
		systemSettingsReq := getSystemSettingsConfig(ctx, data)

		byteBody, err := json.Marshal(systemSettingsReq)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating System Settings payload, got error: %s", err))
			return
		}
		tflog.Debug(ctx, fmt.Sprintf("[CREATE], Request Body  %+v", string(byteBody)))

		res, err := r.client.PatchRequest("/openconfig-system:system/f5-system-settings:settings", byteBody)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while Patching System Settings, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[CREATE], System Settings Resp:%+v", string(res)))
	}

	baseURL := "/openconfig-system:system/f5-security-ciphers:security/services/service"
	serviceName := "httpd"
	fullURI := fmt.Sprintf(`%s=%s/config`, baseURL, serviceName)

	if !data.HttpdCipherSuite.IsNull() && !data.HttpdCipherSuite.IsUnknown() {
		cipherSuiteReq := getHttpdCipherConfig(ctx, data)

		byteBody, err := json.Marshal(cipherSuiteReq)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating Http Cipher payload, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[CREATE], Full Uri:%+v", fullURI))
		res, err := r.client.PutRequest(fullURI, byteBody)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while Http Cipher, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[CREATE], Http Cipher Resp:%+v", string(res)))
	}

	serviceName = "sshd"
	fullURI = fmt.Sprintf(`%s=%s/config`, baseURL, serviceName)

	if !data.SshdCiphers.IsNull() && !data.SshdCiphers.IsUnknown() {
		cipherSuiteReq := getSshdCipherConfig(ctx, data)

		byteBody, err := json.Marshal(cipherSuiteReq)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating Sshd Cipher payload, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[CREATE], Full Uri:%+v", fullURI))
		res, err := r.client.PutRequest(fullURI+"/ciphers", byteBody)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while Sshd Cipher, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[CREATE], Sshd Cipher Resp:%+v", string(res)))

	}

	if !data.SshdKeyAlg.IsNull() && !data.SshdKeyAlg.IsUnknown() {
		keyExchangeAlgoReq := getKeyExchangeAlgoConfig(ctx, data)

		byteBody, err := json.Marshal(keyExchangeAlgoReq)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating Key Exchange Algo payload, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[CREATE], Full Uri:%+v", fullURI))
		res, err := r.client.PutRequest(fullURI+"/kexalgorithms", byteBody)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while creating Sshd Key Algo, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[CREATE], Sshd Key Algo Resp:%+v", string(res)))

	}

	if !data.SshdMacAlg.IsNull() && !data.SshdMacAlg.IsUnknown() {
		sshdMacAlgReq := getSshdMacConfig(ctx, data)

		byteBody, err := json.Marshal(sshdMacAlgReq)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating Mac Algo payload, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[CREATE], Full Uri:%+v", fullURI))
		res, err := r.client.PutRequest(fullURI+"/macs", byteBody)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while creating Mac Algo, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[CREATE], Mac Algo Resp:%+v", string(res)))

	}

	if !data.SshdHkeyAlg.IsNull() && !data.SshdHkeyAlg.IsUnknown() {
		sshHostKeyAlgoReq := getSshdHKeyAlgoConfig(ctx, data)

		byteBody, err := json.Marshal(sshHostKeyAlgoReq)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating Hostname Key Algo payload, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[CREATE], Full Uri:%+v", fullURI))
		res, err := r.client.PutRequest(fullURI+"/host-key-algorithms", byteBody)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while creating Ssh Host Key Algo, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[CREATE], Ssh Host Key Algo Resp:%+v", string(res)))

	}

	// SSHD cipher/kex/mac/hkey changes cause the RESTCONF service to restart
	// asynchronously. Wait for the device to stabilize before returning so
	// the framework's automatic post-Create Read succeeds.
	if sshdChanged := !data.SshdCiphers.IsNull() || !data.SshdKeyAlg.IsNull() ||
		!data.SshdMacAlg.IsNull() || !data.SshdHkeyAlg.IsNull(); sshdChanged {
		if err := r.waitForDeviceReady(ctx, deviceStabilizeTimeout); err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("device did not stabilize after SSHD changes: %s", err))
			return
		}
	}

	data.Id = data.Hostname
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

func (r *SystemResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *SystemResourceModel
	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprint("[READ]", "Read System Config"))
	system, err := r.client.GetRequest("/openconfig-system:system/config")
	if err != nil {
		resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while fetching System Config, got error: %s", err))
		return
	}
	resSystemConfig := &f5ossdk.F5ResSystemConfig{}
	err = json.Unmarshal(system, resSystemConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("error: %s", err))
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("Response:%+v", string(system)))

	// Clock Settings
	tflog.Debug(ctx, fmt.Sprint("[READ]", "Clock Settings"))
	clock, err := r.client.GetRequest("/openconfig-system:system/clock")

	if err != nil {
		resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while fetching Clock Config, got error: %s", err))
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("[GetSystemClock] Response:%+v", string(clock)))

	resClockConfig := &f5ossdk.F5ResClockConfig{}
	err = json.Unmarshal(clock, resClockConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("error: %s", err))
		return
	}
	// security ciphers settings
	ciphers, err := r.client.GetRequest("/openconfig-system:system/f5-security-ciphers:security/services/service")

	if err != nil {
		resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while fetching Ciphers Config, got error: %s", err))
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("[GetSystemCiphers]Response:%+v", string(ciphers)))
	resSshdBlock := &f5ossdk.SshdBlock{}
	resHttpdBlock := &f5ossdk.HttpdBlock{}

	var raw map[string][]map[string]any
	err = json.Unmarshal([]byte(ciphers), &raw)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("error: %s", err))
		return
	}

	services := raw["f5-security-ciphers:service"]
	for _, svc := range services {
		switch svc["name"] {
		case "httpd":
			b, _ := json.Marshal(svc)
			err = json.Unmarshal(b, resHttpdBlock)
			if err != nil {
				resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("error: %s", err))
				return
			}
		case "sshd":
			b, _ := json.Marshal(svc)
			err = json.Unmarshal(b, resSshdBlock)
			if err != nil {
				resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("error: %s", err))
				return
			}
		}
	}
	// system settings
	settings, err := r.client.GetRequest("/openconfig-system:system/f5-system-settings:settings")

	if err != nil {
		resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while fetching Settings Config, got error: %s", err))
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("[GetSystemSettings] Response:%+v", string(settings)))
	resSystemSettingsConfig := &f5ossdk.F5ResSettingsConfig{}
	err = json.Unmarshal(settings, resSystemSettingsConfig)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("error: %s", err))
		return
	}

	// token lifetime
	lifetime, err := r.client.GetRequest("/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime")
	if err != nil {
		resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while fetching Token Lifetime, got error: %s", err))
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("[GetSystemLifetime] Response:%+v", string(lifetime)))
	resTokenLifetime := &f5ossdk.F5ResTokenLifetime{}
	err = json.Unmarshal(lifetime, resTokenLifetime)

	if err != nil {
		resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("error: %s", err))
		return
	}

	// Hostname is null only during import (ImportState sets id alone).
	isImport := data.Hostname.IsNull()

	r.SystemResourceModelToState(ctx, resSystemConfig, resClockConfig, resHttpdBlock, resSshdBlock, resSystemSettingsConfig, resTokenLifetime, data, isImport)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SystemResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *SystemResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, fmt.Sprintf("[UPDATE] Config System :%+v", data.Hostname.ValueString()))

	if !data.TokenLifetime.IsNull() && !data.TokenLifetime.IsUnknown() {
		tokenLifetimeReq := getTokenLifetimeConfig(ctx, data)

		byteBody, err := json.Marshal(tokenLifetimeReq)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating Token Lifetime payload, got error: %s", err))
			return
		}

		res, err := r.client.PatchRequest("/openconfig-system:system/aaa", byteBody)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while Patching Token Lifetime, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[UPDATE], Token Lifetime Resp:%+v", string(res)))
	}

	if (!data.CliTimeout.IsNull() && !data.CliTimeout.IsUnknown()) || (!data.SshdIdleTimeout.IsNull() && !data.SshdIdleTimeout.IsUnknown()) {
		systemSettingsReq := getSystemSettingsConfig(ctx, data)

		byteBody, err := json.Marshal(systemSettingsReq)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating System Settings payload, got error: %s", err))
			return
		}
		tflog.Debug(ctx, fmt.Sprintf("[UPDATE], Request Body  %+v", string(byteBody)))

		res, err := r.client.PatchRequest("/openconfig-system:system/f5-system-settings:settings", byteBody)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while Patching System Settings, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[UPDATE], System Settings Resp:%+v", string(res)))
	}

	baseURL := "/openconfig-system:system/f5-security-ciphers:security/services/service"
	serviceName := "httpd"
	fullURI := fmt.Sprintf(`%s=%s/config`, baseURL, serviceName)

	if !data.HttpdCipherSuite.IsNull() && !data.HttpdCipherSuite.IsUnknown() {
		cipherSuiteReq := getHttpdCipherConfig(ctx, data)

		byteBody, err := json.Marshal(cipherSuiteReq)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating Http Cipher payload, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[UPDATE], Full Uri:%+v", fullURI))
		res, err := r.client.PutRequest(fullURI, byteBody)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while Http Cipher, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[UPDATE], Http Cipher Resp:%+v", string(res)))
	}

	serviceName = "sshd"
	fullURI = fmt.Sprintf(`%s=%s/config`, baseURL, serviceName)

	if !data.SshdCiphers.IsNull() && !data.SshdCiphers.IsUnknown() {
		cipherSuiteReq := getSshdCipherConfig(ctx, data)

		byteBody, err := json.Marshal(cipherSuiteReq)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating Sshd Cipher payload, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[UPDATE], Full Uri:%+v", fullURI))
		res, err := r.client.PutRequest(fullURI+"/ciphers", byteBody)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while Sshd Cipher, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[UPDATE], Sshd Cipher Resp:%+v", string(res)))

	}

	if !data.SshdKeyAlg.IsNull() && !data.SshdKeyAlg.IsUnknown() {
		keyExchangeAlgoReq := getKeyExchangeAlgoConfig(ctx, data)

		byteBody, err := json.Marshal(keyExchangeAlgoReq)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating Key Exchange Algo payload, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[UPDATE], Full Uri:%+v", fullURI))
		res, err := r.client.PutRequest(fullURI+"/kexalgorithms", byteBody)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while creating Sshd Key Algo, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[UPDATE], Sshd Key Algo Resp:%+v", string(res)))

	}

	if !data.SshdMacAlg.IsNull() && !data.SshdMacAlg.IsUnknown() {
		sshdMacAlgReq := getSshdMacConfig(ctx, data)

		byteBody, err := json.Marshal(sshdMacAlgReq)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating Mac Algo payload, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[UPDATE], Full Uri:%+v", fullURI))
		res, err := r.client.PutRequest(fullURI+"/macs", byteBody)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while creating Mac Algo, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[UPDATE], Mac Algo Resp:%+v", string(res)))

	}

	if !data.SshdHkeyAlg.IsNull() && !data.SshdHkeyAlg.IsUnknown() {
		sshHostKeyAlgoReq := getSshdHKeyAlgoConfig(ctx, data)

		byteBody, err := json.Marshal(sshHostKeyAlgoReq)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating Hostname Key Algo payload, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[UPDATE], Full Uri:%+v", fullURI))
		res, err := r.client.PutRequest(fullURI+"/host-key-algorithms", byteBody)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while creating Ssh Host Key Algo, got error: %s", err))
			return
		}

		tflog.Debug(ctx, fmt.Sprintf("[UPDATE], Ssh Host Key Algo Resp:%+v", string(res)))

	}

	systemConfigReq := getSystemConfig(ctx, data)

	byteBody, err := json.Marshal(systemConfigReq)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("failure while creating System payload, got error: %s", err))
		return
	}

	res, err := r.client.PatchRequest("/openconfig-system:system", byteBody)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("failure while creating System, got error: %s", err))
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("[CREATE], System Resp:%+v", string(res)))

	// SSHD cipher/kex/mac/hkey changes cause the RESTCONF service to restart
	// asynchronously. Wait for the device to stabilize before returning so
	// the framework's automatic post-Update Read succeeds. Only wait when an
	// SSHD attribute actually changed (plan differs from prior state), not
	// merely when it is present — avoiding an unnecessary cooldown when only
	// non-SSHD fields were updated.
	var prior SystemResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if sshdAttributesChanged(data, &prior) {
		if err := r.waitForDeviceReady(ctx, deviceStabilizeTimeout); err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("device did not stabilize after SSHD changes: %s", err))
			return
		}
	}

	data.Id = data.Hostname
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// sshdAttributesChanged returns true if any SSHD-related attribute differs
// between the plan and the prior state, indicating a change that may trigger
// a RESTCONF service restart.
func sshdAttributesChanged(plan, prior *SystemResourceModel) bool {
	return !plan.SshdCiphers.Equal(prior.SshdCiphers) ||
		!plan.SshdKeyAlg.Equal(prior.SshdKeyAlg) ||
		!plan.SshdMacAlg.Equal(prior.SshdMacAlg) ||
		!plan.SshdHkeyAlg.Equal(prior.SshdHkeyAlg)
}

// Delete only guards optional fields with IsNull() (not IsUnknown()) because
// state values in Delete are never Unknown — Unknown only appears during planning.
// Create/Update use both IsNull() && IsUnknown() checks since plan values can be Unknown.
func (r *SystemResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *SystemResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := "/openconfig-system:system/config"
	paths := []string{"login-banner", "hostname", "motd-banner"}
	for _, path := range paths {
		err := r.client.DeleteRequest(fmt.Sprintf(`%s/%s`, baseURL, path))
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("Failure while Deleting System Config, got error: %s", err))
			return
		}
	}

	if !data.TokenLifetime.IsNull() {
		baseURL = "/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/config/lifetime/lifetime"
		err := r.client.DeleteRequest(baseURL)
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("Failure while Deleting Token Lifetime, got error: %s", err))
			return
		}
	}

	baseURL = "/openconfig-system:system/f5-system-settings:settings"
	if !data.CliTimeout.IsNull() {
		err := r.client.DeleteRequest(fmt.Sprintf(`%s/%s`, baseURL, "idle-timeout"))
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("Failure while Deleting System Settings, got error: %s", err))
			return
		}
	}
	if !data.SshdIdleTimeout.IsNull() {
		err := r.client.DeleteRequest(fmt.Sprintf(`%s/%s`, baseURL, "sshd-idle-timeout"))
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("Failure while Deleting System Settings, got error: %s", err))
			return
		}
	}

	baseURL = "/openconfig-system:system/f5-security-ciphers:security/services/service"
	if !data.HttpdCipherSuite.IsNull() {
		err := r.client.DeleteRequest(fmt.Sprintf(`%s="httpd"/config/%s`, baseURL, "ssl-cipher-suite"))
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("Failure while Deleting System Settings, got error: %s", err))
			return
		}
	}
	if !data.SshdCiphers.IsNull() {
		err := r.client.DeleteRequest(fmt.Sprintf(`%s="sshd"/config/%s`, baseURL, "ciphers"))
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("Failure while Deleting Ciphers Config, got error: %s", err))
			return
		}
	}
	if !data.SshdKeyAlg.IsNull() {
		err := r.client.DeleteRequest(fmt.Sprintf(`%s="sshd"/config/%s`, baseURL, "kexalgorithms"))
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("Failure while Deleting Ciphers Config, got error: %s", err))
			return
		}
	}
	if !data.SshdMacAlg.IsNull() {
		err := r.client.DeleteRequest(fmt.Sprintf(`%s="sshd"/config/%s`, baseURL, "macs"))
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("Failure while Deleting Ciphers Config, got error: %s", err))
			return
		}
	}
	if !data.SshdHkeyAlg.IsNull() {
		err := r.client.DeleteRequest(fmt.Sprintf(`%s="sshd"/config/%s`, baseURL, "host-key-algorithms"))
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("Failure while Deleting Ciphers Config, got error: %s", err))
			return
		}
	}

	// SSHD cipher/kex/mac/hkey deletions cause the RESTCONF service to restart
	// asynchronously. Wait for the device to stabilize so that subsequent
	// operations (e.g., a new test creating the resource) succeed.
	// Note: For Delete, we log but don't fail if stabilization times out since
	// the resource removal itself completed successfully.
	if sshdChanged := !data.SshdCiphers.IsNull() || !data.SshdKeyAlg.IsNull() ||
		!data.SshdMacAlg.IsNull() || !data.SshdHkeyAlg.IsNull(); sshdChanged {
		if err := r.waitForDeviceReady(ctx, deviceStabilizeTimeout); err != nil {
			tflog.Warn(ctx, fmt.Sprintf("[DELETE] %s", err))
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *SystemResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// waitForDeviceReady polls the device RESTCONF API until it responds
// successfully on both the system config and cipher service endpoints,
// with a cooldown period to confirm stability. SSHD cipher/kex/mac/hkey
// changes cause the RESTCONF service to restart asynchronously; this
// method should be called after such changes to ensure the subsequent
// Read succeeds.
func (r *SystemResource) waitForDeviceReady(ctx context.Context, timeout time.Duration) error {
	check := func() bool {
		_, err := r.client.GetRequest("/openconfig-system:system/config")
		if err != nil {
			return false
		}
		_, err = r.client.GetRequest("/openconfig-system:system/f5-security-ciphers:security/services/service")
		if err != nil {
			return false
		}
		_, err = r.client.GetRequest("/openconfig-system:system/aaa")
		return err == nil
	}

	if err := pollUntilStable(check, timeout); err != nil {
		return err
	}
	tflog.Info(ctx, "[waitForDeviceReady] Device stabilized after SSHD changes")
	return nil
}

// pollUntilStable is the shared polling/cooldown loop used by both the
// provider (waitForDeviceReady) and tests (waitForDeviceAvailable).
// It calls check repeatedly until two consecutive successes are separated
// by a cooldown period, confirming the device has truly stabilized.
// The cooldown is capped to the remaining deadline to prevent overshoot.
//
// When F5OS_POLL_INTERVAL is set (unit test mode), the cooldown and interval
// are reduced to 1ms so that unit tests are not blocked by the 30-second
// stabilization wait that real devices need.
func pollUntilStable(check func() bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	interval := devicePollInterval
	cooldown := deviceCooldown

	// In unit tests, F5OS_POLL_INTERVAL is set to a short value (e.g. "1ms").
	// Honor that for the cooldown and interval so tests run fast.
	if v := os.Getenv("F5OS_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
			cooldown = d
		}
	}

	for time.Now().Before(deadline) {
		if check() {
			// Cap cooldown to remaining time before deadline.
			remaining := time.Until(deadline)
			if remaining < cooldown {
				cooldown = remaining
			}
			if cooldown <= 0 {
				break
			}
			time.Sleep(cooldown)
			if check() {
				return nil
			}
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("device did not stabilize within %v", timeout)
}

func (r *SystemResource) SystemResourceModelToState(ctx context.Context, resSystemConfig *f5ossdk.F5ResSystemConfig, resClockConfig *f5ossdk.F5ResClockConfig,
	resHttpdBlock *f5ossdk.HttpdBlock, resSshdBlock *f5ossdk.SshdBlock, resSystemSettingsConfig *f5ossdk.F5ResSettingsConfig,
	resTokenLifetime *f5ossdk.F5ResTokenLifetime, data *SystemResourceModel, isImport bool) {

	data.Hostname = types.StringValue(resSystemConfig.OpenConfigSystem.Hostname)

	data.Motd = types.StringValue(resSystemConfig.OpenConfigSystem.Motd)
	data.LoginBanner = types.StringValue(resSystemConfig.OpenConfigSystem.LoginBanner)

	data.Timezone = types.StringValue(resClockConfig.OpenConfigClock.Config.TimeZoneName)

	if !data.HttpdCipherSuite.IsNull() || isImport {
		data.HttpdCipherSuite = types.StringValue(resHttpdBlock.Config.SSLCipherSuite)
	}

	if !data.SshdCiphers.IsNull() || isImport {
		data.SshdCiphers, _ = types.ListValueFrom(ctx, types.StringType, resSshdBlock.Config.Ciphers)
	}
	if !data.SshdMacAlg.IsNull() || isImport {
		data.SshdMacAlg, _ = types.ListValueFrom(ctx, types.StringType, resSshdBlock.Config.MACs)
	}
	if !data.SshdKeyAlg.IsNull() || isImport {
		data.SshdKeyAlg, _ = types.ListValueFrom(ctx, types.StringType, resSshdBlock.Config.KexAlgorithms)
	}
	if !data.SshdHkeyAlg.IsNull() || isImport {
		data.SshdHkeyAlg, _ = types.ListValueFrom(ctx, types.StringType, resSshdBlock.Config.HostKeyAlgos)
	}

	if !data.CliTimeout.IsNull() || isImport {
		switch v := resSystemSettingsConfig.Settings.Config.CliTimeout.(type) {
		case string:
			id, _ := strconv.Atoi(v)
			data.CliTimeout = types.Int64Value(int64(id))
		case int:
			data.CliTimeout = types.Int64Value(int64(v))
		case float64:
			data.CliTimeout = types.Int64Value(int64(v))
		default:
			// Unrecognized type; leave CliTimeout unchanged
		}
	}

	if !data.SshdIdleTimeout.IsNull() || isImport {
		if s, ok := resSystemSettingsConfig.Settings.Config.SshdIdleTimeout.(string); ok {
			data.SshdIdleTimeout = types.StringValue(s)
		}
	}

	if !data.TokenLifetime.IsNull() || isImport {
		data.TokenLifetime = types.Int64Value(int64(resTokenLifetime.Lifetime))
	}

}

func getTokenLifetimeConfig(ctx context.Context, data *SystemResourceModel) *f5ossdk.F5ReqTokenLifetime {
	tokenLifetimeReq := f5ossdk.F5ReqTokenLifetime{}
	tokenLifetimeReq.OpenConfigSystem.RestConfigToken.Config.Lifetime = int(data.TokenLifetime.ValueInt64())
	return &tokenLifetimeReq
}

func getSystemSettingsConfig(ctx context.Context, data *SystemResourceModel) *f5ossdk.F5ReqSystemSettingConfig {
	settingsReq := f5ossdk.F5ReqSystemSettingConfig{}
	settingsReq.Settings.Config.CliTimeout = int(data.CliTimeout.ValueInt64())
	settingsReq.Settings.Config.SshdIdleTimeout = data.SshdIdleTimeout.ValueString()
	return &settingsReq
}

func getHttpdCipherConfig(ctx context.Context, data *SystemResourceModel) *f5ossdk.F5ReqHttpCipherConfig {
	httpCipherSuite := f5ossdk.F5ReqHttpCipherConfig{}
	httpCipherSuite.Config.Name = "httpd"
	httpCipherSuite.Config.SslCipherSuite = data.HttpdCipherSuite.ValueString()
	return &httpCipherSuite
}

func getSshdCipherConfig(ctx context.Context, data *SystemResourceModel) *f5ossdk.F5ReqSshdCipherConfig {
	sshdCipherSuite := f5ossdk.F5ReqSshdCipherConfig{}
	data.SshdCiphers.ElementsAs(ctx, &sshdCipherSuite.Ciphers, false)
	return &sshdCipherSuite
}

func getKeyExchangeAlgoConfig(ctx context.Context, data *SystemResourceModel) *f5ossdk.F5ReqSshdKeyAlgConfig {
	keyExchangeAlgoReq := f5ossdk.F5ReqSshdKeyAlgConfig{}
	data.SshdKeyAlg.ElementsAs(ctx, &keyExchangeAlgoReq.KeyExchangeAlgorithms, false)
	return &keyExchangeAlgoReq
}

func getSshdMacConfig(ctx context.Context, data *SystemResourceModel) *f5ossdk.F5ReqSshdMacConfig {
	sshdMacAlg := f5ossdk.F5ReqSshdMacConfig{}
	data.SshdMacAlg.ElementsAs(ctx, &sshdMacAlg.Macs, false)
	return &sshdMacAlg
}

func getSshdHKeyAlgoConfig(ctx context.Context, data *SystemResourceModel) *f5ossdk.F5ReqSshdHkeyAlgConfig {
	sshdHKeyAlg := f5ossdk.F5ReqSshdHkeyAlgConfig{}
	data.SshdHkeyAlg.ElementsAs(ctx, &sshdHKeyAlg.HostKeyAlgorithms, false)
	return &sshdHKeyAlg
}

func getSystemConfig(ctx context.Context, data *SystemResourceModel) *f5ossdk.F5ReqSystemConfig {
	tflog.Debug(ctx, fmt.Sprintf("[getSystemConfig], getSystemConfig:%+v", data))
	reqSystemConfig := f5ossdk.F5ReqSystemConfig{}
	reqSystemConfig.OpenConfigSystem.Clock.Config.TimezoneName = data.Timezone.ValueString()
	reqSystemConfig.OpenConfigSystem.Config.Hostname = data.Hostname.ValueString()
	reqSystemConfig.OpenConfigSystem.Config.Motd = data.Motd.ValueString()
	reqSystemConfig.OpenConfigSystem.Config.LoginBanner = data.LoginBanner.ValueString()
	return &reqSystemConfig
}
