package pipeline

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/uuid"
)

const durationExpr = `EXTRACT(EPOCH FROM (COALESCE(p.completed_at, NOW()) - p.created_at))`

var runListSortColumns = map[string]string{
	"name":         "COALESCE(pd.name, '')",
	"status":       "p.status",
	"network":      "COALESCE(n.name, '')",
	"organization": "COALESCE(o.name, '')",
	"currentLevel": "p.current_level",
	"createdAt":    "p.created_at",
	"completedAt":  "p.completed_at",
	"duration":     "(" + durationExpr + ")",
}

var pipelineStatuses = map[string]struct{}{
	"pending":   {},
	"running":   {},
	"completed": {},
	"failed":    {},
}

func parseRunListParams(r *http.Request) (runListParams, error) {
	query := r.URL.Query()
	params := runListParams{
		Query:          strings.TrimSpace(query.Get("q")),
		Sort:           strings.TrimSpace(query.Get("sort")),
		Order:          strings.ToLower(strings.TrimSpace(query.Get("order"))),
		Name:           strings.TrimSpace(query.Get("name")),
		NameOp:         strings.TrimSpace(query.Get("nameOp")),
		Status:         strings.TrimSpace(query.Get("status")),
		Network:        strings.TrimSpace(query.Get("network")),
		NetworkOp:      strings.TrimSpace(query.Get("networkOp")),
		Organization:   strings.TrimSpace(query.Get("organization")),
		OrganizationOp: strings.TrimSpace(query.Get("organizationOp")),
		NetworkID:      strings.TrimSpace(query.Get("networkId")),
		OrganizationID: strings.TrimSpace(query.Get("organizationId")),
		Page:           defaultPage,
	}

	if params.Sort == "" {
		params.Sort = "createdAt"
		if params.Order == "" {
			params.Order = "desc"
		}
	} else if params.Order == "" {
		params.Order = "asc"
	}
	if _, ok := runListSortColumns[params.Sort]; !ok {
		return runListParams{}, apperror.NewBadRequestError("Invalid sort field", nil)
	}
	if params.Order != "asc" && params.Order != "desc" {
		return runListParams{}, apperror.NewBadRequestError("Invalid sort order", nil)
	}

	nameOp, err := normalizeStringOp(params.NameOp)
	if err != nil {
		return runListParams{}, err
	}
	params.NameOp = nameOp

	networkOp, err := normalizeStringOp(params.NetworkOp)
	if err != nil {
		return runListParams{}, err
	}
	params.NetworkOp = networkOp

	organizationOp, err := normalizeStringOp(params.OrganizationOp)
	if err != nil {
		return runListParams{}, err
	}
	params.OrganizationOp = organizationOp

	if params.Status != "" {
		if _, ok := pipelineStatuses[params.Status]; !ok {
			return runListParams{}, apperror.NewBadRequestError("Invalid status filter", nil)
		}
	}

	if params.NetworkID != "" && !uuid.Valid(params.NetworkID) {
		return runListParams{}, apperror.NewBadRequestError("Invalid network ID", nil)
	}
	if params.OrganizationID != "" && !uuid.Valid(params.OrganizationID) {
		return runListParams{}, apperror.NewBadRequestError("Invalid organization ID", nil)
	}

	if value := strings.TrimSpace(query.Get("page")); value != "" {
		page, convErr := strconv.Atoi(value)
		if convErr != nil || page < 1 {
			return runListParams{}, apperror.NewBadRequestError("Invalid page", nil)
		}
		params.Page = page
	}

	if value := strings.TrimSpace(query.Get("pageSize")); value != "" {
		pageSize, convErr := strconv.Atoi(value)
		if convErr != nil || pageSize < 1 {
			return runListParams{}, apperror.NewBadRequestError("Invalid page size", nil)
		}
		if pageSize > maxPageSize {
			pageSize = maxPageSize
		}
		params.PageSize = pageSize
	}

	_, hasPage := query["page"]
	_, hasPageSize := query["pageSize"]
	if (hasPage || hasPageSize) && params.PageSize == 0 {
		params.PageSize = defaultPageSize
	}

	return params, nil
}

func buildRunListQuery(params runListParams) (countSQL, listSQL string, countArgs, listArgs []any) {
	b := &queryBuilder{}
	where := []string{"n.deleted_at IS NULL"}

	if params.UserID != "" {
		where = append(where, "n.user_id = "+b.add(params.UserID))
	}
	if params.NetworkID != "" {
		where = append(where, "p.network_id = "+b.add(params.NetworkID))
	}
	if params.OrganizationID != "" {
		where = append(where, "p.organization_id = "+b.add(params.OrganizationID))
	}
	if params.Query != "" {
		pattern := "%" + escapeLike(params.Query) + "%"
		namePlaceholder := b.add(pattern)
		statusPlaceholder := b.add(pattern)
		organizationPlaceholder := b.add(pattern)
		networkPlaceholder := b.add(pattern)
		where = append(where, fmt.Sprintf(
			"(pd.name ILIKE %s ESCAPE '%s' OR p.status ILIKE %s ESCAPE '%s' OR o.name ILIKE %s ESCAPE '%s' OR n.name ILIKE %s ESCAPE '%s')",
			namePlaceholder, likeEscapeChar,
			statusPlaceholder, likeEscapeChar,
			organizationPlaceholder, likeEscapeChar,
			networkPlaceholder, likeEscapeChar,
		))
	}
	if hasStringFilter(params.Name, params.NameOp) {
		applyStringFilter(b, &where, "pd.name", params.Name, params.NameOp)
	}
	if params.Status != "" {
		where = append(where, "p.status = "+b.add(params.Status))
	}
	if hasStringFilter(params.Network, params.NetworkOp) {
		applyStringFilter(b, &where, "n.name", params.Network, params.NetworkOp)
	}
	if hasStringFilter(params.Organization, params.OrganizationOp) {
		applyStringFilter(b, &where, "o.name", params.Organization, params.OrganizationOp)
	}

	whereSQL := strings.Join(where, " AND ")
	fromSQL := `
		FROM public.pipelines p
		JOIN public.networks n ON n.id = p.network_id
		LEFT JOIN public.pipeline_definitions pd ON pd.id = p.pipeline_definition_id
		LEFT JOIN public.organizations o ON o.id = p.organization_id AND o.deleted_at IS NULL
		WHERE ` + whereSQL

	countSQL = "SELECT count(*)" + fromSQL
	countArgs = append([]any{}, b.args...)

	orderColumn := runListSortColumns[params.Sort]
	order := "ASC"
	if params.Order == "desc" {
		order = "DESC"
	}

	listSQL = "SELECT" + pipelineRunSelectColumns + fromSQL + fmt.Sprintf(" ORDER BY %s %s, p.id ASC", orderColumn, order)
	listArgs = append([]any{}, b.args...)
	if params.PageSize > 0 {
		offset := (params.Page - 1) * params.PageSize
		listSQL += " LIMIT " + b.add(params.PageSize) + " OFFSET " + b.add(offset)
		listArgs = append([]any{}, b.args...)
	}

	return countSQL, listSQL, countArgs, listArgs
}
