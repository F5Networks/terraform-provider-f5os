package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
	"golang.org/x/mod/semver"
)

var _ resource.Resource = &PartitionCertKeyResource{}

// var _ resource.ResourceWithImportState = &PartitionCertKeyResource{}

func NewPartitionCertKeyResource() resource.Resource {
	return &PartitionCertKeyResource{}
}

type PartitionCertKeyResource struct {
	client   *f5ossdk.F5os
	teemData *TeemData
}

type PartitionCertKeyResourceModel struct {
	Name                   types.String `tfsdk:"name"`
	SubjectAlternativeName types.String `tfsdk:"subject_alternative_name"`
	DaysValid              types.Int64  `tfsdk:"days_valid"`
	Email                  types.String `tfsdk:"email"`
	City                   types.String `tfsdk:"city"`
	Province               types.String `tfsdk:"province"`
	Country                types.String `tfsdk:"country"`
	Organization           types.String `tfsdk:"organization"`
	Unit                   types.String `tfsdk:"unit"`
	Version                types.Int64  `tfsdk:"version"`
	KeyType                types.String `tfsdk:"key_type"`
	KeySize                types.Int64  `tfsdk:"key_size"`
	KeyCurve               types.String `tfsdk:"key_curve"`
	KeyPassphrase          types.String `tfsdk:"key_passphrase"`
	ConfirmKeyPassphrase   types.String `tfsdk:"confirm_key_passphrase"`
	Id                     types.String `tfsdk:"id"`
}

func (r *PartitionCertKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tls_cert_key"
}

func (r *PartitionCertKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Resource used to manage tls cert and key on F5OS partitions",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the tls certificate.",
			},
			"subject_alternative_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The subject alternative name of the tls certificate. This attribute is required for F5OS v1.8 and above and not supported for F5OS below v1.8",
			},
			"days_valid": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(30),
				MarkdownDescription: "The number of days for which the certificate is valid, the default value is 30 days",
			},
			"email": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The email address of the certificate holder.",
			},
			"city": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The residing cty of the certificate holder.",
			},
			"province": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The residing province of the certificate holder.",
			},
			"country": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The residing country of the certificate holder.",
			},
			"organization": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The organization of the certificate holder",
			},
			"unit": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The organizational unit of the certificate holder.",
			},
			"version": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
				MarkdownDescription: "The version of the certificate",
			},
			"key_type": schema.StringAttribute{
				Optional:            true,
				Validators:          []validator.String{stringvalidator.OneOf("rsa", "ecdsa", "encrypted-rsa", "encrypted-ecdsa")},
				MarkdownDescription: "The type of the tls key",
			},
			"key_size": schema.Int64Attribute{
				Optional:            true,
				Validators:          []validator.Int64{int64validator.OneOf(2048, 3072, 4096)},
				MarkdownDescription: "This specifies the length of the key, this is only applicable for RSA keys. This attribute is required when `key_type` is set to `rsa` or `encrypted-rsa`",
			},
			"key_curve": schema.StringAttribute{
				Optional:            true,
				Validators:          []validator.String{stringvalidator.OneOf("prime256v1", "secp384r1")},
				MarkdownDescription: "This specifies the specific elliptic curve used in ECC, this is only applicable for ECDSA keys. This attribute is required when `key_type` is set to `ecdsa` or `encrypted-ecdsa`",
			},
			"key_passphrase": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "This specifies the passphrase for the key. This attribute is required when `key_type` is set to `encrypted-rsa` or `encrypted-ecdsa`",
				Sensitive:           true,
			},
			"confirm_key_passphrase": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "This specifies the confirmation of the passphrase for the key, the value should be the same as the `key_passphrase`. This attribute is required when `key_type` is set to `encrypted-rsa` or `encrypted-ecdsa`",
				Sensitive:           true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique resource identifier",
			},
		},
	}
}

func (r *PartitionCertKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
	teemData.ProviderName = "f5os"
	teemData.ResourceName = "f5os_partition_cert_key"
	r.teemData = teemData
}

func (r *PartitionCertKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *PartitionCertKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	version := "v" + r.client.PlatformVersion
	if semver.Compare(semver.MajorMinor(version), "v1.8") >= 0 {
		if data.SubjectAlternativeName.IsNull() || data.SubjectAlternativeName.IsUnknown() {
			resp.Diagnostics.AddError("subject_alternative_name is required for platform version v1.8 and above", "")
			return
		}
	} else {
		if !data.SubjectAlternativeName.IsNull() || data.SubjectAlternativeName.IsUnknown() {
			resp.Diagnostics.AddError("subject_alternative_name is not supported for platform version below v1.8", "")
			return
		}
	}

	tlsConfig := getTLSConfig(data)

	err := r.client.CreateTlsCertKey(tlsConfig)

	if err != nil {
		resp.Diagnostics.AddError("Failed to create partition cert key", err.Error())
		return
	}

	data.Id = types.StringValue(tlsConfig.Name)
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *PartitionCertKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *PartitionCertKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PartitionCertKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *PartitionCertKeyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	version := r.client.PlatformVersion

	if semver.Compare(semver.MajorMinor(version), "v1.8") >= 0 {
		if data.SubjectAlternativeName.IsNull() || data.SubjectAlternativeName.IsUnknown() {
			resp.Diagnostics.AddError("subject_alternative_name is required for platform version v1.8 and above", "")
			return
		}
	} else {
		if !data.SubjectAlternativeName.IsNull() || data.SubjectAlternativeName.IsUnknown() {
			resp.Diagnostics.AddError("subject_alternative_name is not supported for platform version below v1.8", "")
			return
		}
	}

	tlsConfig := getTLSConfig(data)

	err := r.client.CreateTlsCertKey(tlsConfig)

	if err != nil {
		resp.Diagnostics.AddError("Failed to update partition cert key", err.Error())
		return
	}

	data.Id = types.StringValue(tlsConfig.Name)
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *PartitionCertKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *PartitionCertKeyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteTlsCertKey(data.Id.ValueString())

	if err != nil {
		resp.Diagnostics.AddError("Failed to delete partition cert key", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func getTLSConfig(data *PartitionCertKeyResourceModel) *f5ossdk.TlsCertKey {

	certKeyConfig := &f5ossdk.TlsCertKey{
		Name: data.Name.ValueString(),
		// SubjectAlternativeName: data.SubjectAlternativeName.ValueString(),
		DaysValid:            data.DaysValid.ValueInt64(),
		Email:                data.Email.ValueString(),
		City:                 data.City.ValueString(),
		Province:             data.Province.ValueString(),
		Country:              data.Country.ValueString(),
		Organization:         data.Organization.ValueString(),
		Unit:                 data.Unit.ValueString(),
		Version:              data.Version.ValueInt64(),
		KeyType:              data.KeyType.ValueString(),
		KeySize:              data.KeySize.ValueInt64(),
		KeyCurve:             data.KeyCurve.ValueString(),
		KeyPassphrase:        data.KeyPassphrase.ValueString(),
		ConfirmKeyPassphrase: data.ConfirmKeyPassphrase.ValueString(),
		StoreTls:             true,
	}

	if !data.SubjectAlternativeName.IsNull() || !data.SubjectAlternativeName.IsUnknown() {
		certKeyConfig.SubjectAlternativeName = data.SubjectAlternativeName.ValueString()
	}

	return certKeyConfig
}
