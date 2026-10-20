package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/privatus-analytics/terraform-provider-privatus/internal/client"
)

var (
	_ datasource.DataSource              = &workspaceDataSource{}
	_ datasource.DataSourceWithConfigure = &workspaceDataSource{}
)

// NewWorkspaceDataSource returns the privatus_workspace data source.
func NewWorkspaceDataSource() datasource.DataSource { return &workspaceDataSource{} }

type workspaceDataSource struct{ client *client.Client }

type workspaceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Timezone           types.String `tfsdk:"timezone"`
	Currency           types.String `tfsdk:"currency"`
	Plan               types.String `tfsdk:"plan"`
	PlanEvents         types.Int64  `tfsdk:"plan_events"`
	SubscriptionStatus types.String `tfsdk:"subscription_status"`
	Role               types.String `tfsdk:"role"`
	CreatedAt          types.String `tfsdk:"created_at"`
}

func (d *workspaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (d *workspaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	computed := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Description: desc, Computed: true}
	}
	resp.Schema = schema.Schema{
		Description: "Looks up a workspace the API token can access, by `id` or `name`. " +
			"With neither set, the token must have access to exactly one workspace.",
		Attributes: map[string]schema.Attribute{
			"id":                  schema.StringAttribute{Description: "Workspace id (`ws_…`).", Optional: true, Computed: true},
			"name":                schema.StringAttribute{Description: "Workspace name (exact, case-insensitive match).", Optional: true, Computed: true},
			"timezone":            computed("Default reporting timezone."),
			"currency":            computed("Default currency."),
			"plan":                computed("`free`, `pro`, `business` or `enterprise`."),
			"plan_events":         schema.Int64Attribute{Description: "Events per month included in the plan.", Computed: true},
			"subscription_status": computed("Billing subscription status."),
			"role":                computed("Your role in this workspace."),
			"created_at":          computed("Creation timestamp (RFC 3339)."),
		},
	}
}

func (d *workspaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

// pickWorkspace selects one workspace by id or name.
func pickWorkspace(all []client.Workspace, id, name string) (*client.Workspace, error) {
	var matches []client.Workspace
	for _, w := range all {
		if (id == "" || w.ID == id) && (name == "" || strings.EqualFold(w.Name, name)) {
			matches = append(matches, w)
		}
	}
	switch {
	case len(matches) == 1:
		return &matches[0], nil
	case len(matches) == 0:
		return nil, fmt.Errorf("no workspace matches id=%q name=%q (the token can see %d workspaces)", id, name, len(all))
	default:
		return nil, fmt.Errorf("%d workspaces match id=%q name=%q; set `id` to pick one", len(matches), id, name)
	}
}

func (d *workspaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg workspaceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	all, err := d.client.ListWorkspaces(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing workspaces", err.Error())
		return
	}
	w, err := pickWorkspace(all, cfg.ID.ValueString(), cfg.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Workspace not found", err.Error())
		return
	}
	state := workspaceModel{
		ID:                 types.StringValue(w.ID),
		Name:               types.StringValue(w.Name),
		Timezone:           types.StringValue(w.Timezone),
		Currency:           types.StringValue(w.Currency),
		Plan:               types.StringValue(w.Plan),
		PlanEvents:         types.Int64Value(w.PlanEvents),
		SubscriptionStatus: types.StringValue(w.SubscriptionStatus),
		Role:               types.StringValue(w.Role),
		CreatedAt:          types.StringValue(w.CreatedAt),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
