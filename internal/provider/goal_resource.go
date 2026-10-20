package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/privatus-analytics/terraform-provider-privatus/internal/client"
)

var (
	_ resource.Resource                = &goalResource{}
	_ resource.ResourceWithConfigure   = &goalResource{}
	_ resource.ResourceWithImportState = &goalResource{}
)

// NewGoalResource returns the privatus_goal resource.
func NewGoalResource() resource.Resource { return &goalResource{} }

type goalResource struct{ client *client.Client }

type goalConditionModel struct {
	Key      types.String `tfsdk:"key"`
	Operator types.String `tfsdk:"operator"`
	Value    types.String `tfsdk:"value"`
}

type goalModel struct {
	ID                   types.String         `tfsdk:"id"`
	SiteID               types.String         `tfsdk:"site_id"`
	Name                 types.String         `tfsdk:"name"`
	Kind                 types.String         `tfsdk:"kind"`
	MatchType            types.String         `tfsdk:"match_type"`
	Pattern              types.String         `tfsdk:"pattern"`
	Hostname             types.String         `tfsdk:"hostname"`
	EventName            types.String         `tfsdk:"event_name"`
	Conditions           []goalConditionModel `tfsdk:"conditions"`
	ScrollThreshold      types.Int64          `tfsdk:"scroll_threshold"`
	TimeThresholdSeconds types.Int64          `tfsdk:"time_threshold_seconds"`
	ValueMode            types.String         `tfsdk:"value_mode"`
	FixedValue           types.Float64        `tfsdk:"fixed_value"`
	CountMode            types.String         `tfsdk:"count_mode"`
	Archived             types.Bool           `tfsdk:"archived"`
}

func (r *goalResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_goal"
}

func (r *goalResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A conversion goal on a Privatus Analytics site (page visit, custom event, engagement, outbound link or file download).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Goal id (`goal_…`).",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"site_id": schema.StringAttribute{
				Description:   "Site id (`pa_…`). Changing it forces a new goal.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{Description: "Goal name shown in reports.", Required: true},
			"kind": schema.StringAttribute{
				Description: "One of `page`, `event`, `engagement`, `outbound`, `download`.",
				Required:    true,
				Validators:  []validator.String{stringvalidator.OneOf("page", "event", "engagement", "outbound", "download")},
			},
			"match_type": optStr("How `pattern` is matched: `exact`, `prefix`, `glob` or `regex`.",
				stringvalidator.OneOf("exact", "prefix", "glob", "regex")),
			"pattern":    schema.StringAttribute{Description: "Path (page/engagement goals) or URL pattern (outbound/download goals).", Optional: true},
			"hostname":   schema.StringAttribute{Description: "Restrict the goal to one hostname.", Optional: true},
			"event_name": schema.StringAttribute{Description: "Custom event name (for `event` goals).", Optional: true},
			"conditions": schema.ListNestedAttribute{
				Description: "Property conditions that must all match (event goals).",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{Description: "Property name.", Required: true},
						"operator": schema.StringAttribute{
							Description: "`is`, `is_not`, `contains`, `gt` or `lt`.",
							Required:    true,
							Validators:  []validator.String{stringvalidator.OneOf("is", "is_not", "contains", "gt", "lt")},
						},
						"value": schema.StringAttribute{Description: "Value to compare with.", Required: true},
					},
				},
			},
			"scroll_threshold": schema.Int64Attribute{
				Description: "Scroll depth % required (engagement goals).",
				Optional:    true,
				Validators:  []validator.Int64{int64validator.Between(1, 100)},
			},
			"time_threshold_seconds": schema.Int64Attribute{
				Description: "Engaged seconds required (engagement goals).",
				Optional:    true,
				Validators:  []validator.Int64{int64validator.AtLeast(1)},
			},
			"value_mode": optStr("Conversion value: `none`, `fixed` or `revenue`.",
				stringvalidator.OneOf("none", "fixed", "revenue")),
			"fixed_value": schema.Float64Attribute{
				Description: "Fixed conversion value in major currency units (when `value_mode = \"fixed\"`).",
				Optional:    true,
				Validators:  []validator.Float64{float64validator.AtLeast(0)},
			},
			"count_mode": optStr("`once_per_visit` or `every`.",
				stringvalidator.OneOf("once_per_visit", "every")),
			"archived": optBool("Archived goals stop counting but keep history."),
		},
	}
}

func (r *goalResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

// goalFields builds the request body. When forUpdate is true, cleared
// optional attributes are sent as null so the API removes them.
func goalFields(m goalModel, forUpdate bool) map[string]any {
	f := map[string]any{
		"name": m.Name.ValueString(),
		"kind": m.Kind.ValueString(),
	}
	optS := func(key string, v types.String) {
		switch {
		case set(v):
			f[key] = v.ValueString()
		case forUpdate && v.IsNull():
			f[key] = nil
		}
	}
	optI := func(key string, v types.Int64) {
		switch {
		case set(v):
			f[key] = v.ValueInt64()
		case forUpdate && v.IsNull():
			f[key] = nil
		}
	}
	optS("match_type", m.MatchType)
	optS("pattern", m.Pattern)
	optS("hostname", m.Hostname)
	optS("event_name", m.EventName)
	optI("scroll_threshold", m.ScrollThreshold)
	optI("time_threshold_seconds", m.TimeThresholdSeconds)
	optS("value_mode", m.ValueMode)
	optS("count_mode", m.CountMode)
	if set(m.FixedValue) {
		f["fixed_value"] = m.FixedValue.ValueFloat64()
	} else if forUpdate && m.FixedValue.IsNull() {
		f["fixed_value"] = nil
	}
	if m.Conditions != nil || forUpdate {
		conds := make([]client.GoalCondition, 0, len(m.Conditions))
		for _, c := range m.Conditions {
			conds = append(conds, client.GoalCondition{
				Key: c.Key.ValueString(), Operator: c.Operator.ValueString(), Value: c.Value.ValueString(),
			})
		}
		f["conditions"] = conds
	}
	return f
}

func goalToModel(g *client.Goal, m *goalModel) {
	m.ID = types.StringValue(g.ID)
	m.Name = types.StringValue(g.Name)
	m.Kind = types.StringValue(g.Kind)
	m.MatchType = types.StringValue(g.MatchType)
	m.Pattern = optString(g.Pattern)
	m.Hostname = optString(g.Hostname)
	m.EventName = optString(g.EventName)
	m.ScrollThreshold = optInt64(g.ScrollThreshold)
	m.TimeThresholdSeconds = optInt64(g.TimeThresholdSeconds)
	m.ValueMode = types.StringValue(g.ValueMode)
	m.FixedValue = optFloat64(g.FixedValue)
	m.CountMode = types.StringValue(g.CountMode)
	m.Archived = types.BoolValue(g.Archived)
	m.Conditions = nil
	for _, c := range g.Conditions {
		m.Conditions = append(m.Conditions, goalConditionModel{
			Key: types.StringValue(c.Key), Operator: types.StringValue(c.Operator), Value: types.StringValue(c.Value),
		})
	}
}

func (r *goalResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan goalModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.client.CreateGoal(ctx, plan.SiteID.ValueString(), goalFields(plan, false))
	if err != nil {
		resp.Diagnostics.AddError("Error creating goal", err.Error())
		return
	}
	if set(plan.Archived) && plan.Archived.ValueBool() && !g.Archived {
		archived, err := r.client.SetGoalArchived(ctx, plan.SiteID.ValueString(), g.ID, true)
		if err != nil {
			goalToModel(g, &plan)
			resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
			resp.Diagnostics.AddError("Error archiving goal after create", err.Error())
			return
		}
		g = archived
	}
	goalToModel(g, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *goalResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state goalModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.client.GetGoal(ctx, state.SiteID.ValueString(), state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading goal", err.Error())
		return
	}
	goalToModel(g, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *goalResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state goalModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.client.UpdateGoal(ctx, state.SiteID.ValueString(), state.ID.ValueString(), goalFields(plan, true))
	if err != nil {
		resp.Diagnostics.AddError("Error updating goal", err.Error())
		return
	}
	// Archiving has its own endpoints (POST …/archive and …/unarchive).
	if set(plan.Archived) && plan.Archived.ValueBool() != g.Archived {
		g, err = r.client.SetGoalArchived(ctx, state.SiteID.ValueString(), state.ID.ValueString(), plan.Archived.ValueBool())
		if err != nil {
			resp.Diagnostics.AddError("Error changing goal archive state", err.Error())
			return
		}
	}
	goalToModel(g, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *goalResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state goalModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteGoal(ctx, state.SiteID.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting goal", err.Error())
	}
}

// ImportState accepts "<site_id>/<goal_id>".
func (r *goalResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	siteID, goalID, err := splitImportID(req.ID, "site_id", "goal_id")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("site_id"), siteID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), goalID)...)
}
