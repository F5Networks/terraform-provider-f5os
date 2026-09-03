package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

const portGroupPath = "/restconf/data/f5-portgroup:portgroups/portgroup=1%2F1"

func setupPortGroupMock(t *testing.T, mode *string) *int {
	t.Helper()
	deleteCalls := 0
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/rseries_platform_state_ok.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-image:image/state/install", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/rseries_platform_version.json"))
	})
	mux.HandleFunc(portGroupPath, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/restconf/data/f5-portgroup:portgroups/portgroup=1/1", r.URL.Path)
		assert.Equal(t, "/restconf/data/f5-portgroup:portgroups/portgroup=1%2F1", r.URL.EscapedPath())
		_, _ = fmt.Fprintf(w, `{"f5-portgroup:portgroups":{"portgroup":[{"portgroup_name":"1/1","config":{"name":"1/1","mode":%q,"f5-ddm:ddm":{"f5-ddm:ddm-poll-frequency":30}}}]}}`, *mode)
	})
	mux.HandleFunc(portGroupPath+"/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalls++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		assert.Equal(t, http.MethodPatch, r.Method)
		var request struct {
			Config struct {
				Name string `json:"name"`
				Mode string `json:"mode"`
				DDM  struct {
					PollFrequency int64 `json:"f5-ddm:ddm-poll-frequency"`
				} `json:"f5-ddm:ddm"`
			} `json:"f5-portgroup:config"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.Equal(t, "1/1", request.Config.Name)
		assert.Equal(t, int64(30), request.Config.DDM.PollFrequency)
		*mode = request.Config.Mode
		w.WriteHeader(http.StatusNoContent)
	})
	return &deleteCalls
}

func portGroupSchema(t *testing.T, resource *PortGroupResource) resourceschema.Schema {
	t.Helper()
	response := &fwresource.SchemaResponse{}
	resource.Schema(context.Background(), fwresource.SchemaRequest{}, response)
	if response.Diagnostics.HasError() {
		t.Fatalf("Schema returned diagnostics: %v", response.Diagnostics)
	}
	return response.Schema
}

func portGroupState(t *testing.T, schema resourceschema.Schema, model PortGroupResourceModel) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	state := tfsdk.State{
		Schema: schema,
		Raw:    tftypes.NewValue(schema.Type().TerraformType(ctx), nil),
	}
	if diagnostics := state.Set(ctx, &model); diagnostics.HasError() {
		t.Fatalf("failed to create Terraform state: %v", diagnostics)
	}
	return state
}

func portGroupPlan(t *testing.T, schema resourceschema.Schema, model PortGroupResourceModel) tfsdk.Plan {
	t.Helper()
	ctx := context.Background()
	plan := tfsdk.Plan{
		Schema: schema,
		Raw:    tftypes.NewValue(schema.Type().TerraformType(ctx), nil),
	}
	if diagnostics := plan.Set(ctx, &model); diagnostics.HasError() {
		t.Fatalf("failed to create Terraform plan: %v", diagnostics)
	}
	return plan
}

func TestUnitPortGroupResourceInProcessLifecycle(t *testing.T) {
	testAccPreUnitCheck(t)
	mode := "MODE_4x25G"
	deleteCalls := setupPortGroupMock(t, &mode)
	defer teardown()

	client, err := newTestClientFromEnv()
	if err != nil {
		t.Fatalf("failed to create mock client: %s", err)
	}
	ctx := context.Background()
	resource := &PortGroupResource{client: client}
	schema := portGroupSchema(t, resource)

	plan := portGroupPlan(t, schema, PortGroupResourceModel{
		Name:             types.StringValue("1/1"),
		Mode:             types.StringValue("MODE_4x25GB"),
		DDMPollFrequency: types.Int64Value(30),
	})
	createResponse := &fwresource.CreateResponse{State: portGroupState(t, schema, PortGroupResourceModel{})}
	resource.Create(ctx, fwresource.CreateRequest{Plan: plan}, createResponse)
	if createResponse.Diagnostics.HasError() {
		t.Fatalf("Create returned diagnostics: %v", createResponse.Diagnostics)
	}

	var created PortGroupResourceModel
	if diagnostics := createResponse.State.Get(ctx, &created); diagnostics.HasError() {
		t.Fatalf("failed to read created state: %v", diagnostics)
	}
	assert.Equal(t, "1/1", created.ID.ValueString())
	assert.Equal(t, "MODE_4x25GB", created.Mode.ValueString())
	assert.Equal(t, int64(30), created.DDMPollFrequency.ValueInt64())

	updatedPlan := portGroupPlan(t, schema, PortGroupResourceModel{
		Name:             types.StringValue("1/1"),
		Mode:             types.StringValue("MODE_100GB"),
		DDMPollFrequency: types.Int64Value(30),
	})
	updateResponse := &fwresource.UpdateResponse{State: createResponse.State}
	resource.Update(ctx, fwresource.UpdateRequest{Plan: updatedPlan}, updateResponse)
	if updateResponse.Diagnostics.HasError() {
		t.Fatalf("Update returned diagnostics: %v", updateResponse.Diagnostics)
	}

	readResponse := &fwresource.ReadResponse{State: updateResponse.State}
	resource.Read(ctx, fwresource.ReadRequest{State: updateResponse.State}, readResponse)
	if readResponse.Diagnostics.HasError() {
		t.Fatalf("Read returned diagnostics: %v", readResponse.Diagnostics)
	}
	var read PortGroupResourceModel
	if diagnostics := readResponse.State.Get(ctx, &read); diagnostics.HasError() {
		t.Fatalf("failed to read refreshed state: %v", diagnostics)
	}
	assert.Equal(t, "MODE_100GB", read.Mode.ValueString())

	deleteResponse := &fwresource.DeleteResponse{}
	resource.Delete(ctx, fwresource.DeleteRequest{State: readResponse.State}, deleteResponse)
	assert.False(t, deleteResponse.Diagnostics.HasError(), "Delete returned diagnostics: %v", deleteResponse.Diagnostics)
	assert.Equal(t, 1, *deleteCalls)

	importResponse := &fwresource.ImportStateResponse{State: portGroupState(t, schema, PortGroupResourceModel{})}
	resource.ImportState(ctx, fwresource.ImportStateRequest{ID: "1/1"}, importResponse)
	if importResponse.Diagnostics.HasError() {
		t.Fatalf("ImportState returned diagnostics: %v", importResponse.Diagnostics)
	}
	var imported PortGroupResourceModel
	if diagnostics := importResponse.State.Get(ctx, &imported); diagnostics.HasError() {
		t.Fatalf("failed to read imported state: %v", diagnostics)
	}
	assert.Equal(t, "1/1", imported.ID.ValueString())
	assert.Equal(t, "1/1", imported.Name.ValueString())
}

func TestUnitPortGroupResourceInProcessDiagnostics(t *testing.T) {
	ctx := context.Background()
	resource := &PortGroupResource{client: &f5ossdk.F5os{PlatformType: "Velos Controller"}}
	schema := portGroupSchema(t, resource)
	plan := portGroupPlan(t, schema, PortGroupResourceModel{
		Name: types.StringValue("1/1"),
		Mode: types.StringValue("MODE_4x25GB"),
	})
	createResponse := &fwresource.CreateResponse{State: portGroupState(t, schema, PortGroupResourceModel{})}
	resource.Create(ctx, fwresource.CreateRequest{Plan: plan}, createResponse)
	assert.True(t, createResponse.Diagnostics.HasError())

	testAccPreUnitCheck(t)
	mode := "MODE_4x25G"
	setupPortGroupMock(t, &mode)
	defer teardown()
	client, err := newTestClientFromEnv()
	if err != nil {
		t.Fatalf("failed to create mock client: %s", err)
	}
	client.Host = "http://127.0.0.1:1"
	resource = &PortGroupResource{client: client}

	createResponse = &fwresource.CreateResponse{State: portGroupState(t, schema, PortGroupResourceModel{})}
	resource.Create(ctx, fwresource.CreateRequest{Plan: plan}, createResponse)
	assert.True(t, createResponse.Diagnostics.HasError())

	state := portGroupState(t, schema, PortGroupResourceModel{
		ID:   types.StringValue("1/1"),
		Name: types.StringValue("1/1"),
		Mode: types.StringValue("MODE_4x25GB"),
	})
	readResponse := &fwresource.ReadResponse{State: state}
	resource.Read(ctx, fwresource.ReadRequest{State: state}, readResponse)
	assert.True(t, readResponse.Diagnostics.HasError())

	updateResponse := &fwresource.UpdateResponse{State: state}
	resource.Update(ctx, fwresource.UpdateRequest{Plan: plan}, updateResponse)
	assert.True(t, updateResponse.Diagnostics.HasError())

	deleteResponse := &fwresource.DeleteResponse{}
	resource.Delete(ctx, fwresource.DeleteRequest{State: state}, deleteResponse)
	assert.True(t, deleteResponse.Diagnostics.HasError())
}

func TestUnitPortGroupResourceLifecycle(t *testing.T) {
	testAccPreUnitCheck(t)
	mode := "MODE_4x25G"
	setupPortGroupMock(t, &mode)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `resource "f5os_portgroup" "test" {
  name = "1/1"
	mode = "MODE_4x25GB"
	ddm_poll_frequency = 30
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_portgroup.test", "id", "1/1"),
					resource.TestCheckResourceAttr("f5os_portgroup.test", "mode", "MODE_4x25GB"),
					resource.TestCheckResourceAttr("f5os_portgroup.test", "ddm_poll_frequency", "30"),
				),
			},
			{
				Config: `resource "f5os_portgroup" "test" {
  name = "1/1"
  mode = "MODE_100GB"
  ddm_poll_frequency = 30
}`,
				Check: resource.TestCheckResourceAttr("f5os_portgroup.test", "mode", "MODE_100GB"),
			},
			{
				ResourceName:      "f5os_portgroup.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestUnitPortGroupResourceRejectsNonRSeries(t *testing.T) {
	testAccPreUnitCheck(t)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/platform_components_velos_controller.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-controller-image:image", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-system-controller-image:image":{"state":{"controllers":{"controller":[{"number":1,"os-version":"2.0.0"}]}}}}`)
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `resource "f5os_portgroup" "test" {
  name = "1/1"
	mode = "MODE_4x25GB"
}`,
			ExpectError: regexp.MustCompile("supported only on rSeries"),
		}},
	})
}
