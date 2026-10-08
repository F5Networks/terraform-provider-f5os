package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
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
	ddmDeleteCalls := 0
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
	mux.HandleFunc(portGroupPath+"/config/f5-ddm:ddm-poll-frequency", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			ddmDeleteCalls++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	return &ddmDeleteCalls
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
	ddmDeleteCalls := setupPortGroupMock(t, &mode)
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
	resource.Update(ctx, fwresource.UpdateRequest{Plan: updatedPlan, State: createResponse.State}, updateResponse)
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
	assert.Equal(t, 1, *ddmDeleteCalls, "Expected DDM delete to be called once")

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
	resource.Update(ctx, fwresource.UpdateRequest{Plan: plan, State: state}, updateResponse)
	assert.True(t, updateResponse.Diagnostics.HasError())

	deleteResponse := &fwresource.DeleteResponse{}
	resource.Delete(ctx, fwresource.DeleteRequest{State: state}, deleteResponse)
	// Delete warns on failure but doesn't error, so resource is removed from state
	assert.False(t, deleteResponse.Diagnostics.HasError())
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

// TestUnitPortGroupResourceSingleItemResponseShape tests that GetPortGroup
// successfully parses the single-item payload shape returned by real F5OS hardware
// (under "f5-portgroup:portgroup") in addition to collection shapes.
func TestUnitPortGroupResourceSingleItemResponseShape(t *testing.T) {
	testAccPreUnitCheck(t)
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
		// Single-item shape returned by live F5OS appliance
		_, _ = fmt.Fprint(w, `{"f5-portgroup:portgroup":[{"portgroup_name":"1/1","config":{"name":"1/1","mode":"MODE_10GB","f5-ddm:ddm":{"f5-ddm:ddm-poll-frequency":30}}}]}`)
	})
	mux.HandleFunc(portGroupPath+"/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `resource "f5os_portgroup" "test" {
  name = "1/1"
  mode = "MODE_10GB"
  ddm_poll_frequency = 30
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_portgroup.test", "id", "1/1"),
					resource.TestCheckResourceAttr("f5os_portgroup.test", "mode", "MODE_10GB"),
					resource.TestCheckResourceAttr("f5os_portgroup.test", "ddm_poll_frequency", "30"),
				),
			},
		},
	})
}

// TestAccPortGroupResourceLifecycle tests the full Create, Read, Update, and
// Destroy lifecycle of f5os_portgroup on a live rSeries device.
// This test discovers a port group without optics/state, tests DDM polling
// frequency changes (no reboot), and finally tests a MODE change (which
// triggers a device reboot and waits for recovery).
func TestAccPortGroupResourceLifecycle(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	// Identify port group target, current settings and its availabe modes
	pgName, currentMode, currentDDM, altMode := findPortGroupAndOptions(t)
	if pgName == "" {
		t.Skip("No port group without state found on device")
	}
	newDDM := map[bool]string{true: "0", false: "60"}[currentDDM == "60"]

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create and read initial config with discovered current mode
			{
				Config: fmt.Sprintf(`resource "f5os_portgroup" "acc_test" {
  name = "%s"
  mode = "%s"
}`, pgName, currentMode),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_portgroup.acc_test", "id", pgName),
					resource.TestCheckResourceAttr("f5os_portgroup.acc_test", "name", pgName),
					resource.TestCheckResourceAttrSet("f5os_portgroup.acc_test", "mode"),
				),
			},
			// Step 2: Import
			{
				ResourceName:      "f5os_portgroup.acc_test",
				ImportState:       true,
				ImportStateVerify: true,
			},

			// Step 3: Update
			{
				Config: fmt.Sprintf(`resource "f5os_portgroup" "acc_test" {
  name               = "%s"
  mode               = "%s"
  ddm_poll_frequency = "%s"
}`, pgName, currentMode, newDDM),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_portgroup.acc_test", "id", pgName),
					resource.TestCheckResourceAttrSet("f5os_portgroup.acc_test", "mode"),
					resource.TestCheckResourceAttr("f5os_portgroup.acc_test", "ddm_poll_frequency", newDDM),
				),
			},

			// Step 3: MODE change (will trigger device reboot)
			{
				Config: fmt.Sprintf(`resource "f5os_portgroup" "acc_test" {
  name = "%s"
  mode = "%s"
}`, pgName, altMode),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_portgroup.acc_test", "id", pgName),
					resource.TestCheckResourceAttr("f5os_portgroup.acc_test", "mode", altMode),
				),
			},
			// Step 3: Restore original mode before destroy
			{
				Config: fmt.Sprintf(`resource "f5os_portgroup" "acc_test" {
  name = "%s"
  mode = "%s"
}`, pgName, currentMode),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_portgroup.acc_test", "id", pgName),
					resource.TestCheckResourceAttr("f5os_portgroup.acc_test", "mode", currentMode),
				),
			},
			// Step 4: Destroy is automatic
		},
	})
}

// findPortGroupAndOptions discovers the first port group without optics/state
// (and with no adjacent port groups that have state), its current mode, current DDM
// poll frequency, and an alternate supported mode by querying the device CLI via SSH.
func findPortGroupAndOptions(t *testing.T) (pgName, currentMode, currentDDM, altMode string) {
	t.Helper()
	client, err := newTestClientFromEnv()
	if err != nil {
		t.Fatalf("failed to create client: %s", err)
	}

	// Get all port groups by querying the collection endpoint
	resp, err := client.GetRequest("/f5-portgroup:portgroups")
	if err != nil {
		t.Fatalf("failed to query port groups: %s", err)
	}

	// Parse response to find port groups without state
	var parsed struct {
		PortGroups struct {
			PortGroup []struct {
				Name  string `json:"portgroup_name"`
				State struct {
					OpticState string `json:"optic-state"`
				} `json:"state"`
				Config struct {
					Mode string `json:"mode"`
					DDM  struct {
						PollFrequency string `json:"f5-ddm:ddm-poll-frequency"`
					} `json:"f5-ddm:ddm"`
				} `json:"config"`
			} `json:"portgroup"`
		} `json:"f5-portgroup:portgroups"`
	}

	if err := json.Unmarshal(resp, &parsed); err != nil {
		t.Fatalf("failed to parse port groups: %s", err)
	}

	// Find first port group without optic-state AND no adjacent port groups with state
	// (adjacent port groups must be homogeneous in mode)
	for i, pg := range parsed.PortGroups.PortGroup {
		if pg.State.OpticState == "" {
			// Check if adjacent port groups (i-1, i+1) also have no state
			pgNum := i

			// Check left neighbor
			if pgNum > 0 && parsed.PortGroups.PortGroup[pgNum-1].State.OpticState != "" {
				continue
			}

			// Check right neighbor
			if pgNum < len(parsed.PortGroups.PortGroup)-1 && parsed.PortGroups.PortGroup[pgNum+1].State.OpticState != "" {
				continue
			}

			pgName = pg.Name
			currentMode = pg.Config.Mode
			currentDDM = pg.Config.DDM.PollFrequency
			if currentDDM == "" {
				currentDDM = "30"
			}
			break
		}
	}

	if pgName == "" {
		return "", "", "", ""
	}

	// Query CLI to discover supported modes for this port group
	supportedModes := discoverPortGroupModes(t, pgName)
	if len(supportedModes) == 0 {
		t.Fatalf("No supported modes discovered for port group %s", pgName)
	}

	// Find an alternate mode different from current
	for _, mode := range supportedModes {
		if mode != currentMode {
			altMode = mode
			break
		}
	}

	if altMode == "" {
		t.Fatalf("No alternate mode found for port group %s (only mode available: %s)", pgName, currentMode)
	}

	return pgName, currentMode, currentDDM, altMode
}

// discoverPortGroupModes uses expect to SSH into the device and query supported modes via CLI
func discoverPortGroupModes(t *testing.T, pgName string) []string {
	t.Helper()
	host := os.Getenv("F5OS_HOST")
	username := os.Getenv("F5OS_USERNAME")
	password := os.Getenv("F5OS_PASSWORD")

	if host == "" || username == "" || password == "" {
		t.Logf("F5OS_HOST, F5OS_USERNAME, F5OS_PASSWORD not set, returning default modes")
		return []string{"MODE_100GB", "MODE_4x25GB", "MODE_40GB", "MODE_4x10GB", "MODE_10GB", "MODE_25GB", "MODE_400GB", "MODE_4x100GB"}
	}

	// Create expect script to query CLI for supported modes
	expectScript := fmt.Sprintf(`
set timeout 10
spawn sshpass -p "%s" ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null %s@%s
expect "admin@"
send "config\r"
expect "(config)#"
send "portgroups portgroup %s config mode \t\t\r"
expect "(config)#"
puts $expect_out(buffer)
send "exit\r"
`, password, username, host, pgName)

	// Run expect script
	cmd := exec.Command("expect", "-c", expectScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Failed to query CLI via expect: %v, returning default modes", err)
		return []string{"MODE_100GB", "MODE_4x25GB", "MODE_40GB", "MODE_4x10GB", "MODE_10GB", "MODE_25GB", "MODE_400GB", "MODE_4x100GB"}
	}

	// Parse output for MODE_* strings
	modes := extractModes(string(output))
	if len(modes) > 0 {
		t.Logf("Discovered %d supported modes via CLI for portgroup %s: %v", len(modes), pgName, modes)
		return modes
	}

	t.Logf("Could not discover modes from CLI output, returning default modes. Output: %s", string(output))
	return []string{"MODE_100GB", "MODE_4x25GB", "MODE_40GB", "MODE_4x10GB", "MODE_10GB", "MODE_25GB", "MODE_400GB", "MODE_4x100GB"}
}

// extractModes parses CLI output to find MODE_* strings
func extractModes(output string) []string {
	var modes []string
	// Look for lines with MODE_* enums (e.g., MODE_10GB, MODE_25GB)
	modePattern := regexp.MustCompile(`MODE_[A-Za-z0-9x]+`)
	matches := modePattern.FindAllString(output, -1)

	// Remove duplicates
	seen := make(map[string]bool)
	for _, mode := range matches {
		if !seen[mode] {
			modes = append(modes, mode)
			seen[mode] = true
		}
	}

	return modes
}
