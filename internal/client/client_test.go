package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(srv.URL+"/", "pat_test", "test-agent")
}

func TestNormalizePath(t *testing.T) {
	cases := map[string]string{
		"workspaces":            "/workspaces.json",
		"/workspaces":           "/workspaces.json",
		"/sites/pa_1.json":      "/sites/pa_1.json",
		"/sites/pa_1/goals/g_1": "/sites/pa_1/goals/g_1.json",
	}
	for in, want := range cases {
		if got := NormalizePath(in); got != want {
			t.Errorf("NormalizePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAuthHeadersAndDecode(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer pat_test" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "test-agent" {
			t.Errorf("User-Agent = %q", got)
		}
		if r.URL.Path != "/sites/pa_ABC.json" {
			t.Errorf("path = %q", r.URL.Path)
		}
		fmt.Fprint(w, `{"data":{"id":"pa_ABC","domain":"example.com","retention_months":null,"labels":["a"]},"meta":{}}`)
	})
	s, err := c.GetSite(context.Background(), "pa_ABC")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != "pa_ABC" || s.Domain != "example.com" || s.RetentionMonths != nil || len(s.Labels) != 1 {
		t.Fatalf("unexpected site %+v", s)
	}
}

func TestErrorEnvelopeAndNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"code":"not_found","message":"Site not found","details":{}}}`)
	})
	_, err := c.GetSite(context.Background(), "pa_gone")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Fatalf("expected IsNotFound, got %v", err)
	}
	apiErr := err.(*APIError)
	if apiErr.Code != "not_found" || apiErr.Message != "Site not found" {
		t.Fatalf("unexpected error %+v", apiErr)
	}
}

func TestValidationError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"error":{"code":"validation_failed","message":"Domain is invalid","details":{"domain":["is invalid"]}}}`)
	})
	_, err := c.CreateSite(context.Background(), "ws_1", map[string]any{"domain": "nope"})
	if err == nil || IsNotFound(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if want := "privatus API error 422 (validation_failed): Domain is invalid"; len(err.Error()) < len(want) || err.Error()[:len(want)] != want {
		t.Fatalf("error text = %q", err.Error())
	}
}

func TestNonJSONError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, "<html>bad gateway</html>")
	})
	_, err := c.GetSite(context.Background(), "pa_1")
	var apiErr *APIError
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.StatusCode != 502 {
		t.Fatalf("unexpected %v", err)
	}
}

func TestCreateSiteSendsJSONBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws_1/sites.json" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		var body map[string]any
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Fatal(err)
		}
		if body["domain"] != "example.com" || body["name"] != "Example" {
			t.Errorf("body = %v", body)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"data":{"id":"pa_NEW","workspace_id":"ws_1","domain":"example.com","name":"Example"}}`)
	})
	s, err := c.CreateSite(context.Background(), "ws_1", map[string]any{"domain": "example.com", "name": "Example"})
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != "pa_NEW" || s.WorkspaceID != "ws_1" {
		t.Fatalf("site = %+v", s)
	}
}

func TestDeleteSiteSendsConfirm(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s", r.Method)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["confirm"] != "example.com" {
			t.Errorf("confirm = %q", body["confirm"])
		}
		fmt.Fprint(w, `{"data":{"id":"pa_1"}}`)
	})
	if err := c.DeleteSite(context.Background(), "pa_1", "example.com"); err != nil {
		t.Fatal(err)
	}
}

func TestListWorkspacesPaginates(t *testing.T) {
	calls := 0
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if r.URL.Query().Get("per_page") != "100" {
			t.Errorf("per_page = %q", r.URL.Query().Get("per_page"))
		}
		fmt.Fprintf(w, `{"data":[{"id":"ws_%d","name":"W%d"}],"meta":{"page":%d,"per_page":100,"total":3,"total_pages":3}}`, page, page, page)
	})
	ws, err := c.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 3 || calls != 3 || ws[2].ID != "ws_3" {
		t.Fatalf("got %d workspaces in %d calls: %+v", len(ws), calls, ws)
	}
}

func TestGoalAndTrafficRulePaths(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sites/pa_1/goals/goal_1.json":
			fmt.Fprint(w, `{"data":{"id":"goal_1","name":"Signup","kind":"page","match_type":"exact","pattern":"/thanks","fixed_value":9.5}}`)
		case "/sites/pa_1/traffic_rules/tr_1.json":
			fmt.Fprint(w, `{"data":{"id":"tr_1","list":"paths","pattern":"/admin/*","label":null}}`)
		default:
			w.WriteHeader(404)
			fmt.Fprint(w, `{"error":{"code":"not_found","message":"nope"}}`)
		}
	})
	g, err := c.GetGoal(context.Background(), "pa_1", "goal_1")
	if err != nil {
		t.Fatal(err)
	}
	if g.Pattern == nil || *g.Pattern != "/thanks" || g.FixedValue == nil || *g.FixedValue != 9.5 {
		t.Fatalf("goal = %+v", g)
	}
	r, err := c.GetTrafficRule(context.Background(), "pa_1", "tr_1")
	if err != nil {
		t.Fatal(err)
	}
	if r.List != "paths" || r.Label != nil {
		t.Fatalf("rule = %+v", r)
	}
	if _, err := c.GetGoal(context.Background(), "pa_1", "goal_x"); !IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}
