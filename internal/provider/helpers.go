package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/privatus-analytics/terraform-provider-privatus/internal/client"
)

// set reports whether a planned/configured value is known and non-null.
func set(v attr.Value) bool { return !v.IsNull() && !v.IsUnknown() }

// clientFrom extracts the API client configured by the provider.
func clientFrom(data any, diags *diag.Diagnostics) *client.Client {
	if data == nil {
		return nil // provider not configured yet (e.g. during validation)
	}
	c, ok := data.(*client.Client)
	if !ok {
		diags.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", data))
		return nil
	}
	return c
}

// splitImportID parses "<a>/<b>" import ids.
func splitImportID(id, first, second string) (string, string, error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected import id in the form %s/%s, got %q", first, second, id)
	}
	return parts[0], parts[1], nil
}

func optString(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

func optInt64(p *int64) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*p)
}

func optFloat64(p *float64) types.Float64 {
	if p == nil {
		return types.Float64Null()
	}
	return types.Float64Value(*p)
}

// stringList converts an API string slice to a (never null) list value.
func stringList(ctx context.Context, in []string, diags *diag.Diagnostics) types.List {
	if in == nil {
		in = []string{}
	}
	l, d := types.ListValueFrom(ctx, types.StringType, in)
	diags.Append(d...)
	return l
}

func listToStrings(ctx context.Context, l types.List, diags *diag.Diagnostics) []string {
	out := []string{}
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out
}

func stringMap(ctx context.Context, in map[string]string, diags *diag.Diagnostics) types.Map {
	if in == nil {
		in = map[string]string{}
	}
	m, d := types.MapValueFrom(ctx, types.StringType, in)
	diags.Append(d...)
	return m
}

func mapToStrings(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]string {
	out := map[string]string{}
	diags.Append(m.ElementsAs(ctx, &out, false)...)
	return out
}
