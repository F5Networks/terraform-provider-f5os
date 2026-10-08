package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// vlanNameRegexp mirrors f5os_vlan's own `name` schema documentation
// (internal/provider/vlan_resource.go): must start with a letter,
// remaining characters alphanumeric/period/comma/hyphen/underscore
// only, max 58 characters total.
var vlanNameRegexp = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9.,_-]{0,57}$`)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &VlansResource{}
var _ resource.ResourceWithImportState = &VlansResource{}
var _ resource.ResourceWithValidateConfig = &VlansResource{}

func NewVlansResource() resource.Resource {
	return &VlansResource{}
}

// VlansResource manages a whole set of VLANs on an F5OS partition/rSeries
// platform through a single resource instance, complementing the
// singular f5os_vlan resource (which remains unchanged for backward
// compatibility -- both resources can coexist, managing disjoint VLAN
// ID sets, since both ultimately PATCH the same
// /openconfig-vlan:vlans container).
//
// The primary motivation is apply-time performance for large VLAN
// counts (e.g. an iSeries-to-F5OS migration with hundreds of VLANs):
// f5os_vlan issues one PATCH per VLAN (plus one GET read-back) on every
// apply, while f5os_vlans batches every managed VLAN's create/update
// into a single PATCH to /openconfig-vlan:vlans in Create/Update, and a
// single GET to /openconfig-vlan:vlans/vlan (F5RespVlan's bulk list) in
// Read -- confirmed against a live F5OS device that a multi-entry PATCH
// creates every entry atomically in one round trip (an invalid entry
// rejects the whole batch with no partial creation, so there is no
// partial-success state to reconcile).
//
// Delete has no batch primitive in the vendored client (RESTCONF DELETE
// against a YANG list only supports one key at a time), so removing
// VLANs still issues one DeleteVlan call per removed VLAN ID -- this
// matches f5os_vlan's own Delete semantics (hard error on first
// failure, matching the existing single-VLAN resource's convention
// rather than the log-and-continue convention f5os_snmp_resource.go
// uses for its own multi-item Delete).
//
// Trade-off versus `for_each` + f5os_vlan (see
// https://registry.terraform.io/providers/F5Networks/f5os/latest/docs/guides/create-vlans-from-iseries): collapsing many VLANs into
// one f5os_vlans resource instance means Terraform's native per-item
// resource addressing (`terraform state mv`, `-target`, individual
// `terraform import` of a single VLAN) no longer applies to individual
// VLANs -- add/remove of a single VLAN is now an in-resource map diff,
// not a distinct resource instance appearing/disappearing from `plan`.
// Operators who need per-VLAN resource identity (e.g. targeted
// destroy/import workflows) should keep using `for_each` + f5os_vlan;
// f5os_vlans is an opt-in alternative for the common case of applying a
// bulk set of migrated VLANs where apply-time performance matters more
// than granular per-VLAN Terraform addressing.
type VlansResource struct {
	client   *f5ossdk.F5os
	teemData *TeemData
}

type VlansResourceModel struct {
	Vlans types.Map    `tfsdk:"vlans"`
	Id    types.String `tfsdk:"id"`
}

func (r *VlansResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vlans"
}

func (r *VlansResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Resource to manage a whole set of VLANs on F5OS based systems (chassis partitions or rSeries platforms) " +
			"in a single resource instance, complementing `f5os_vlan`. Every VLAN in `vlans` is created or updated in a single " +
			"RESTCONF PATCH call (rather than one call per VLAN), making this resource significantly faster than `for_each` + " +
			"`f5os_vlan` for migrations with large VLAN counts. `f5os_vlan` remains unchanged and fully supported -- the two " +
			"resources can be used together (including on the same device) as long as they manage disjoint VLAN ID sets, since " +
			"both ultimately PATCH the same underlying `/openconfig-vlan:vlans` container. Removing a VLAN from `vlans` still " +
			"issues one DELETE call per removed VLAN (RESTCONF has no batch-delete primitive for this list), so delete-heavy " +
			"workflows do not see the same speedup as create/update. Collapsing many VLANs into one resource instance also means " +
			"individual VLANs no longer have their own Terraform resource address -- `terraform state mv`/`-target`/`import` " +
			"apply to this resource as a whole, not to a single VLAN within it. Use `for_each` + `f5os_vlan` instead if per-VLAN " +
			"resource addressing matters more than apply-time performance for your use case.",

		Attributes: map[string]schema.Attribute{
			"vlans": schema.MapAttribute{
				MarkdownDescription: "Map of VLAN name to VLAN ID/tag to create on the F5OS platform. Every entry is created or " +
					"updated in a single RESTCONF PATCH call. Valid VLAN ID range is `0` to `4095`. VLAN names must start with a " +
					"letter, contain only alphanumeric characters, periods, commas, hyphens, or underscores, and not exceed 58 " +
					"characters (matching `f5os_vlan`'s own `name` constraint). Must contain at least one entry.",
				Required:    true,
				ElementType: types.Int64Type,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for this resource. Not a device identifier -- this resource instance manages a set of VLANs, not a single VLAN with its own ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *VlansResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)

	// Build a resource-owned copy of the shared, package-level
	// telemetry metadata rather than mutating the `teemData` pointer
	// in place and aliasing it directly into r.teemData: several other
	// resources' own Configure methods (f5os_vlan, f5os_interface,
	// f5os_partition, etc.) each write their own ProviderName/
	// ResourceName onto that exact same shared *TeemData and alias it
	// the same way, and the plugin framework can invoke multiple
	// resources' Configure concurrently -- writing here would race
	// with (or be raced by) those other resources' writes, and every
	// resource holding the same pointer means whichever resource sends
	// telemetry last wins, silently reporting the wrong ResourceName
	// for every earlier SendTeem call. Copying the struct once and
	// customizing only the fields this resource needs keeps r.teemData
	// independent of any other resource's Configure call.
	teemDataCopy := *teemData
	teemDataCopy.ProviderName = "f5os"
	teemDataCopy.ResourceName = "f5os_vlans"
	r.teemData = &teemDataCopy
}

// ValidateConfig enforces per-entry constraints on `vlans` that a plain
// schema.MapAttribute cannot express on its own (this repo does not
// vendor terraform-plugin-framework-validators/mapvalidator, so these
// checks are expressed here instead, following the same
// resource.ResourceWithValidateConfig pattern tenant_image_resource.go
// and config_restore_resource.go use for their own cross-field/
// per-value constraints): `vlans` must be non-empty, every VLAN ID must
// be in the valid 0-4095 range, every VLAN name must match f5os_vlan's
// own name format, and no two names may map to the same VLAN ID (two
// entries colliding on ID would otherwise silently create/manage only
// one VLAN, since buildBatchVlanConfig/readVlansIntoState key by ID
// internally in places).
func (r *VlansResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data VlansResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// data.Vlans.IsUnknown() only reports true when the whole map's
	// identity is unknown (e.g. the attribute itself comes from an
	// unknown value); when that's the case there is nothing at all to
	// validate yet.
	if data.Vlans.IsUnknown() {
		return
	}

	elements := data.Vlans.Elements()

	if !data.Vlans.IsNull() && len(elements) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("vlans"),
			"Invalid Attribute Value",
			"vlans must contain at least one entry; an empty map (`vlans = {}`) was provided.",
		)
	}

	// A known map can still contain one or more unknown *element*
	// values (e.g. `vlans = { "external" = 100, "seeded" =
	// tonumber(some_other_resource.id) }`, a legitimate pattern for a
	// dynamically-allocated VLAN ID) -- map keys, unlike values, are
	// always known once the map itself is known, so the name-format
	// check below applies to every entry regardless of whether its
	// value is known yet, while the ID-range and duplicate-ID checks
	// are skipped only for the individual entries whose value isn't
	// resolved yet (extractVlansMap likewise skips unknown element
	// values rather than erroring on them). Previously, ANY unknown
	// element value caused this whole function to return early,
	// silently skipping validation for every other -- known and
	// possibly invalid -- entry in the map too.
	vlans, diags := extractVlansMap(ctx, data.Vlans)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	seenIDs := make(map[int64]string, len(vlans))
	for name := range elements {
		if !vlanNameRegexp.MatchString(name) {
			resp.Diagnostics.AddAttributeError(
				path.Root("vlans"),
				"Invalid Attribute Value",
				fmt.Sprintf("vlans key %q is not a valid VLAN name: must start with a letter, contain only alphanumeric characters, periods, commas, hyphens, or underscores, and not exceed 58 characters.", name),
			)
		}

		id, known := vlans[name]
		if !known {
			// This entry's value is still unknown -- nothing further
			// to check against it until it resolves at apply time.
			continue
		}
		if id < 0 || id > 4095 {
			resp.Diagnostics.AddAttributeError(
				path.Root("vlans"),
				"Invalid Attribute Value",
				fmt.Sprintf("vlans[%q] = %d is out of range: VLAN IDs must be between 0 and 4095.", name, id),
			)
		}
		if other, dup := seenIDs[id]; dup {
			resp.Diagnostics.AddAttributeError(
				path.Root("vlans"),
				"Invalid Attribute Value",
				fmt.Sprintf("vlans[%q] and vlans[%q] both map to VLAN ID %d -- every VLAN ID in vlans must be unique.", other, name, id),
			)
			continue
		}
		seenIDs[id] = name
	}
}

func (r *VlansResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VlansResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client.PlatformType == "Velos Controller" {
		resp.Diagnostics.AddError("Client Error", "`f5os_vlans` resource is supported with Velos Partition level/rSeries appliance.")
		return
	}

	vlans, diags := extractVlansMap(ctx, data.Vlans)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, fmt.Sprintf("[CREATE] f5os_vlans: %d VLAN(s) in a single batch", len(vlans)))
	vlanReqConfig := buildBatchVlanConfig(vlans)

	teemInfo := make(map[string]any)
	teemInfo["teemData"] = r.teemData
	_ = r.client.SendTeem(teemInfo)

	if _, err := r.client.VlanConfig(vlanReqConfig); err != nil {
		resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Create VLANs failed, got error: %s", err))
		return
	}

	// id is a purely-Terraform-side bookkeeping value (this resource
	// manages a set of VLANs, not one device object with its own natural
	// ID -- see the resource's own doc comment) generated once here and
	// never recomputed afterward (Read/Update leave it untouched, same
	// as `UseStateForUnknown`'s contract expects): deriving it from the
	// VLAN set's own contents was tried and rejected during development
	// -- it produced "provider produced inconsistent result after
	// apply" the moment a VLAN was added/removed, since UseStateForUnknown
	// requires id to stay stable across Update unless explicitly planned
	// as unknown.
	data.Id = types.StringValue(uuid.NewString())

	if diags := r.readVlansIntoState(ctx, &data, vlans); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VlansResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VlansResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	priorVlans, diags := extractVlansMap(ctx, data.Vlans)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if diags := r.readVlansIntoState(ctx, &data, priorVlans); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VlansResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan VlansResourceModel
	var state VlansResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client.PlatformType == "Velos Controller" {
		resp.Diagnostics.AddError("Client Error", "`f5os_vlans` resource is supported with Velos Partition level/rSeries appliance.")
		return
	}

	planVlans, diags := extractVlansMap(ctx, plan.Vlans)
	resp.Diagnostics.Append(diags...)
	stateVlans, diags := extractVlansMap(ctx, state.Vlans)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A prior-state VLAN ID must be deleted whenever it is no longer
	// present anywhere in the plan's VLAN ID set -- not merely when its
	// *name* key disappears from the plan. These are different checks:
	// if an existing key's value changes (e.g. `vlans = { "external" =
	// 100 }` -> `vlans = { "external" = 200 }`), the name "external" is
	// still present in planVlans, but VLAN ID 100 is not -- a
	// name-presence-only check would treat this as "nothing to delete"
	// and silently orphan VLAN 100 on the device forever (it would
	// never again be included in this resource's managed-ID set, so
	// readVlansIntoState's filter would permanently hide it from every
	// future Read too). Comparing by ID set instead of by name-key
	// presence correctly catches this case.
	//
	// RESTCONF has no batch-delete primitive for a YANG list, so this
	// is N sequential DeleteVlan calls (same as f5os_vlan's own Delete,
	// and the one part of this resource's lifecycle that does not get
	// the batch-performance benefit). A failure here hard-errors
	// immediately (matches f5os_vlan's convention), leaving any
	// not-yet-processed removed VLAN ID, and any newly-added/changed
	// VLAN in the plan (see below), untouched on the device --
	// Terraform's next plan/apply will retry both from the resulting
	// (partially-updated) prior state.
	planIDs := make(map[int64]bool, len(planVlans))
	for _, id := range planVlans {
		planIDs[id] = true
	}
	for name, vlanID := range stateVlans {
		if planIDs[vlanID] {
			continue
		}
		tflog.Info(ctx, fmt.Sprintf("[UPDATE] f5os_vlans: removing VLAN %q (id=%d)", name, vlanID))
		if err := r.client.DeleteVlan(int(vlanID)); err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Delete VLAN %q (id=%d) failed, got error: %s", name, vlanID, err))
			return
		}
	}

	// Every VLAN remaining in the plan (new or changed -- unchanged
	// entries are harmlessly re-PATCHed too, since RESTCONF PATCH on an
	// already-matching leaf is a no-op on the device) is batched into a
	// single PATCH, exactly like Create.
	if len(planVlans) > 0 {
		tflog.Info(ctx, fmt.Sprintf("[UPDATE] f5os_vlans: %d VLAN(s) in a single batch", len(planVlans)))
		vlanReqConfig := buildBatchVlanConfig(planVlans)
		if _, err := r.client.VlanConfig(vlanReqConfig); err != nil {
			resp.Diagnostics.AddError("F5OS Client Error:", fmt.Sprintf("Update VLANs failed, got error: %s", err))
			return
		}
	}

	// id is intentionally NOT recomputed here -- it must stay stable
	// across Update (see Create's comment on why it is not derived from
	// the VLAN set's contents). plan.Id is already unknown at this point
	// (Terraform's own plan for a Computed+UseStateForUnknown attribute
	// carries the prior state's value forward automatically when the
	// resource doesn't otherwise force replacement), so simply carrying
	// it through to State.Set below is correct.
	if diags := r.readVlansIntoState(ctx, &plan, planVlans); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VlansResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VlansResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vlans, diags := extractVlansMap(ctx, data.Vlans)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	for name, vlanID := range vlans {
		tflog.Info(ctx, fmt.Sprintf("[DELETE] f5os_vlans: removing VLAN %q (id=%d)", name, vlanID))
		if err := r.client.DeleteVlan(int(vlanID)); err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete VLAN %q (id=%d), got error: %s", name, vlanID, err))
			return
		}
	}
}

// ImportState imports every VLAN currently on the device into a single
// f5os_vlans resource instance -- there is no per-VLAN identifier to
// import here (unlike f5os_vlan's ImportState, which imports exactly
// one VLAN by ID), so the import ID string itself is not used as `id`
// (a fresh one is generated, same as Create -- see Create's comment on
// why `id` is opaque Terraform bookkeeping, not derived from device
// data) and the subsequent Read adopts every VLAN reported by the
// device (via readVlansIntoState's empty-managed-set fallthrough, since
// prior state is empty immediately after import).
func (r *VlansResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), uuid.NewString())...)
}

// readVlansIntoState bulk-GETs every VLAN on the device in a single
// call (f5ossdk.GetVlansInfo, GET /openconfig-vlan:vlans/vlan) and
// filters the result down to the set of (name, VLAN ID) pairs this
// resource instance manages (managed), mirroring
// f5os_snmp_resource.go's managedCommunities/filterCommunities pattern:
// when managed is non-empty, only device VLANs whose (name, ID) pair
// exactly matches a managed entry are reported into state (VLANs
// belonging to a sibling f5os_vlan resource, or to another f5os_vlans
// instance, or unmanaged entirely, are left alone). When managed is
// empty (post-import), every VLAN on the device is adopted.
//
// Filtering requires matching the full (name, ID) pair, not ID alone:
// an ID-only filter would incorrectly adopt a device VLAN under
// whatever name the device currently reports for a managed ID, even if
// that name doesn't match what this resource instance actually
// configured for that ID -- which previously caused a hard "provider
// produced inconsistent result after apply" crash during Create/Read
// (Terraform's own plugin-protocol consistency check catches the
// returned map not matching what was planned) whenever a managed ID's
// device-reported name diverged from the configured name (e.g. from an
// accidental ID overlap with a sibling f5os_vlan/f5os_vlans instance --
// see this resource's own doc comment on the disjoint-ID-set
// requirement). Matching the exact pair instead means a diverged VLAN
// is simply excluded from this Read (surfacing as an ordinary "this
// VLAN is missing, needs re-creating" plan diff on the next apply, via
// the normal batch-PATCH path) rather than crashing or silently
// adopting a name this resource instance never configured.
func (r *VlansResource) readVlansIntoState(ctx context.Context, data *VlansResourceModel, managed map[string]int64) diag.Diagnostics {
	var diags diag.Diagnostics

	deviceVlans, err := r.client.GetVlansInfo()
	if err != nil {
		diags.AddError("F5OS Client Error", fmt.Sprintf("Unable to read VLANs, got error: %s", err))
		return diags
	}

	filter := len(managed) > 0

	result := make(map[string]int64, len(deviceVlans.OpenconfigVlanVlan))
	for _, v := range deviceVlans.OpenconfigVlanVlan {
		id := int64(v.Config.VlanID)
		name := v.Config.Name
		if filter {
			managedID, ok := managed[name]
			if !ok || managedID != id {
				continue
			}
		}
		result[name] = id
	}

	vlansMap, mapDiags := types.MapValueFrom(ctx, types.Int64Type, result)
	diags.Append(mapDiags...)
	data.Vlans = vlansMap

	return diags
}

// extractVlansMap converts a types.Map into a plain map[string]int64,
// treating null/unknown as empty (mirroring f5os_snmp_resource.go's
// extractSnmp* null/unknown guards). Entries are converted
// element-by-element via Elements() rather than delegating to
// ElementsAs, which cannot handle a known map containing one or more
// unknown *element* values (a legitimate pattern when a VLAN ID is
// sourced from another resource's not-yet-known computed attribute,
// e.g. `tonumber(f5os_vlan.seed.id)`) -- ElementsAs fails on that shape
// with an unhelpful internal "please report this to the provider
// developer" error instead of gracefully treating just that one value
// as not-yet-available. Unknown element values are silently skipped
// here (the returned map simply omits that entry); an actual
// type-assertion failure against a *known* element value (which would
// indicate a genuine provider bug, since the schema's ElementType is
// fixed to types.Int64Type) still surfaces as a diagnostic.
func extractVlansMap(ctx context.Context, m types.Map) (map[string]int64, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := make(map[string]int64)
	if m.IsNull() || m.IsUnknown() {
		return result, diags
	}
	for name, v := range m.Elements() {
		if v.IsUnknown() || v.IsNull() {
			continue
		}
		id, ok := v.(types.Int64)
		if !ok {
			diags.AddError(
				"Unexpected VLAN Map Element Type",
				fmt.Sprintf("vlans[%q] has an unexpected value type %T; this is always a bug in the provider code and should be reported to the provider developers.", name, v),
			)
			continue
		}
		result[name] = id.ValueInt64()
	}
	return result, diags
}

// buildBatchVlanConfig builds a single F5ReqVlansConfig containing one
// F5ReqVlanConfig entry per VLAN, for a single batched
// r.client.VlanConfig(...) PATCH call -- the same client method and
// struct f5os_vlan's own getPartitionVlanConfig uses for a single VLAN,
// just with N entries appended instead of one. No vendored client
// changes are required: F5ReqVlansConfig.OpenconfigVlanVlans.Vlan is
// already a slice, and PATCHing /openconfig-vlan:vlans with a
// multi-entry "vlan" array is confirmed (against a live F5OS device) to
// create/update every entry atomically in one round trip.
func buildBatchVlanConfig(vlans map[string]int64) *f5ossdk.F5ReqVlansConfig {
	vlanReqConfig := &f5ossdk.F5ReqVlansConfig{}
	for name, id := range vlans {
		entry := f5ossdk.F5ReqVlanConfig{}
		entry.Config.Name = name
		entry.Config.VlanId = int(id)
		entry.VlanId = fmt.Sprintf("%d", id)
		vlanReqConfig.OpenconfigVlanVlans.Vlan = append(vlanReqConfig.OpenconfigVlanVlans.Vlan, entry)
	}
	return vlanReqConfig
}
