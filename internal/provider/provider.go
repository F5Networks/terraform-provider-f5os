package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
	"os"
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
	Host     types.String `tfsdk:"host"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	Port     types.Int64  `tfsdk:"port"`
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
				MarkdownDescription: "Username for F5os Device,can be provided via `F5OS_USERNAME` environment variable.",
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

	// Configuration values are now available.
	// if data.Endpoint.IsNull() { /* ... */ }

	// Default values to environment variables, but override
	// with Terraform configuration value if set.

	host := os.Getenv("F5OS_HOST")
	username := os.Getenv("F5OS_USERNAME")
	password := os.Getenv("F5OS_PASSWORD")
	hostPort := 8888

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

	//ctx = tflog.SetField(ctx, "f5os_host", host)
	//ctx = tflog.SetField(ctx, "f5os_username", username)
	////ctx = tflog.SetField(ctx, "f5os_password", password)
	//ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "f5os_password")

	// Example client configuration for data sources and resources
	f5osConfig := &f5ossdk.F5osConfig{
		Host:     host,
		User:     username,
		Password: password,
		Port:     hostPort,
	}

	//tflog.Info(ctx, fmt.Sprintf("f5osConfig client:%+v", f5osConfig))

	client, err := f5ossdk.NewSession(f5osConfig)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create f5os Client",
			"An unexpected error occurred when creating the f5os client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"f5os Client Error: "+err.Error(),
		)
		return
	}
	resp.DataSourceData = client
	resp.ResourceData = client
	tflog.Info(ctx, fmt.Sprintf("f5osConfig client:%+v", client))
	tflog.Info(ctx, "Configured F5OS client", map[string]any{"success": true})
}

func (p *F5osProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewTenantImageResource,
		NewTenantResource,
		NewPartitionResource,
		NewPartitionChangePasswordResource,
		NewVlanResource,
	}
}

func (p *F5osProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
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

//// hashForState computes the hexadecimal representation of the SHA1 checksum of a string.
//// This is used by most resources/data-sources here to compute their Unique Identifier (ID).
//func hashForState(value string) string {
//	if value == "" {
//		return ""
//	}
//	hash := sha1.Sum([]byte(strings.TrimSpace(value)))
//	return hex.EncodeToString(hash[:])
//}
