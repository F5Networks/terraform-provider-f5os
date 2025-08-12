package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
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

	if (!data.CliTimeout.IsNull() && !data.CliTimeout.IsUnknown()) || (data.SshdIdleTimeout.IsNull() && data.SshdIdleTimeout.IsUnknown()) {
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

	r.SystemResourceModelToState(ctx, resSystemConfig, resClockConfig, resHttpdBlock, resSshdBlock, resSystemSettingsConfig, resTokenLifetime, data)

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

	if (!data.CliTimeout.IsNull() && !data.CliTimeout.IsUnknown()) || (data.SshdIdleTimeout.IsNull() && data.SshdIdleTimeout.IsUnknown()) {
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

	data.Id = data.Hostname
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

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

	baseURL = "/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/config/lifetime/lifetime"
	err := r.client.DeleteRequest(baseURL)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("Failure while Deleting Token Lifetime, got error: %s", err))
		return
	}

	baseURL = "/openconfig-system:system/f5-system-settings:settings"
	paths = []string{"idle-timeout", "sshd-idle-timeout"}
	for _, path := range paths {
		err := r.client.DeleteRequest(fmt.Sprintf(`%s/%s`, baseURL, path))
		if err != nil {
			resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("Failure while Deleting System Settings, got error: %s", err))
			return
		}
	}

	baseURL = "/openconfig-system:system/f5-security-ciphers:security/services/service"
	paths = []string{"ssl-cipher-suite", "ciphers", "kexalgorithms", "macs", "host-key-algorithms"}
	// services := []string{"httpd", "sshd"}

	for _, path := range paths {
		if path == "ssl-cipher-suite" {
			err := r.client.DeleteRequest(fmt.Sprintf(`%s="httpd"/config/%s`, baseURL, path))
			if err != nil {
				resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("Failure while Deleting System Settings, got error: %s", err))
				return
			}
		} else {
			err := r.client.DeleteRequest(fmt.Sprintf(`%s="sshd"/config/%s`, baseURL, path))
			if err != nil {
				resp.Diagnostics.AddError("F5OS Error:", fmt.Sprintf("Failure while Deleting Ciphers Config, got error: %s", err))
				return
			}
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *SystemResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *SystemResource) SystemResourceModelToState(ctx context.Context, resSystemConfig *f5ossdk.F5ResSystemConfig, resClockConfig *f5ossdk.F5ResClockConfig,
	resHttpdBlock *f5ossdk.HttpdBlock, resSshdBlock *f5ossdk.SshdBlock, resSystemSettingsConfig *f5ossdk.F5ResSettingsConfig,
	resTokenLifetime *f5ossdk.F5ResTokenLifetime, data *SystemResourceModel) {

	data.Hostname = types.StringValue(resSystemConfig.OpenConfigSystem.Hostname)

	data.Motd = types.StringValue(resSystemConfig.OpenConfigSystem.Motd)
	data.LoginBanner = types.StringValue(resSystemConfig.OpenConfigSystem.LoginBanner)

	data.Timezone = types.StringValue(resClockConfig.OpenConfigClock.Config.TimeZoneName)

	data.HttpdCipherSuite = types.StringValue(resHttpdBlock.Config.SSLCipherSuite)

	data.SshdCiphers.ElementsAs(ctx, &resSshdBlock.Config.Ciphers, false)
	data.SshdMacAlg.ElementsAs(ctx, &resSshdBlock.Config.MACs, false)
	data.SshdKeyAlg.ElementsAs(ctx, &resSshdBlock.Config.KexAlgorithms, false)
	data.SshdHkeyAlg.ElementsAs(ctx, &resSshdBlock.Config.HostKeyAlgos, false)

	switch v := resSystemSettingsConfig.Settings.Config.CliTimeout.(type) {
	case string:
		id, _ := strconv.Atoi(v)
		data.CliTimeout = types.Int64Value(int64(id))
	case int:
		id := int(v)
		data.CliTimeout = types.Int64Value(int64(id))
	default:
		// Use id
	}

	// data.CliTimeout = types.Int64Value(int64(resSystemSettingsConfig.Settings.Config.CliTimeout.(int)))
	data.SshdIdleTimeout = types.StringValue(resSystemSettingsConfig.Settings.Config.SshdIdleTimeout.(string))

	data.TokenLifetime = types.Int64Value(int64(resTokenLifetime.Lifetime))

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
