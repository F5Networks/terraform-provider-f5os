package provider

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- Mocks & helpers ---

type mockSnmp struct {
	calls          []string
	payloads       [][]byte
	delTargets     []string
	delCommunities []string
	delUsers       []string
	failOn         string
}

func (m *mockSnmp) record(call string, payload []byte) error {
	m.calls = append(m.calls, call)
	if payload != nil {
		m.payloads = append(m.payloads, payload)
	}
	if m.failOn == call {
		return errors.New("forced error: " + call)
	}
	return nil
}

func (m *mockSnmp) CreateSnmpCommunities(b []byte) error { return m.record("CreateSnmpCommunities", b) }
func (m *mockSnmp) CreateSnmpUsers(b []byte) error       { return m.record("CreateSnmpUsers", b) }
func (m *mockSnmp) CreateSnmpTargets(b []byte) error     { return m.record("CreateSnmpTargets", b) }
func (m *mockSnmp) UpdateSnmpMib(b []byte) error         { return m.record("UpdateSnmpMib", b) }
func (m *mockSnmp) UpdateSnmpCommunities(b []byte) error { return m.record("UpdateSnmpCommunities", b) }
func (m *mockSnmp) UpdateSnmpUsers(b []byte) error       { return m.record("UpdateSnmpUsers", b) }
func (m *mockSnmp) UpdateSnmpTargets(b []byte) error     { return m.record("UpdateSnmpTargets", b) }
func (m *mockSnmp) DeleteSnmpTarget(name string) error {
	m.calls = append(m.calls, "DeleteSnmpTarget")
	m.delTargets = append(m.delTargets, name)
	if m.failOn == "DeleteSnmpTarget" {
		return errors.New("forced error: DeleteSnmpTarget")
	}
	return nil
}
func (m *mockSnmp) DeleteSnmpCommunity(name string) error {
	m.calls = append(m.calls, "DeleteSnmpCommunity")
	m.delCommunities = append(m.delCommunities, name)
	if m.failOn == "DeleteSnmpCommunity" {
		return errors.New("forced error: DeleteSnmpCommunity")
	}
	return nil
}
func (m *mockSnmp) DeleteSnmpUser(name string) error {
	m.calls = append(m.calls, "DeleteSnmpUser")
	m.delUsers = append(m.delUsers, name)
	if m.failOn == "DeleteSnmpUser" {
		return errors.New("forced error: DeleteSnmpUser")
	}
	return nil
}

// --- Tests ---

// 1) Community payload
func TestBuildCommunityPayload(t *testing.T) {
	r := &SnmpResource{}

	communities := []SnmpCommunityModel{
		{
			Name:          types.StringValue("commA"),
			SecurityModel: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("v1"), types.StringValue("v2c")}),
		},
		{
			Name:          types.StringValue("commB"),
			SecurityModel: types.ListNull(types.StringType), // default to v1
		},
	}

	payload := r.buildCommunityPayload(communities)

	// Normalize for comparison
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	want := map[string]interface{}{
		"communities": map[string]interface{}{
			"community": []interface{}{
				map[string]interface{}{
					"name": "commA",
					"config": map[string]interface{}{
						"name":           "commA",
						"security-model": []interface{}{"v1", "v2c"},
					},
				},
				map[string]interface{}{
					"name": "commB",
					"config": map[string]interface{}{
						"name":           "commB",
						"security-model": []interface{}{"v1"},
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("payload mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

// 2) Target payload (ipv4/ipv6)
func TestBuildTargetPayload(t *testing.T) {
	r := &SnmpResource{}
	targets := []SnmpTargetModel{
		{
			Name:          types.StringValue("t4"),
			SecurityModel: types.StringValue("v2c"),
			Community:     types.StringValue("commA"),
			Ipv4Address:   types.StringValue("192.0.2.10"),
			Port:          types.Int64Value(162),
		},
		{
			Name:        types.StringValue("t6"),
			User:        types.StringValue("user1"),
			Ipv6Address: types.StringValue("2001:db8::1"),
			Port:        types.Int64Value(1162),
		},
	}

	payload := r.buildTargetPayload(targets)
	b, _ := json.Marshal(payload)
	var got map[string]interface{}
	_ = json.Unmarshal(b, &got)

	// quick shape assertions
	list := got["targets"].(map[string]interface{})["target"].([]interface{})
	if len(list) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(list))
	}
	// ipv4 entry
	t4 := list[0].(map[string]interface{})["config"].(map[string]interface{})
	if t4["community"] != "commA" || t4["security-model"] != "v2c" {
		t.Fatalf("unexpected ipv4 config: %#v", t4)
	}
	if t4["ipv4"].(map[string]interface{})["address"] != "192.0.2.10" {
		t.Fatalf("ipv4 address mismatch")
	}
	// ipv6 entry
	t6 := list[1].(map[string]interface{})["config"].(map[string]interface{})
	if t6["user"] != "user1" {
		t.Fatalf("unexpected ipv6 user: %#v", t6)
	}
	if t6["ipv6"].(map[string]interface{})["address"] != "2001:db8::1" {
		t.Fatalf("ipv6 address mismatch")
	}
}

// 3) User payload
func TestBuildUserPayload(t *testing.T) {
	r := &SnmpResource{}
	users := []SnmpUserModel{
		{
			Name:          types.StringValue("u1"),
			AuthProto:     types.StringValue("sha"),
			AuthPasswd:    types.StringValue("apass"),
			PrivacyProto:  types.StringValue("aes"),
			PrivacyPasswd: types.StringValue("ppass"),
		},
	}
	payload := r.buildUserPayload(users)
	b, _ := json.Marshal(payload)
	var got map[string]interface{}
	_ = json.Unmarshal(b, &got)
	list := got["users"].(map[string]interface{})["user"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected 1 user, got %d", len(list))
	}
	cfg := list[0].(map[string]interface{})["config"].(map[string]interface{})
	if cfg["authentication-protocol"] != "sha" || cfg["privacy-protocol"] != "aes" {
		t.Fatalf("unexpected user config: %#v", cfg)
	}
}

// 4) MIB payload
func TestBuildMibPayload(t *testing.T) {
	r := &SnmpResource{}
	mib := &SnmpMibModel{
		SysName:     types.StringValue("device1"),
		SysContact:  types.StringValue("admin@example.com"),
		SysLocation: types.StringValue("DC-1"),
	}
	payload := r.buildMibPayload(mib)
	b, _ := json.Marshal(payload)
	var got map[string]interface{}
	_ = json.Unmarshal(b, &got)
	sys := got["SNMPv2-MIB:system"].(map[string]interface{})
	if sys["SNMPv2-MIB:sysName"] != "device1" || sys["SNMPv2-MIB:sysLocation"] != "DC-1" {
		t.Fatalf("unexpected MIB payload: %#v", sys)
	}
}

// 5) createSnmpConfig order & calls
func TestCreateSnmpConfig_OrderAndCalls(t *testing.T) {
	m := &mockSnmp{}
	r := &SnmpResource{client: m}
	ctx := context.Background()

	communities := []SnmpCommunityModel{{Name: types.StringValue("c1")}}
	users := []SnmpUserModel{{Name: types.StringValue("u1")}}
	targets := []SnmpTargetModel{{Name: types.StringValue("t1"), Port: types.Int64Value(162), Ipv4Address: types.StringValue("192.0.2.1")}}
	mib := &SnmpMibModel{SysName: types.StringValue("dev")}

	if err := r.createSnmpConfig(ctx, communities, targets, users, mib); err != nil {
		t.Fatalf("createSnmpConfig error: %v", err)
	}

	want := []string{"CreateSnmpCommunities", "CreateSnmpUsers", "CreateSnmpTargets", "UpdateSnmpMib"}
	if !reflect.DeepEqual(m.calls, want) {
		t.Fatalf("call order mismatch\n got: %v\nwant: %v", m.calls, want)
	}
}

// 6) updateSnmpConfig order & calls
func TestUpdateSnmpConfig_OrderAndCalls(t *testing.T) {
	m := &mockSnmp{}
	r := &SnmpResource{client: m}
	ctx := context.Background()

	communities := []SnmpCommunityModel{{Name: types.StringValue("c1")}}
	users := []SnmpUserModel{{Name: types.StringValue("u1")}}
	targets := []SnmpTargetModel{{Name: types.StringValue("t1"), Port: types.Int64Value(162), Ipv4Address: types.StringValue("192.0.2.1")}}
	mib := &SnmpMibModel{SysName: types.StringValue("dev")}

	if err := r.updateSnmpConfig(ctx, communities, targets, users, mib); err != nil {
		t.Fatalf("updateSnmpConfig error: %v", err)
	}

	want := []string{"UpdateSnmpCommunities", "UpdateSnmpUsers", "UpdateSnmpTargets", "UpdateSnmpMib"}
	if !reflect.DeepEqual(m.calls, want) {
		t.Fatalf("call order mismatch\n got: %v\nwant: %v", m.calls, want)
	}
}

// 7) deleteSnmpConfig order & names
func TestDeleteSnmpConfig_OrderAndNames(t *testing.T) {
	m := &mockSnmp{}
	r := &SnmpResource{client: m}
	ctx := context.Background()

	targets := []SnmpTargetModel{{Name: types.StringValue("t1")}, {Name: types.StringValue("t2")}}
	communities := []SnmpCommunityModel{{Name: types.StringValue("c1")}}
	users := []SnmpUserModel{{Name: types.StringValue("u1")}}

	if err := r.deleteSnmpConfig(ctx, communities, targets, users); err != nil {
		t.Fatalf("deleteSnmpConfig error: %v", err)
	}

	// Verify call order roughly: deletes recorded as methods called
	wantPrefix := []string{"DeleteSnmpTarget", "DeleteSnmpTarget", "DeleteSnmpCommunity", "DeleteSnmpUser"}
	if !reflect.DeepEqual(m.calls, wantPrefix) {
		t.Fatalf("delete order mismatch\n got: %v\nwant: %v", m.calls, wantPrefix)
	}
	if !reflect.DeepEqual(m.delTargets, []string{"t1", "t2"}) {
		t.Fatalf("deleted targets mismatch: %v", m.delTargets)
	}
	if !reflect.DeepEqual(m.delCommunities, []string{"c1"}) {
		t.Fatalf("deleted communities mismatch: %v", m.delCommunities)
	}
	if !reflect.DeepEqual(m.delUsers, []string{"u1"}) {
		t.Fatalf("deleted users mismatch: %v", m.delUsers)
	}
}

// // 8) computeSnmpResourceID stable hashing
// func TestComputeSnmpResourceID_Deterministic(t *testing.T) {
// 	mib := &SnmpMibModel{SysName: types.StringValue("dev")}
// 	aCom := []SnmpCommunityModel{{Name: types.StringValue("a")}, {Name: types.StringValue("b")}}
// 	bCom := []SnmpCommunityModel{{Name: types.StringValue("b")}, {Name: types.StringValue("a")}}
// 	aTar := []SnmpTargetModel{{Name: types.StringValue("t1")}, {Name: types.StringValue("t2")}}
// 	bTar := []SnmpTargetModel{{Name: types.StringValue("t2")}, {Name: types.StringValue("t1")}}
// 	aUsr := []SnmpUserModel{{Name: types.StringValue("u1")}, {Name: types.StringValue("u2")}}
// 	bUsr := []SnmpUserModel{{Name: types.StringValue("u2")}, {Name: types.StringValue("u1")}}

// 	id1 := computeSnmpResourceID(aCom, aTar, aUsr, mib)
// 	id2 := computeSnmpResourceID(bCom, bTar, bUsr, mib)
// 	if id1 != id2 {
// 		t.Fatalf("hash should be deterministic; got %q vs %q", id1, id2)
// 	}
// }
