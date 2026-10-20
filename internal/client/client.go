// Package client is a small JSON client for the Privatus Analytics
// management API (https://privatusanalytics.com/openapi.json).
//
// Every endpoint answers `{ "data": …, "meta": {…} }` on success and
// `{ "error": { "code", "message", "details" } }` with a 4xx/5xx status on
// failure. Authentication is a personal API token (`pat_…`) sent as a
// Bearer token.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is used when no base URL is configured.
const DefaultBaseURL = "https://privatusanalytics.com"

// Client talks to the Privatus Analytics management API.
type Client struct {
	BaseURL    string
	Token      string
	UserAgent  string
	HTTPClient *http.Client
}

// New returns a client with sane defaults.
func New(baseURL, token, userAgent string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if userAgent == "" {
		userAgent = "terraform-provider-privatus"
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		UserAgent:  userAgent,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Meta is the `meta` object of a response. Collections carry pagination.
type Meta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// APIError is returned for any non-2xx response.
type APIError struct {
	StatusCode int
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details"`
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	s := fmt.Sprintf("privatus API error %d", e.StatusCode)
	if e.Code != "" {
		s += " (" + e.Code + ")"
	}
	s += ": " + msg
	if len(e.Details) > 0 {
		if b, err := json.Marshal(e.Details); err == nil {
			s += " " + string(b)
		}
	}
	return s
}

// IsNotFound reports whether err is an API 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// NormalizePath makes sure the path starts with "/" and ends in ".json"
// (the API answers JSON when `.json` is appended to any app path).
func NormalizePath(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, ".json") {
		p += ".json"
	}
	return p
}

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Meta  json.RawMessage `json:"meta"`
	Error *APIError       `json:"error"`
}

// Do performs a request. body (if non-nil) is JSON-encoded; the `data`
// member of the response is decoded into out (if non-nil).
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body, out any) (*Meta, error) {
	u := c.BaseURL + NormalizePath(path)
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}

	var env envelope
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &env); err != nil && resp.StatusCode < 300 {
			return nil, fmt.Errorf("decoding response from %s %s: %w", method, path, err)
		}
	}

	if resp.StatusCode >= 300 {
		apiErr := env.Error
		if apiErr == nil {
			apiErr = &APIError{Message: strings.TrimSpace(string(raw))}
			if len(apiErr.Message) > 200 {
				apiErr.Message = apiErr.Message[:200]
			}
		}
		apiErr.StatusCode = resp.StatusCode
		return nil, apiErr
	}

	meta := &Meta{}
	if len(env.Meta) > 0 && string(env.Meta) != "null" {
		_ = json.Unmarshal(env.Meta, meta)
	}
	if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return meta, fmt.Errorf("decoding data from %s %s: %w", method, path, err)
		}
	}
	return meta, nil
}

// ---- Workspaces ----------------------------------------------------------

// Workspace mirrors the API workspace object.
type Workspace struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Timezone           string `json:"timezone"`
	Currency           string `json:"currency"`
	Plan               string `json:"plan"`
	PlanEvents         int64  `json:"plan_events"`
	SubscriptionStatus string `json:"subscription_status"`
	Role               string `json:"role"`
	CreatedAt          string `json:"created_at"`
}

// ListWorkspaces returns every workspace the token can see, following
// pagination (100 per page).
func (c *Client) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	var all []Workspace
	for page := 1; page <= 1000; page++ {
		var batch []Workspace
		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"100"}}
		meta, err := c.Do(ctx, http.MethodGet, "/workspaces", q, nil, &batch)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) == 0 || meta == nil {
			break
		}
		if meta.TotalPages > 0 {
			if page >= meta.TotalPages {
				break
			}
		} else if len(batch) < 100 {
			break
		}
	}
	return all, nil
}

// ---- Sites ---------------------------------------------------------------

// Site mirrors the API site object (subset relevant to configuration).
type Site struct {
	ID               string            `json:"id"`
	WorkspaceID      string            `json:"workspace_id"`
	Domain           string            `json:"domain"`
	Name             string            `json:"name"`
	ExtraDomains     []string          `json:"extra_domains"`
	MultiDomainMode  string            `json:"multi_domain_mode"`
	Timezone         string            `json:"timezone"`
	Currency         string            `json:"currency"`
	Labels           []string          `json:"labels"`
	HonorDNT         bool              `json:"honor_dnt"`
	HonorGPC         bool              `json:"honor_gpc"`
	PIIScrubber      bool              `json:"pii_scrubber"`
	AllowedParams    []string          `json:"allowed_params"`
	RetentionMonths  *int64            `json:"retention_months"`
	BotFiltering     bool              `json:"bot_filtering"`
	TrafficRuleModes map[string]string `json:"traffic_rule_modes"`
	PublicDashboard  bool              `json:"public_dashboard"`
	Snippet          string            `json:"snippet"`
	CreatedAt        string            `json:"created_at"`
}

func sitePath(id string) string { return "/sites/" + url.PathEscape(id) }

// CreateSite calls POST /workspaces/{ws}/sites.json. Accepted fields:
// domain, name, timezone, currency.
func (c *Client) CreateSite(ctx context.Context, workspaceID string, fields map[string]any) (*Site, error) {
	var s Site
	_, err := c.Do(ctx, http.MethodPost, "/workspaces/"+url.PathEscape(workspaceID)+"/sites", nil, fields, &s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetSite calls GET /sites/{id}.json.
func (c *Client) GetSite(ctx context.Context, id string) (*Site, error) {
	var s Site
	if _, err := c.Do(ctx, http.MethodGet, sitePath(id), nil, nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// UpdateSite calls PATCH /sites/{id}.json with the given flat fields.
func (c *Client) UpdateSite(ctx context.Context, id string, fields map[string]any) (*Site, error) {
	var s Site
	if _, err := c.Do(ctx, http.MethodPatch, sitePath(id), nil, fields, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// DeleteSite calls DELETE /sites/{id}.json. The API requires the domain as
// confirmation.
func (c *Client) DeleteSite(ctx context.Context, id, domain string) error {
	_, err := c.Do(ctx, http.MethodDelete, sitePath(id), nil, map[string]any{"confirm": domain}, nil)
	return err
}

// ---- Goals (assumed endpoints, see README) -------------------------------

// GoalCondition is a property condition on an event goal.
type GoalCondition struct {
	Key      string `json:"key"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// Goal mirrors the API goal object.
type Goal struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Kind                 string          `json:"kind"`
	MatchType            string          `json:"match_type"`
	Pattern              *string         `json:"pattern"`
	Hostname             *string         `json:"hostname"`
	EventName            *string         `json:"event_name"`
	Conditions           []GoalCondition `json:"conditions"`
	ScrollThreshold      *int64          `json:"scroll_threshold"`
	TimeThresholdSeconds *int64          `json:"time_threshold_seconds"`
	ValueMode            string          `json:"value_mode"`
	FixedValue           *float64        `json:"fixed_value"`
	CountMode            string          `json:"count_mode"`
	Archived             bool            `json:"archived"`
}

func goalsPath(siteID string) string { return sitePath(siteID) + "/goals" }

// ListGoals calls GET /sites/{site_id}/goals.json (all pages).
func (c *Client) ListGoals(ctx context.Context, siteID string) ([]Goal, error) {
	var all []Goal
	for page := 1; page <= 1000; page++ {
		var batch []Goal
		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"100"}}
		meta, err := c.Do(ctx, http.MethodGet, goalsPath(siteID), q, nil, &batch)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) == 0 || meta == nil || (meta.TotalPages > 0 && page >= meta.TotalPages) || (meta.TotalPages == 0 && len(batch) < 100) {
			break
		}
	}
	return all, nil
}

// CreateGoal calls POST /sites/{site_id}/goals.json.
func (c *Client) CreateGoal(ctx context.Context, siteID string, fields map[string]any) (*Goal, error) {
	var g Goal
	if _, err := c.Do(ctx, http.MethodPost, goalsPath(siteID), nil, fields, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

// GetGoal calls GET /sites/{site_id}/goals/{id}.json.
func (c *Client) GetGoal(ctx context.Context, siteID, id string) (*Goal, error) {
	var g Goal
	if _, err := c.Do(ctx, http.MethodGet, goalsPath(siteID)+"/"+url.PathEscape(id), nil, nil, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

// UpdateGoal calls PATCH /sites/{site_id}/goals/{id}.json.
func (c *Client) UpdateGoal(ctx context.Context, siteID, id string, fields map[string]any) (*Goal, error) {
	var g Goal
	if _, err := c.Do(ctx, http.MethodPatch, goalsPath(siteID)+"/"+url.PathEscape(id), nil, fields, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

// SetGoalArchived calls POST /sites/{site_id}/goals/{id}/archive.json or
// /unarchive.json.
func (c *Client) SetGoalArchived(ctx context.Context, siteID, id string, archived bool) (*Goal, error) {
	action := "/unarchive"
	if archived {
		action = "/archive"
	}
	var g Goal
	if _, err := c.Do(ctx, http.MethodPost, goalsPath(siteID)+"/"+url.PathEscape(id)+action, nil, map[string]any{}, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

// DeleteGoal calls DELETE /sites/{site_id}/goals/{id}.json.
func (c *Client) DeleteGoal(ctx context.Context, siteID, id string) error {
	_, err := c.Do(ctx, http.MethodDelete, goalsPath(siteID)+"/"+url.PathEscape(id), nil, nil, nil)
	return err
}

// ---- Traffic rules (assumed endpoints, see README) -----------------------

// TrafficRule mirrors the API traffic rule object. The API id is an
// integer; FlexID keeps it as a string.
type TrafficRule struct {
	ID      FlexID  `json:"id"`
	List    string  `json:"list"`
	Pattern string  `json:"pattern"`
	Label   *string `json:"label"`
	Mode    string  `json:"mode"`
}

func rulesPath(siteID string) string { return sitePath(siteID) + "/traffic_rules" }

// CreateTrafficRule calls POST /sites/{site_id}/traffic_rules.json.
func (c *Client) CreateTrafficRule(ctx context.Context, siteID string, fields map[string]any) (*TrafficRule, error) {
	var r TrafficRule
	if _, err := c.Do(ctx, http.MethodPost, rulesPath(siteID), nil, fields, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// GetTrafficRule calls GET /sites/{site_id}/traffic_rules/{id}.json. If the
// server has no member GET route (404), it falls back to scanning the list
// endpoint; a rule missing from the list is reported as a 404 APIError.
func (c *Client) GetTrafficRule(ctx context.Context, siteID, id string) (*TrafficRule, error) {
	var r TrafficRule
	_, err := c.Do(ctx, http.MethodGet, rulesPath(siteID)+"/"+url.PathEscape(id), nil, nil, &r)
	if err == nil {
		return &r, nil
	}
	if !IsNotFound(err) {
		return nil, err
	}
	rules, lerr := c.ListTrafficRules(ctx, siteID)
	if lerr != nil {
		return nil, lerr
	}
	for i := range rules {
		if string(rules[i].ID) == id {
			return &rules[i], nil
		}
	}
	return nil, &APIError{StatusCode: http.StatusNotFound, Code: "not_found", Message: "traffic rule " + id + " not found"}
}

// ListTrafficRules calls GET /sites/{site_id}/traffic_rules.json (all pages).
func (c *Client) ListTrafficRules(ctx context.Context, siteID string) ([]TrafficRule, error) {
	var all []TrafficRule
	for page := 1; page <= 1000; page++ {
		var batch []TrafficRule
		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"100"}}
		meta, err := c.Do(ctx, http.MethodGet, rulesPath(siteID), q, nil, &batch)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) == 0 || meta == nil || (meta.TotalPages > 0 && page >= meta.TotalPages) || (meta.TotalPages == 0 && len(batch) < 100) {
			break
		}
	}
	return all, nil
}

// UpdateTrafficRule calls PATCH /sites/{site_id}/traffic_rules/{id}.json.
func (c *Client) UpdateTrafficRule(ctx context.Context, siteID, id string, fields map[string]any) (*TrafficRule, error) {
	var r TrafficRule
	if _, err := c.Do(ctx, http.MethodPatch, rulesPath(siteID)+"/"+url.PathEscape(id), nil, fields, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// DeleteTrafficRule calls DELETE /sites/{site_id}/traffic_rules/{id}.json.
func (c *Client) DeleteTrafficRule(ctx context.Context, siteID, id string) error {
	_, err := c.Do(ctx, http.MethodDelete, rulesPath(siteID)+"/"+url.PathEscape(id), nil, nil, nil)
	return err
}

// FlexID accepts an id encoded as either a JSON string or a JSON number.
type FlexID string

// UnmarshalJSON implements json.Unmarshaler.
func (f *FlexID) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = FlexID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("id is neither a string nor a number: %s", string(b))
	}
	*f = FlexID(n.String())
	return nil
}
