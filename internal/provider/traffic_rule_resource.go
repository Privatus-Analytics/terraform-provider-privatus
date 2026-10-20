package provider

import (
	"context"
	"strings"

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
	_ resource.Resource                = &trafficRuleResource{}
	_ resource.ResourceWithConfigure   = &trafficRuleResource{}
	_ resource.ResourceWithImportState = &trafficRuleResource{}
)

// TrafficRuleLists are the lists a traffic rule can belong to.
var TrafficRuleLists = []string{"hostnames", "paths", "referrers", "countries", "ip_ranges", "user_agents", "events"}

// NewTrafficRuleResource returns the privatus_traffic_rule resource.
func NewTrafficRuleResource() resource.Resource { return &trafficRuleResource{} }

type trafficRuleResource struct{ client *client.Client }

type trafficRuleModel struct {
	ID      types.String `tfsdk:"id"`
	SiteID  types.String `tfsdk:"site_id"`
	List    types.String `tfsdk:"list"`
	Pattern types.String `tfsdk:"pattern"`
	Label   types.String `tfsdk:"label"`
	Mode    types.String `tfsdk:"mode"`
}

func (r *trafficRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_traffic_rule"
}

func (r *trafficRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "One entry in a site's block/allow lists, checked at ingest. Whether a list blocks or allows " +
			"is set per list with `privatus_site.traffic_rule_modes`. IP rules are matched in memory; IPs are never stored.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Rule id.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"site_id": schema.StringAttribute{
				Description:   "Site id (`pa_…`). Changing it forces a new rule.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"list": schema.StringAttribute{
				Description:   "One of `" + strings.Join(TrafficRuleLists, "`, `") + "`. Changing it forces a new rule.",
				Required:      true,
				Validators:    []validator.String{stringvalidator.OneOf(TrafficRuleLists...)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"pattern": schema.StringAttribute{
				Description: "Value to match; `*` is a wildcard. `ip_ranges` take an IP or CIDR, `countries` an ISO code.",
				Required:    true,
			},
			"label": schema.StringAttribute{
				Description: "Optional note, e.g. `Office`.",
				Optional:    true,
			},
			"mode": schema.StringAttribute{
				Description:   "`block` or `allow` (the mode of the rule's list).",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *trafficRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

// ruleToModel copies the API rule into the model. The server upper-cases
// country codes and trims patterns; keep the configured spelling when it
// only differs in case/whitespace so plans stay clean.
func ruleToModel(t *client.TrafficRule, m *trafficRuleModel) {
	m.ID = types.StringValue(string(t.ID))
	m.List = types.StringValue(t.List)
	if !set(m.Pattern) || !strings.EqualFold(strings.TrimSpace(m.Pattern.ValueString()), t.Pattern) {
		m.Pattern = types.StringValue(t.Pattern)
	}
	m.Label = optString(t.Label)
	if t.Mode == "" {
		t.Mode = "block"
	}
	m.Mode = types.StringValue(t.Mode)
}

func (r *trafficRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan trafficRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	fields := map[string]any{"list": plan.List.ValueString(), "pattern": plan.Pattern.ValueString()}
	if set(plan.Label) {
		fields["label"] = plan.Label.ValueString()
	}
	rule, err := r.client.CreateTrafficRule(ctx, plan.SiteID.ValueString(), fields)
	if err != nil {
		resp.Diagnostics.AddError("Error creating traffic rule", err.Error())
		return
	}
	ruleToModel(rule, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *trafficRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state trafficRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule, err := r.client.GetTrafficRule(ctx, state.SiteID.ValueString(), state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading traffic rule", err.Error())
		return
	}
	ruleToModel(rule, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *trafficRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state trafficRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The API treats a blank label as "remove the label".
	fields := map[string]any{"pattern": plan.Pattern.ValueString(), "label": ""}
	if set(plan.Label) {
		fields["label"] = plan.Label.ValueString()
	}
	rule, err := r.client.UpdateTrafficRule(ctx, state.SiteID.ValueString(), state.ID.ValueString(), fields)
	if err != nil {
		resp.Diagnostics.AddError("Error updating traffic rule", err.Error())
		return
	}
	plan.ID = state.ID
	ruleToModel(rule, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *trafficRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state trafficRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteTrafficRule(ctx, state.SiteID.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting traffic rule", err.Error())
	}
}

// ImportState accepts "<site_id>/<rule_id>".
func (r *trafficRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	siteID, ruleID, err := splitImportID(req.ID, "site_id", "rule_id")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("site_id"), siteID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), ruleID)...)
}
