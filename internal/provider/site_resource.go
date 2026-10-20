package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/privatus-analytics/terraform-provider-privatus/internal/client"
)

var (
	_ resource.Resource                = &siteResource{}
	_ resource.ResourceWithConfigure   = &siteResource{}
	_ resource.ResourceWithImportState = &siteResource{}
)

// NewSiteResource returns the privatus_site resource.
func NewSiteResource() resource.Resource { return &siteResource{} }

type siteResource struct{ client *client.Client }

type siteModel struct {
	ID               types.String `tfsdk:"id"`
	WorkspaceID      types.String `tfsdk:"workspace_id"`
	Domain           types.String `tfsdk:"domain"`
	Name             types.String `tfsdk:"name"`
	Timezone         types.String `tfsdk:"timezone"`
	Currency         types.String `tfsdk:"currency"`
	ExtraDomains     types.List   `tfsdk:"extra_domains"`
	Labels           types.List   `tfsdk:"labels"`
	HonorDNT         types.Bool   `tfsdk:"honor_dnt"`
	HonorGPC         types.Bool   `tfsdk:"honor_gpc"`
	PIIScrubber      types.Bool   `tfsdk:"pii_scrubber"`
	BotFiltering     types.Bool   `tfsdk:"bot_filtering"`
	AllowedParams    types.List   `tfsdk:"allowed_params"`
	RetentionMonths  types.Int64  `tfsdk:"retention_months"`
	TrafficRuleModes types.Map    `tfsdk:"traffic_rule_modes"`
	PublicDashboard  types.Bool   `tfsdk:"public_dashboard"`
	Snippet          types.String `tfsdk:"snippet"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

func (r *siteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site"
}

func optStr(desc string, validators ...validator.String) schema.StringAttribute {
	return schema.StringAttribute{
		Description:   desc,
		Optional:      true,
		Computed:      true,
		Validators:    validators,
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

func optBool(desc string) schema.BoolAttribute {
	return schema.BoolAttribute{
		Description:   desc,
		Optional:      true,
		Computed:      true,
		PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
	}
}

func optList(desc string) schema.ListAttribute {
	return schema.ListAttribute{
		Description:   desc,
		ElementType:   types.StringType,
		Optional:      true,
		Computed:      true,
		PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
	}
}

func (r *siteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A website tracked by Privatus Analytics. Its `id` (`pa_…`) is the public site id used in the tracking snippet.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Site id (`pa_…`), used as `data-site` in the tracking snippet.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"workspace_id": schema.StringAttribute{
				Description:   "Workspace (`ws_…`) that owns the site. Changing it forces a new site.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"domain": schema.StringAttribute{
				Description: "Primary domain, e.g. `example.com`.",
				Required:    true,
			},
			"name":     optStr("Display name (defaults to the domain)."),
			"timezone": optStr("Reporting timezone (IANA name, e.g. `Europe/Berlin`)."),
			"currency": optStr("Currency for revenue (ISO 4217)."),
			"extra_domains":  optList("Additional hostnames accepted for this site."),
			"labels":         optList("Labels used to organise sites."),
			"honor_dnt":      optBool("Skip visitors that send Do Not Track."),
			"honor_gpc":      optBool("Skip visitors that send Global Privacy Control."),
			"pii_scrubber":   optBool("Scrub e-mail addresses and other PII from URLs and props."),
			"bot_filtering":  optBool("Filter known bots and automation."),
			"allowed_params": optList("Extra query parameters kept on page URLs."),
			"retention_months": schema.Int64Attribute{
				Description:   "Months of raw data to keep. Omit to keep the server default (unlimited).",
				Optional:      true,
				Computed:      true,
				Validators:    []validator.Int64{int64validator.AtLeast(1)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"traffic_rule_modes": schema.MapAttribute{
				Description:   "Per-list mode (`block` or `allow`) for traffic rules, keyed by list name (hostnames, paths, referrers, countries, ip_ranges, user_agents, events).",
				ElementType:   types.StringType,
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
			},
			"public_dashboard": optBool("Make the dashboard publicly viewable."),
			"snippet": schema.StringAttribute{
				Description:   "HTML tracking snippet for this site.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				Description:   "Creation timestamp (RFC 3339).",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *siteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

// createFields are accepted by POST /workspaces/{ws}/sites.json.
func siteCreateFields(m siteModel) map[string]any {
	f := map[string]any{"domain": m.Domain.ValueString()}
	if set(m.Name) {
		f["name"] = m.Name.ValueString()
	}
	if set(m.Timezone) {
		f["timezone"] = m.Timezone.ValueString()
	}
	if set(m.Currency) {
		f["currency"] = m.Currency.ValueString()
	}
	return f
}

// sitePatchFields are the settings only PATCH /sites/{id}.json accepts.
func sitePatchFields(ctx context.Context, m siteModel, diags *diag.Diagnostics) map[string]any {
	f := map[string]any{}
	if set(m.ExtraDomains) {
		f["extra_domains"] = listToStrings(ctx, m.ExtraDomains, diags)
	}
	if set(m.Labels) {
		f["labels"] = listToStrings(ctx, m.Labels, diags)
	}
	if set(m.HonorDNT) {
		f["honor_dnt"] = m.HonorDNT.ValueBool()
	}
	if set(m.HonorGPC) {
		f["honor_gpc"] = m.HonorGPC.ValueBool()
	}
	if set(m.PIIScrubber) {
		f["pii_scrubber"] = m.PIIScrubber.ValueBool()
	}
	if set(m.BotFiltering) {
		f["bot_filtering"] = m.BotFiltering.ValueBool()
	}
	if set(m.AllowedParams) {
		f["allowed_params"] = listToStrings(ctx, m.AllowedParams, diags)
	}
	if set(m.RetentionMonths) {
		f["retention_months"] = m.RetentionMonths.ValueInt64()
	}
	if set(m.TrafficRuleModes) {
		f["traffic_rule_modes"] = mapToStrings(ctx, m.TrafficRuleModes, diags)
	}
	if set(m.PublicDashboard) {
		f["public_dashboard"] = m.PublicDashboard.ValueBool()
	}
	return f
}

func siteToModel(ctx context.Context, s *client.Site, m *siteModel, diags *diag.Diagnostics) {
	m.ID = types.StringValue(s.ID)
	if s.WorkspaceID != "" {
		m.WorkspaceID = types.StringValue(s.WorkspaceID)
	}
	m.Domain = types.StringValue(s.Domain)
	m.Name = types.StringValue(s.Name)
	m.Timezone = types.StringValue(s.Timezone)
	m.Currency = types.StringValue(s.Currency)
	m.ExtraDomains = stringList(ctx, s.ExtraDomains, diags)
	m.Labels = stringList(ctx, s.Labels, diags)
	m.HonorDNT = types.BoolValue(s.HonorDNT)
	m.HonorGPC = types.BoolValue(s.HonorGPC)
	m.PIIScrubber = types.BoolValue(s.PIIScrubber)
	m.BotFiltering = types.BoolValue(s.BotFiltering)
	m.AllowedParams = stringList(ctx, s.AllowedParams, diags)
	m.RetentionMonths = optInt64(s.RetentionMonths)
	m.TrafficRuleModes = stringMap(ctx, s.TrafficRuleModes, diags)
	m.PublicDashboard = types.BoolValue(s.PublicDashboard)
	m.Snippet = types.StringValue(s.Snippet)
	m.CreatedAt = types.StringValue(s.CreatedAt)
}

func (r *siteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	site, err := r.client.CreateSite(ctx, plan.WorkspaceID.ValueString(), siteCreateFields(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating site", err.Error())
		return
	}

	patch := sitePatchFields(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) > 0 {
		updated, err := r.client.UpdateSite(ctx, site.ID, patch)
		if err != nil {
			// Save what exists so Terraform marks it tainted instead of orphaning it.
			siteToModel(ctx, site, &plan, &resp.Diagnostics)
			resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
			resp.Diagnostics.AddError("Error applying site settings after create", err.Error())
			return
		}
		site = updated
	}

	siteToModel(ctx, site, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *siteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	site, err := r.client.GetSite(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading site", err.Error())
		return
	}
	siteToModel(ctx, site, &state, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *siteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state siteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fields := siteCreateFields(plan)
	for k, v := range sitePatchFields(ctx, plan, &resp.Diagnostics) {
		fields[k] = v
	}
	if resp.Diagnostics.HasError() {
		return
	}
	site, err := r.client.UpdateSite(ctx, state.ID.ValueString(), fields)
	if err != nil {
		resp.Diagnostics.AddError("Error updating site", err.Error())
		return
	}
	siteToModel(ctx, site, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *siteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteSite(ctx, state.ID.ValueString(), state.Domain.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting site", err.Error())
	}
}

func (r *siteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
