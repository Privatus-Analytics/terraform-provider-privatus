package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/privatus-analytics/terraform-provider-privatus/internal/client"
)

// ---- fake API --------------------------------------------------------------

type fakeAPI struct {
	mu       sync.Mutex
	sites    map[string]map[string]any
	goals    map[string]map[string]any
	rules    map[string]map[string]any
	nextRule int
	requests []string
}

func newFakeAPI(t *testing.T) (*fakeAPI, *client.Client) {
	t.Helper()
	f := &fakeAPI{sites: map[string]map[string]any{}, goals: map[string]map[string]any{}, rules: map[string]map[string]any{}, nextRule: 41}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	return f, client.New(srv.URL, "pat_test", "test")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func notFound(w http.ResponseWriter) {
	writeJSON(w, 404, map[string]any{"error": map[string]any{"code": "not_found", "message": "Not found"}})
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer pat_test" {
		writeJSON(w, 401, map[string]any{"error": map[string]any{"code": "unauthorized", "message": "bad token"}})
		return
	}
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	body := map[string]any{}
	if b, _ := io.ReadAll(r.Body); len(b) > 0 {
		_ = json.Unmarshal(b, &body)
	}
	p := strings.Split(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), ".json"), "/")

	switch {
	case r.Method == "GET" && len(p) == 1 && p[0] == "workspaces":
		writeJSON(w, 200, map[string]any{
			"data": []any{
				map[string]any{"id": "ws_A", "name": "Acme", "plan": "pro", "plan_events": 100000, "role": "owner", "timezone": "UTC", "currency": "USD"},
				map[string]any{"id": "ws_B", "name": "Beta Agency", "plan": "free"},
			},
			"meta": map[string]any{"page": 1, "per_page": 100, "total": 2, "total_pages": 1},
		})
	case r.Method == "POST" && len(p) == 3 && p[0] == "workspaces" && p[2] == "sites":
		site := map[string]any{
			"id": "pa_NEW", "workspace_id": p[1], "domain": body["domain"], "name": body["domain"],
			"timezone": "UTC", "currency": "USD",
			"extra_domains": []any{}, "labels": []any{}, "allowed_params": []any{},
			"honor_dnt": false, "honor_gpc": true, "pii_scrubber": true, "bot_filtering": true,
			"retention_months": nil, "traffic_rule_modes": map[string]any{}, "public_dashboard": false,
			"snippet":    `<script defer data-site="pa_NEW" src="https://privatusanalytics.com/js/pa.js"></script>`,
			"created_at": "2026-09-29T00:00:00Z",
		}
		for k, v := range body {
			site[k] = v
		}
		f.sites["pa_NEW"] = site
		writeJSON(w, 201, map[string]any{"data": site})
	case len(p) == 2 && p[0] == "sites":
		site, ok := f.sites[p[1]]
		if !ok {
			notFound(w)
			return
		}
		switch r.Method {
		case "GET":
			writeJSON(w, 200, map[string]any{"data": site})
		case "PATCH":
			for k, v := range body {
				site[k] = v
			}
			writeJSON(w, 200, map[string]any{"data": site})
		case "DELETE":
			if body["confirm"] != site["domain"] {
				writeJSON(w, 422, map[string]any{"error": map[string]any{"code": "validation_failed", "message": "Confirm is invalid"}})
				return
			}
			delete(f.sites, p[1])
			writeJSON(w, 200, map[string]any{"data": site})
		}
	case r.Method == "POST" && len(p) == 3 && p[2] == "goals":
		g := map[string]any{"id": "goal_1", "match_type": "exact", "value_mode": "none", "count_mode": "once_per_visit", "archived": false, "conditions": []any{}}
		for k, v := range body {
			g[k] = v
		}
		f.goals["goal_1"] = g
		writeJSON(w, 201, map[string]any{"data": g})
	case len(p) == 5 && p[2] == "goals" && (p[4] == "archive" || p[4] == "unarchive"):
		g, ok := f.goals[p[3]]
		if !ok {
			notFound(w)
			return
		}
		g["archived"] = p[4] == "archive"
		writeJSON(w, 200, map[string]any{"data": g})
	case len(p) == 4 && p[2] == "goals":
		g, ok := f.goals[p[3]]
		if !ok {
			notFound(w)
			return
		}
		switch r.Method {
		case "GET":
			writeJSON(w, 200, map[string]any{"data": g})
		case "PATCH":
			for k, v := range body {
				if v == nil {
					delete(g, k)
				} else {
					g[k] = v
				}
			}
			writeJSON(w, 200, map[string]any{"data": g})
		case "DELETE":
			delete(f.goals, p[3])
			writeJSON(w, 200, map[string]any{"data": g})
		}
	case len(p) == 3 && p[2] == "traffic_rules":
		if r.Method == "GET" {
			list := []any{}
			for _, rule := range f.rules {
				list = append(list, rule)
			}
			writeJSON(w, 200, map[string]any{"data": list, "meta": map[string]any{}})
			return
		}
		f.nextRule++
		pattern := body["pattern"].(string)
		if body["list"] == "countries" {
			pattern = strings.ToUpper(pattern)
		}
		rule := map[string]any{"id": f.nextRule, "list": body["list"], "pattern": pattern, "label": body["label"], "mode": "block"}
		f.rules[fmt.Sprint(f.nextRule)] = rule
		writeJSON(w, 201, map[string]any{"data": rule})
	case len(p) == 4 && p[2] == "traffic_rules":
		rule, ok := f.rules[p[3]]
		if !ok || r.Method == "GET" { // no member GET route, like the real API
			notFound(w)
			return
		}
		switch r.Method {
		case "PATCH":
			rule["pattern"] = body["pattern"]
			if l, _ := body["label"].(string); l == "" {
				rule["label"] = nil
			} else {
				rule["label"] = l
			}
			writeJSON(w, 200, map[string]any{"data": rule})
		case "DELETE":
			delete(f.rules, p[3])
			writeJSON(w, 200, map[string]any{"data": rule})
		}
	default:
		notFound(w)
	}
}

// ---- framework plumbing ----------------------------------------------------

func resourceSchema(t *testing.T, r resource.Resource) tfsdk.State {
	t.Helper()
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}
	return tfsdk.State{Schema: resp.Schema, Raw: tftypes.NewValue(resp.Schema.Type().TerraformType(context.Background()), nil)}
}

func toPlan(t *testing.T, s tfsdk.State, model any) tfsdk.Plan {
	t.Helper()
	st := s
	if d := st.Set(context.Background(), model); d.HasError() {
		t.Fatalf("building plan: %v", d)
	}
	return tfsdk.Plan{Schema: st.Schema, Raw: st.Raw}
}

func configure(t *testing.T, r resource.Resource, c *client.Client) {
	t.Helper()
	resp := &resource.ConfigureResponse{}
	r.(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: c}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
}

// ---- tests -----------------------------------------------------------------

func TestProviderSchemaIsValid(t *testing.T) {
	server, err := providerserver.NewProtocol6WithError(New("test")())()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("schema diagnostic: %s: %s", d.Summary, d.Detail)
		}
	}
	for _, name := range []string{"privatus_site", "privatus_goal", "privatus_traffic_rule"} {
		if _, ok := resp.ResourceSchemas[name]; !ok {
			t.Errorf("missing resource %s", name)
		}
	}
	if _, ok := resp.DataSourceSchemas["privatus_workspace"]; !ok {
		t.Error("missing data source privatus_workspace")
	}
	attrs := map[string]bool{}
	for _, a := range resp.Provider.Block.Attributes {
		attrs[a.Name] = a.Sensitive
	}
	if sensitive, ok := attrs["api_token"]; !ok || !sensitive {
		t.Error("api_token must exist and be sensitive")
	}
	if _, ok := attrs["endpoint"]; !ok {
		t.Error("missing endpoint attribute")
	}
}

func unknownSite() siteModel {
	return siteModel{
		ID: types.StringUnknown(), WorkspaceID: types.StringValue("ws_A"), Domain: types.StringValue("example.com"),
		Name: types.StringValue("Example"), Timezone: types.StringUnknown(), Currency: types.StringValue("EUR"),
		ExtraDomains: types.ListValueMust(types.StringType, nil), Labels: types.ListUnknown(types.StringType),
		HonorDNT: types.BoolValue(true), HonorGPC: types.BoolUnknown(), PIIScrubber: types.BoolUnknown(),
		BotFiltering: types.BoolUnknown(), AllowedParams: types.ListUnknown(types.StringType),
		RetentionMonths: types.Int64Unknown(), TrafficRuleModes: types.MapUnknown(types.StringType),
		PublicDashboard: types.BoolUnknown(), Snippet: types.StringUnknown(), CreatedAt: types.StringUnknown(),
	}
}

func TestSiteLifecycle(t *testing.T) {
	ctx := context.Background()
	api, c := newFakeAPI(t)
	r := NewSiteResource()
	configure(t, r, c)
	empty := resourceSchema(t, r)

	// Create: POST with create-only fields, then PATCH for the rest.
	plan := toPlan(t, empty, unknownSite())
	cresp := &resource.CreateResponse{State: empty}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, cresp)
	if cresp.Diagnostics.HasError() {
		t.Fatal(cresp.Diagnostics)
	}
	var got siteModel
	cresp.State.Get(ctx, &got)
	if got.ID.ValueString() != "pa_NEW" || got.Name.ValueString() != "Example" || !got.HonorDNT.ValueBool() || got.Currency.ValueString() != "EUR" {
		t.Fatalf("unexpected state after create: %+v", got)
	}
	if !got.RetentionMonths.IsNull() || !strings.Contains(got.Snippet.ValueString(), "pa_NEW") {
		t.Fatalf("computed values wrong: %+v", got)
	}
	want := []string{"POST /workspaces/ws_A/sites.json", "PATCH /sites/pa_NEW.json"}
	if strings.Join(api.requests, ",") != strings.Join(want, ",") {
		t.Fatalf("requests = %v", api.requests)
	}
	if _, sentOnCreate := api.sites["pa_NEW"]["honor_dnt"]; !sentOnCreate {
		t.Fatal("honor_dnt should have been PATCHed")
	}

	// Update
	upd := got
	upd.Name = types.StringValue("Renamed")
	uresp := &resource.UpdateResponse{State: cresp.State}
	r.Update(ctx, resource.UpdateRequest{Plan: toPlan(t, empty, upd), State: cresp.State}, uresp)
	if uresp.Diagnostics.HasError() {
		t.Fatal(uresp.Diagnostics)
	}
	if api.sites["pa_NEW"]["name"] != "Renamed" {
		t.Fatalf("name not updated: %v", api.sites["pa_NEW"]["name"])
	}

	// Delete sends the domain as confirmation.
	dresp := &resource.DeleteResponse{}
	r.Delete(ctx, resource.DeleteRequest{State: uresp.State}, dresp)
	if dresp.Diagnostics.HasError() {
		t.Fatal(dresp.Diagnostics)
	}
	if len(api.sites) != 0 {
		t.Fatal("site not deleted")
	}

	// Read of a deleted site removes it from state.
	rresp := &resource.ReadResponse{State: uresp.State}
	r.Read(ctx, resource.ReadRequest{State: uresp.State}, rresp)
	if rresp.Diagnostics.HasError() || !rresp.State.Raw.IsNull() {
		t.Fatalf("expected resource removed, diags=%v", rresp.Diagnostics)
	}
}

func TestGoalLifecycle(t *testing.T) {
	ctx := context.Background()
	api, c := newFakeAPI(t)
	r := NewGoalResource()
	configure(t, r, c)
	empty := resourceSchema(t, r)

	m := goalModel{
		ID: types.StringUnknown(), SiteID: types.StringValue("pa_1"), Name: types.StringValue("Purchase"),
		Kind: types.StringValue("event"), MatchType: types.StringUnknown(), Pattern: types.StringNull(),
		Hostname: types.StringNull(), EventName: types.StringValue("purchase"),
		Conditions:      []goalConditionModel{{Key: types.StringValue("plan"), Operator: types.StringValue("is"), Value: types.StringValue("pro")}},
		ScrollThreshold: types.Int64Null(), TimeThresholdSeconds: types.Int64Null(), ValueMode: types.StringValue("revenue"),
		FixedValue: types.Float64Null(), CountMode: types.StringUnknown(), Archived: types.BoolValue(true),
	}
	cresp := &resource.CreateResponse{State: empty}
	r.Create(ctx, resource.CreateRequest{Plan: toPlan(t, empty, m)}, cresp)
	if cresp.Diagnostics.HasError() {
		t.Fatal(cresp.Diagnostics)
	}
	var got goalModel
	cresp.State.Get(ctx, &got)
	if got.ID.ValueString() != "goal_1" || got.MatchType.ValueString() != "exact" || !got.Archived.ValueBool() || len(got.Conditions) != 1 {
		t.Fatalf("state = %+v", got)
	}
	if api.requests[1] != "POST /sites/pa_1/goals/goal_1/archive.json" {
		t.Fatalf("requests = %v", api.requests)
	}

	// Unarchive + clear event name on update.
	upd := got
	upd.Archived = types.BoolValue(false)
	upd.Kind = types.StringValue("page")
	upd.EventName = types.StringNull()
	upd.Pattern = types.StringValue("/thanks")
	upd.Conditions = nil
	uresp := &resource.UpdateResponse{State: cresp.State}
	r.Update(ctx, resource.UpdateRequest{Plan: toPlan(t, empty, upd), State: cresp.State}, uresp)
	if uresp.Diagnostics.HasError() {
		t.Fatal(uresp.Diagnostics)
	}
	uresp.State.Get(ctx, &got)
	if got.Archived.ValueBool() || !got.EventName.IsNull() || got.Pattern.ValueString() != "/thanks" {
		t.Fatalf("state after update = %+v", got)
	}

	// Import id parsing.
	iresp := &resource.ImportStateResponse{State: empty}
	r.(resource.ResourceWithImportState).ImportState(ctx, resource.ImportStateRequest{ID: "pa_1/goal_1"}, iresp)
	if iresp.Diagnostics.HasError() {
		t.Fatal(iresp.Diagnostics)
	}
	bad := &resource.ImportStateResponse{State: empty}
	r.(resource.ResourceWithImportState).ImportState(ctx, resource.ImportStateRequest{ID: "goal_1"}, bad)
	if !bad.Diagnostics.HasError() {
		t.Fatal("expected invalid import id error")
	}
}

func TestTrafficRuleLifecycle(t *testing.T) {
	ctx := context.Background()
	api, c := newFakeAPI(t)
	r := NewTrafficRuleResource()
	configure(t, r, c)
	empty := resourceSchema(t, r)

	m := trafficRuleModel{ID: types.StringUnknown(), SiteID: types.StringValue("pa_1"), List: types.StringValue("countries"),
		Pattern: types.StringValue("xx"), Label: types.StringValue("Test"), Mode: types.StringUnknown()}
	cresp := &resource.CreateResponse{State: empty}
	r.Create(ctx, resource.CreateRequest{Plan: toPlan(t, empty, m)}, cresp)
	if cresp.Diagnostics.HasError() {
		t.Fatal(cresp.Diagnostics)
	}
	var got trafficRuleModel
	cresp.State.Get(ctx, &got)
	// Numeric API id becomes a string; configured lower-case pattern is kept.
	if got.ID.ValueString() != "42" || got.Pattern.ValueString() != "xx" || got.Mode.ValueString() != "block" {
		t.Fatalf("state = %+v", got)
	}

	// Read falls back to the list endpoint (no member GET).
	rresp := &resource.ReadResponse{State: cresp.State}
	r.Read(ctx, resource.ReadRequest{State: cresp.State}, rresp)
	if rresp.Diagnostics.HasError() || rresp.State.Raw.IsNull() {
		t.Fatalf("read failed: %v", rresp.Diagnostics)
	}

	// Removing the label sends a blank label.
	upd := got
	upd.Label = types.StringNull()
	uresp := &resource.UpdateResponse{State: cresp.State}
	r.Update(ctx, resource.UpdateRequest{Plan: toPlan(t, empty, upd), State: cresp.State}, uresp)
	if uresp.Diagnostics.HasError() {
		t.Fatal(uresp.Diagnostics)
	}
	if api.rules["42"]["label"] != nil {
		t.Fatalf("label not cleared: %v", api.rules["42"])
	}

	dresp := &resource.DeleteResponse{}
	r.Delete(ctx, resource.DeleteRequest{State: uresp.State}, dresp)
	if dresp.Diagnostics.HasError() || len(api.rules) != 0 {
		t.Fatalf("delete failed: %v", dresp.Diagnostics)
	}
	gone := &resource.ReadResponse{State: uresp.State}
	r.Read(ctx, resource.ReadRequest{State: uresp.State}, gone)
	if gone.Diagnostics.HasError() || !gone.State.Raw.IsNull() {
		t.Fatal("deleted rule should be removed from state")
	}
}

func TestWorkspaceDataSource(t *testing.T) {
	ctx := context.Background()
	_, c := newFakeAPI(t)
	d := NewWorkspaceDataSource()
	d.(datasource.DataSourceWithConfigure).Configure(ctx, datasource.ConfigureRequest{ProviderData: c}, &datasource.ConfigureResponse{})
	sresp := &datasource.SchemaResponse{}
	d.Schema(ctx, datasource.SchemaRequest{}, sresp)
	typ := sresp.Schema.Type().TerraformType(ctx)

	run := func(id, name string) (workspaceModel, bool) {
		cfg := tfsdk.State{Schema: sresp.Schema, Raw: tftypes.NewValue(typ, nil)}
		idV, nameV := types.StringNull(), types.StringNull()
		if id != "" {
			idV = types.StringValue(id)
		}
		if name != "" {
			nameV = types.StringValue(name)
		}
		cfg.Set(ctx, workspaceModel{ID: idV, Name: nameV, Timezone: types.StringNull(), Currency: types.StringNull(),
			Plan: types.StringNull(), PlanEvents: types.Int64Null(), SubscriptionStatus: types.StringNull(),
			Role: types.StringNull(), CreatedAt: types.StringNull()})
		resp := &datasource.ReadResponse{State: tfsdk.State{Schema: sresp.Schema, Raw: tftypes.NewValue(typ, nil)}}
		d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: sresp.Schema, Raw: cfg.Raw}}, resp)
		var out workspaceModel
		resp.State.Get(ctx, &out)
		return out, !resp.Diagnostics.HasError()
	}

	if w, ok := run("", "acme"); !ok || w.ID.ValueString() != "ws_A" || w.Plan.ValueString() != "pro" || w.PlanEvents.ValueInt64() != 100000 {
		t.Fatalf("by name: ok=%v %+v", ok, w)
	}
	if w, ok := run("ws_B", ""); !ok || w.Name.ValueString() != "Beta Agency" {
		t.Fatalf("by id: ok=%v %+v", ok, w)
	}
	if _, ok := run("", ""); ok {
		t.Fatal("two workspaces and no filter should be an error")
	}
	if _, ok := run("ws_missing", ""); ok {
		t.Fatal("unknown id should be an error")
	}
}
