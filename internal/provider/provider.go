package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// Ensure F5osProvider satisfies various provider interfaces.
var _ provider.Provider = &F5osProvider{}

// F5osProvider defines the provider implementation.
type F5osProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// F5osProviderModel describes the provider data model.
type F5osProviderModel struct {
	Host             types.String `tfsdk:"host"`
	Username         types.String `tfsdk:"username"`
	Password         types.String `tfsdk:"password"`
	Port             types.Int64  `tfsdk:"port"`
	TeemDisable      types.Bool   `tfsdk:"teem_disable"`
	DisableSslVerify types.Bool   `tfsdk:"disable_tls_verify"`
	CustomHeaders    types.Map    `tfsdk:"custom_headers"`
}
type TeemData struct {
	ResourceName      string
	ProviderName      string
	ProviderVersion   string
	TerraformVersion  string
	F5Platform        string
	F5SoftwareVersion string
	TerraformLicense  string
}

var teemData = &TeemData{}

// sessionCache holds *f5ossdk.F5os clients keyed by a hash of the
// connection parameters that affect authentication. The framework
// invokes Provider.Configure once per plan/apply/refresh phase, which
// on F5OS 2.0 is enough to trip the device's auth rate-limit and
// start returning 401s mid-test. Reusing a single client per unique
// (host, port, user, password, tls, headers) tuple within the process
// keeps the auth count at 1 for the duration of the run.
//
// The cache lives for the lifetime of the process; there is no
// eviction. In production the provider process is short-lived
// (Terraform spawns and tears down the plugin per invocation), and in
// tests the process ends when `go test` exits, so unbounded growth is
// not a concern for realistic key cardinality.
var (
	sessionCacheMu sync.Mutex
	sessionCache   = map[string]*f5ossdk.F5os{}
)

// sessionCacheKey derives a stable, opaque key from the fields that
// determine session identity. The key itself is a SHA-256 digest so
// the map key does not contain the plaintext password.
//
// Security caveat: the cached value is a *f5ossdk.F5os client, which
// currently retains the plaintext password on the struct itself (see
// vendor/gitswarm.f5net.com/terraform-providers/f5osclient/f5os.go).
// Hashing the key therefore does NOT prevent credentials from being
// resident in process memory for the lifetime of the cached client.
// The design tradeoff is intentional: the provider process is
// short-lived (Terraform spawns and tears down the plugin per
// invocation) and the alternative — re-authenticating on every
// Configure — trips F5OS 2.0's stricter auth rate-limit. A future
// refactor could store token-only sessions or clear the password
// after the initial NewSession returns.
func sessionCacheKey(host string, port int, user, password string, disableSSL bool, headers map[string]string) string {
	h := sha256.New()
	fmt.Fprintf(h, "host=%s\x00port=%d\x00user=%s\x00pw=%s\x00tlsSkip=%t\x00", host, port, user, password, disableSSL)
	// Sort header keys so map iteration order doesn't perturb the key.
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(h, "hdr:%s=%s\x00", k, headers[k])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// isTerraformVersionAtLeast returns true when v (e.g. "1.10.0",
// "1.5.0-beta1") is >= major.minor.patch under normal semver
// ordering. It replaces a previous lexicographic string comparison
// that classified "1.10.0" as older than "1.5.0". Missing or
// unparseable components are treated as 0; a version string this
// helper cannot parse is treated as pre-1.5.0 (i.e. the more
// conservative "open" branch below).
func isTerraformVersionAtLeast(v string, wantMajor, wantMinor, wantPatch int) bool {
	// Strip any pre-release / build metadata suffix.
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}
	parts := strings.SplitN(v, ".", 3)
	nums := [3]int{0, 0, 0}
	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return false
		}
		nums[i] = n
	}
	got := [3]int{nums[0], nums[1], nums[2]}
	want := [3]int{wantMajor, wantMinor, wantPatch}
	for i := 0; i < 3; i++ {
		if got[i] != want[i] {
			return got[i] > want[i]
		}
	}
	return true
}

func (p *F5osProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "f5os"
	resp.Version = p.version
}

func (p *F5osProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for Managing F5OS Devices: \n - Velos chassis \n - rSeries appliances",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				MarkdownDescription: "URI/Host details for F5os Device,can be provided via `F5OS_HOST` environment variable.",
				Optional:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Username for F5os Device,can be provided via `F5OS_USERNAME` environment variable.User provided here need to have required permission as per [UserManagement](https://techdocs.f5.com/en-us/f5os-a-1-4-0/f5-rseries-systems-administration-configuration/title-user-mgmt.html)",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password for F5os Device,can be provided via `F5OS_PASSWORD` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"port": schema.Int64Attribute{
				MarkdownDescription: "Port Number to be used to make API calls to HOST",
				Optional:            true,
			},
			"disable_tls_verify": schema.BoolAttribute{
				MarkdownDescription: "`disable_tls_verify` controls whether a client verifies the server's certificate chain and host name. default it is set to `true`. If `disable_tls_verify` is true, crypto/tls accepts any certificate presented by the server and any host name in that certificate. In this mode, TLS is susceptible to machine-in-the-middle attacks unless custom verification is used.\ncan be provided by `DISABLE_TLS_VERIFY` environment variable.\n\n~> **NOTE** If it is set to `false`, certificate/ca certificates should be added to `trusted store` of host where we are running this provider.",
				Optional:            true,
			},
			"teem_disable": schema.BoolAttribute{
				MarkdownDescription: "If this flag set to true,sending telemetry data to TEEM will be disabled,can be provided via `TEEM_DISABLE` environment variable.",
				Optional:            true,
			},
			"custom_headers": schema.MapAttribute{
				MarkdownDescription: "Optional map of custom HTTP headers added to every F5OS API request. When an HTTPS proxy is in use, these headers are also sent in the CONNECT tunnel request.",
				Optional:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (p *F5osProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring F5os client")

	// Retrieve provider data from configuration
	var config F5osProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	host := os.Getenv("F5OS_HOST")
	username := os.Getenv("F5OS_USERNAME")
	password := os.Getenv("F5OS_PASSWORD")
	teemTmp := os.Getenv("TEEM_DISABLE")

	hostPort := 8888
	if portEnv := os.Getenv("F5OS_PORT"); portEnv != "" {
		if port, err := strconv.Atoi(portEnv); err == nil {
			hostPort = port
		}
	}
	var teemDisable bool
	teemDisable = false
	if teemTmp == "true" {
		teemDisable = true
	}
	disableSSL := true
	if disableSSLtemp, ok := os.LookupEnv("DISABLE_TLS_VERIFY"); ok {
		if disableSSLtemp == "false" {
			disableSSL = false
		}
	}
	if !config.Host.IsNull() {
		host = config.Host.ValueString()
	}

	if !config.Username.IsNull() {
		username = config.Username.ValueString()
	}

	if !config.Password.IsNull() {
		password = config.Password.ValueString()
	}
	if !config.Port.IsNull() {
		hostPort = int(config.Port.ValueInt64())
	}
	if !config.TeemDisable.IsNull() {
		teemDisable = config.TeemDisable.ValueBool()
	}
	if !config.DisableSslVerify.IsNull() {
		disableSSL = config.DisableSslVerify.ValueBool()
	}
	// if !disableSSL && config.TrustedCertpath.IsNull() {
	// 	resp.Diagnostics.AddError("trusted_cert_path is required when disable_tls_verify is set to false", "trusted_cert_path is required when disable_tls_verify is set to false")
	// 	return
	// }
	// trustedCAPath := ""
	// if !config.TrustedCertpath.IsNull() {
	// 	trustedCAPath = config.TrustedCertpath.ValueString()
	// }
	if host == "" {
		resp.Diagnostics.AddError(
			"Missing 'host' in provider configuration",
			"While configuring the provider, 'host' was not found in "+
				"the F5OS_HOST environment variable or provider "+
				"configuration block host attribute.",
		)
	}
	if username == "" {
		resp.Diagnostics.AddError(
			"Missing 'username' in provider configuration",
			"While configuring the provider, username was not found in "+
				"the F5OS_USERNAME environment variable or provider "+
				"configuration block 'username' attribute.",
		)
	}
	if password == "" {
		resp.Diagnostics.AddError(
			"Missing 'password' in provider configuration",
			"While configuring the provider, 'password' was not found in "+
				"the F5OS_PASSWORD environment variable or provider "+
				"configuration block 'password' attribute.",
		)
	}
	// Bail out before touching NewSession if any required credential
	// is missing. Continuing here would produce a confusing
	// downstream connection error instead of the clear diagnostics
	// we just recorded.
	if resp.Diagnostics.HasError() {
		return
	}

	// Example client configuration for data sources and resources
	customHeaders := make(map[string]string)
	if !config.CustomHeaders.IsNull() && !config.CustomHeaders.IsUnknown() {
		for k, v := range config.CustomHeaders.Elements() {
			if sv, ok := v.(types.String); ok {
				customHeaders[k] = sv.ValueString()
			}
		}
	}
	f5osConfig := &f5ossdk.F5osConfig{
		Host:             host,
		User:             username,
		Password:         password,
		Port:             hostPort,
		DisableSSLVerify: disableSSL,
		CustomHeaders:    customHeaders,
		// TrustedCACertificate: trustedCAPath,
	}
	// Reuse an existing session if we've already authenticated to this
	// endpoint with these credentials in the current process. See the
	// comment on sessionCache for the motivation.
	//
	// Skip the cache for unit tests: testAccPreUnitCheck sets
	// F5OS_HOST to the httptest server's URL (e.g.
	// "http://127.0.0.1:62861"), which is torn down at the end of each
	// test. The OS can reuse the port for a later test's server,
	// producing the same cache key and reviving a client whose
	// connection points at a dead listener.
	//
	// Detection order:
	//  1. Explicit override via F5OS_SESSION_CACHE=false (preferred
	//     for tests that want to disable the cache without relying on
	//     any host-shape heuristic).
	//  2. Otherwise, treat any host that begins with an http:// or
	//     https:// scheme as non-cacheable — real F5OS hosts are bare
	//     IPs or hostnames.
	cacheable := true
	if os.Getenv("F5OS_SESSION_CACHE") == "false" {
		cacheable = false
	} else if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		cacheable = false
	}
	var client *f5ossdk.F5os
	if cacheable {
		cacheKey := sessionCacheKey(host, hostPort, username, password, disableSSL, customHeaders)
		sessionCacheMu.Lock()
		client = sessionCache[cacheKey]
		sessionCacheMu.Unlock()
		if client == nil {
			var err error
			client, err = f5ossdk.NewSession(f5osConfig)
			if err != nil {
				resp.Diagnostics.AddError(fmt.Sprintf("%+v", err.Error()), "")
				return
			}
			sessionCacheMu.Lock()
			// Double-check in case a concurrent Configure raced us.
			// Prefer the value already in the cache so all callers
			// share one session and the loser's client becomes garbage.
			if existing, ok := sessionCache[cacheKey]; ok {
				client = existing
			} else {
				sessionCache[cacheKey] = client
			}
			sessionCacheMu.Unlock()
		}
	} else {
		var err error
		client, err = f5ossdk.NewSession(f5osConfig)
		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("%+v", err.Error()), "")
			return
		}
	}
	client.Teem = teemDisable
	teemData.TerraformVersion = req.TerraformVersion
	teemData.ProviderName = "f5os"
	teemData.ProviderVersion = p.version
	teemData.F5Platform = fmt.Sprintf("F5OS %s", client.PlatformType)
	teemData.F5SoftwareVersion = client.PlatformVersion
	teemData.TerraformLicense = "open"
	if isTerraformVersionAtLeast(req.TerraformVersion, 1, 5, 0) {
		teemData.TerraformLicense = "business"
	}
	resp.DataSourceData = client
	resp.ResourceData = client
	tflog.Info(ctx, "Configured F5OS client", map[string]any{"success": true})
}

func (p *F5osProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewTenantImageResource,
		NewTenantResource,
		NewPartitionResource,
		NewPartitionChangePasswordResource,
		NewVlanResource,
		NewInterfaceResource,
		NewCfgBackupResource,
		NewLagResource,
		NewPartitionCertKeyResource,
		NewLicenseResource,
		NewSystemResource,
		NewDNSResource,
		NewPrimaryKeyResource,
		NewNTPServerResource,
		NewF5osLoggingResource,
		NewUserResource,
		NewUserPasswordChangeResource,
		NewQkviewResource,
		NewSnmpResource,
		NewAuthResource,
	}
}

func (p *F5osProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewImageInfoDataSource,
		NewDeviceInfoDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &F5osProvider{
			version: version,
		}
	}
}

// toProvider can be used to cast a generic provider.Provider reference to this specific provider.
// This is ideally used in DataSourceType.NewDataSource and ResourceType.NewResource calls.
func toF5osProvider(in any) (*f5ossdk.F5os, diag.Diagnostics) {
	if in == nil {
		return nil, nil
	}

	var diags diag.Diagnostics
	p, ok := in.(*f5ossdk.F5os)

	if !ok {
		diags.AddError(
			"Unexpected Provider Instance Type",
			fmt.Sprintf("While creating the data source or resource, an unexpected provider type (%T) was received. "+
				"This is always a bug in the provider code and should be reported to the provider developers.", in,
			),
		)
		return nil, diags
	}

	return p, diags
}
