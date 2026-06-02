package attribute_plan_modifier

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------------------------------------------------------------------------
// Int64DefaultValue
// ---------------------------------------------------------------------------

func TestInt64DefaultValue(t *testing.T) {
	defaultVal := types.Int64Value(42)
	pm := Int64DefaultValue(defaultVal)
	if pm == nil {
		t.Fatal("expected non-nil plan modifier")
	}
}

func TestInt64Description(t *testing.T) {
	defaultVal := types.Int64Value(10)
	pm := Int64DefaultValue(defaultVal)
	ctx := context.Background()

	desc := pm.Description(ctx)
	if desc == "" {
		t.Fatal("expected non-empty description")
	}
	md := pm.MarkdownDescription(ctx)
	if desc != md {
		t.Errorf("Description and MarkdownDescription should match, got %q vs %q", desc, md)
	}
}

func TestInt64MarkdownDescription(t *testing.T) {
	defaultVal := types.Int64Value(99)
	pm := Int64DefaultValue(defaultVal)
	ctx := context.Background()

	md := pm.MarkdownDescription(ctx)
	if !strings.Contains(md, "default value") {
		t.Errorf("expected MarkdownDescription to contain 'default value', got %q", md)
	}
}

func TestPlanModifyInt64ConfigNotNull(t *testing.T) {
	defaultVal := types.Int64Value(42)
	pm := Int64DefaultValue(defaultVal)
	ctx := context.Background()

	// Config value is set (not null) — modifier should not change plan.
	// Use unknown plan value so that if the early-return guard were removed
	// the default would be applied and this test would catch the regression.
	req := planmodifier.Int64Request{
		ConfigValue: types.Int64Value(7),
		PlanValue:   types.Int64Unknown(),
	}
	resp := &planmodifier.Int64Response{PlanValue: req.PlanValue}
	pm.PlanModifyInt64(ctx, req, resp)

	if !resp.PlanValue.IsUnknown() {
		t.Errorf("expected plan value to remain unknown when config is set, got %v", resp.PlanValue)
	}
}

func TestPlanModifyInt64PlanAlreadySet(t *testing.T) {
	defaultVal := types.Int64Value(42)
	pm := Int64DefaultValue(defaultVal)
	ctx := context.Background()

	// Config is null but plan is already known and not null — prior
	// modifier applied; should not overwrite.
	req := planmodifier.Int64Request{
		ConfigValue: types.Int64Null(),
		PlanValue:   types.Int64Value(99),
	}
	resp := &planmodifier.Int64Response{PlanValue: req.PlanValue}
	pm.PlanModifyInt64(ctx, req, resp)

	if resp.PlanValue.ValueInt64() != 99 {
		t.Errorf("expected plan value 99, got %v", resp.PlanValue.ValueInt64())
	}
}

func TestPlanModifyInt64AppliesDefault(t *testing.T) {
	defaultVal := types.Int64Value(42)
	pm := Int64DefaultValue(defaultVal)
	ctx := context.Background()

	// Config is null and plan is null — should apply default.
	req := planmodifier.Int64Request{
		ConfigValue: types.Int64Null(),
		PlanValue:   types.Int64Null(),
	}
	resp := &planmodifier.Int64Response{PlanValue: req.PlanValue}
	pm.PlanModifyInt64(ctx, req, resp)

	if resp.PlanValue.ValueInt64() != 42 {
		t.Errorf("expected default 42, got %v", resp.PlanValue.ValueInt64())
	}
}

func TestPlanModifyInt64AppliesDefaultWhenUnknown(t *testing.T) {
	defaultVal := types.Int64Value(42)
	pm := Int64DefaultValue(defaultVal)
	ctx := context.Background()

	// Config is null and plan is unknown — should apply default.
	req := planmodifier.Int64Request{
		ConfigValue: types.Int64Null(),
		PlanValue:   types.Int64Unknown(),
	}
	resp := &planmodifier.Int64Response{PlanValue: req.PlanValue}
	pm.PlanModifyInt64(ctx, req, resp)

	if resp.PlanValue.ValueInt64() != 42 {
		t.Errorf("expected default 42, got %v", resp.PlanValue.ValueInt64())
	}
}

// ---------------------------------------------------------------------------
// StringDefaultValue
// ---------------------------------------------------------------------------

func TestStringDefaultValue(t *testing.T) {
	defaultVal := types.StringValue("hello")
	pm := StringDefaultValue(defaultVal)
	if pm == nil {
		t.Fatal("expected non-nil plan modifier")
	}
}

func TestStringDescription(t *testing.T) {
	defaultVal := types.StringValue("hello")
	pm := StringDefaultValue(defaultVal)
	ctx := context.Background()

	desc := pm.Description(ctx)
	if desc == "" {
		t.Fatal("expected non-empty description")
	}
	md := pm.MarkdownDescription(ctx)
	if desc != md {
		t.Errorf("Description and MarkdownDescription should match, got %q vs %q", desc, md)
	}
}

func TestStringMarkdownDescription(t *testing.T) {
	defaultVal := types.StringValue("world")
	pm := StringDefaultValue(defaultVal)
	ctx := context.Background()

	md := pm.MarkdownDescription(ctx)
	if !strings.Contains(md, "default value") {
		t.Errorf("expected MarkdownDescription to contain 'default value', got %q", md)
	}
}

func TestPlanModifyStringConfigNotNull(t *testing.T) {
	defaultVal := types.StringValue("default")
	pm := StringDefaultValue(defaultVal)
	ctx := context.Background()

	// Use unknown plan value so that if the early-return guard were removed
	// the default would be applied and this test would catch the regression.
	req := planmodifier.StringRequest{
		ConfigValue: types.StringValue("custom"),
		PlanValue:   types.StringUnknown(),
	}
	resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	pm.PlanModifyString(ctx, req, resp)

	if !resp.PlanValue.IsUnknown() {
		t.Errorf("expected plan value to remain unknown when config is set, got %v", resp.PlanValue)
	}
}

func TestPlanModifyStringPlanAlreadySet(t *testing.T) {
	defaultVal := types.StringValue("default")
	pm := StringDefaultValue(defaultVal)
	ctx := context.Background()

	req := planmodifier.StringRequest{
		ConfigValue: types.StringNull(),
		PlanValue:   types.StringValue("prior"),
	}
	resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	pm.PlanModifyString(ctx, req, resp)

	if resp.PlanValue.ValueString() != "prior" {
		t.Errorf("expected plan value 'prior', got %q", resp.PlanValue.ValueString())
	}
}

func TestPlanModifyStringAppliesDefault(t *testing.T) {
	defaultVal := types.StringValue("default")
	pm := StringDefaultValue(defaultVal)
	ctx := context.Background()

	req := planmodifier.StringRequest{
		ConfigValue: types.StringNull(),
		PlanValue:   types.StringNull(),
	}
	resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	pm.PlanModifyString(ctx, req, resp)

	if resp.PlanValue.ValueString() != "default" {
		t.Errorf("expected 'default', got %q", resp.PlanValue.ValueString())
	}
}

func TestPlanModifyStringAppliesDefaultWhenUnknown(t *testing.T) {
	defaultVal := types.StringValue("default")
	pm := StringDefaultValue(defaultVal)
	ctx := context.Background()

	req := planmodifier.StringRequest{
		ConfigValue: types.StringNull(),
		PlanValue:   types.StringUnknown(),
	}
	resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	pm.PlanModifyString(ctx, req, resp)

	if resp.PlanValue.ValueString() != "default" {
		t.Errorf("expected 'default', got %q", resp.PlanValue.ValueString())
	}
}

// ---------------------------------------------------------------------------
// BoolDefaultValue
// ---------------------------------------------------------------------------

func TestBoolDefaultValue(t *testing.T) {
	defaultVal := types.BoolValue(true)
	pm := BoolDefaultValue(defaultVal)
	if pm == nil {
		t.Fatal("expected non-nil plan modifier")
	}
}

func TestBoolDescription(t *testing.T) {
	defaultVal := types.BoolValue(false)
	pm := BoolDefaultValue(defaultVal)
	ctx := context.Background()

	desc := pm.Description(ctx)
	if desc == "" {
		t.Fatal("expected non-empty description")
	}
	md := pm.MarkdownDescription(ctx)
	if desc != md {
		t.Errorf("Description and MarkdownDescription should match, got %q vs %q", desc, md)
	}
}

func TestBoolMarkdownDescription(t *testing.T) {
	defaultVal := types.BoolValue(true)
	pm := BoolDefaultValue(defaultVal)
	ctx := context.Background()

	md := pm.MarkdownDescription(ctx)
	if !strings.Contains(md, "default value") {
		t.Errorf("expected MarkdownDescription to contain 'default value', got %q", md)
	}
}

func TestPlanModifyBoolConfigNotNull(t *testing.T) {
	defaultVal := types.BoolValue(true)
	pm := BoolDefaultValue(defaultVal)
	ctx := context.Background()

	// Use unknown plan value so that if the early-return guard were removed
	// the default would be applied and this test would catch the regression.
	req := planmodifier.BoolRequest{
		ConfigValue: types.BoolValue(false),
		PlanValue:   types.BoolUnknown(),
	}
	resp := &planmodifier.BoolResponse{PlanValue: req.PlanValue}
	pm.PlanModifyBool(ctx, req, resp)

	if !resp.PlanValue.IsUnknown() {
		t.Errorf("expected plan value to remain unknown when config is set, got %v", resp.PlanValue)
	}
}

func TestPlanModifyBoolPlanAlreadySet(t *testing.T) {
	defaultVal := types.BoolValue(true)
	pm := BoolDefaultValue(defaultVal)
	ctx := context.Background()

	req := planmodifier.BoolRequest{
		ConfigValue: types.BoolNull(),
		PlanValue:   types.BoolValue(false),
	}
	resp := &planmodifier.BoolResponse{PlanValue: req.PlanValue}
	pm.PlanModifyBool(ctx, req, resp)

	if resp.PlanValue.ValueBool() != false {
		t.Errorf("expected plan value false, got %v", resp.PlanValue.ValueBool())
	}
}

func TestPlanModifyBoolAppliesDefault(t *testing.T) {
	defaultVal := types.BoolValue(true)
	pm := BoolDefaultValue(defaultVal)
	ctx := context.Background()

	req := planmodifier.BoolRequest{
		ConfigValue: types.BoolNull(),
		PlanValue:   types.BoolNull(),
	}
	resp := &planmodifier.BoolResponse{PlanValue: req.PlanValue}
	pm.PlanModifyBool(ctx, req, resp)

	if resp.PlanValue.ValueBool() != true {
		t.Errorf("expected default true, got %v", resp.PlanValue.ValueBool())
	}
}

func TestPlanModifyBoolAppliesDefaultWhenUnknown(t *testing.T) {
	defaultVal := types.BoolValue(true)
	pm := BoolDefaultValue(defaultVal)
	ctx := context.Background()

	req := planmodifier.BoolRequest{
		ConfigValue: types.BoolNull(),
		PlanValue:   types.BoolUnknown(),
	}
	resp := &planmodifier.BoolResponse{PlanValue: req.PlanValue}
	pm.PlanModifyBool(ctx, req, resp)

	if resp.PlanValue.ValueBool() != true {
		t.Errorf("expected default true, got %v", resp.PlanValue.ValueBool())
	}
}
