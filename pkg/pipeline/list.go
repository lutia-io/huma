package pipeline

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/uuid"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
	likeEscapeChar  = "!"

	opContains   = "contains"
	opEq         = "eq"
	opStartsWith = "startsWith"
	opGte        = "gte"
	opLte        = "lte"

	stageCountExpr = `CASE WHEN jsonb_typeof(pd.definition->'nodes') = 'array' THEN jsonb_array_length(pd.definition->'nodes') WHEN jsonb_typeof(pd.definition->'stages') = 'array' THEN jsonb_array_length(pd.definition->'stages') ELSE 0 END`
	sourceExpr     = `COALESCE(NULLIF(pd.definition#>>'{source,name}', ''), NULLIF(pd.definition#>>'{source,type}', ''), NULLIF(pd.definition->>'source', ''), '')`
)

var listSortColumns = map[string]string{
	"name":      "pd.name",
	"slug":      "pd.slug",
	"status":    "pd.active",
	"active":    "pd.active",
	"network":   "COALESCE(n.name, '')",
	"source":    "(" + sourceExpr + ")",
	"stages":    "(" + stageCountExpr + ")",
	"createdAt": "pd.created_at",
	"updatedAt": "pd.updated_at",
}

func parseListParams(r *http.Request) (listParams, error) {
	query := r.URL.Query()
	params := listParams{
		Query:     strings.TrimSpace(query.Get("q")),
		Sort:      strings.TrimSpace(query.Get("sort")),
		Order:     strings.ToLower(strings.TrimSpace(query.Get("order"))),
		Name:      strings.TrimSpace(query.Get("name")),
		NameOp:    strings.TrimSpace(query.Get("nameOp")),
		Slug:      strings.TrimSpace(query.Get("slug")),
		SlugOp:    strings.TrimSpace(query.Get("slugOp")),
		Network:   strings.TrimSpace(query.Get("network")),
		NetworkOp: strings.TrimSpace(query.Get("networkOp")),
		Source:    strings.TrimSpace(query.Get("source")),
		SourceOp:  strings.TrimSpace(query.Get("sourceOp")),
		StagesOp:  strings.TrimSpace(query.Get("stagesOp")),
		NetworkID: strings.TrimSpace(query.Get("networkId")),
		Page:      defaultPage,
	}

	if params.Sort == "" {
		params.Sort = "createdAt"
		if params.Order == "" {
			params.Order = "desc"
		}
	} else if params.Order == "" {
		params.Order = "asc"
	}
	if _, ok := listSortColumns[params.Sort]; !ok {
		return listParams{}, apperror.NewBadRequestError("Invalid sort field", nil)
	}
	if params.Order != "asc" && params.Order != "desc" {
		return listParams{}, apperror.NewBadRequestError("Invalid sort order", nil)
	}

	nameOp, err := normalizeStringOp(params.NameOp)
	if err != nil {
		return listParams{}, err
	}
	params.NameOp = nameOp

	slugOp, err := normalizeStringOp(params.SlugOp)
	if err != nil {
		return listParams{}, err
	}
	params.SlugOp = slugOp

	networkOp, err := normalizeStringOp(params.NetworkOp)
	if err != nil {
		return listParams{}, err
	}
	params.NetworkOp = networkOp

	sourceOp, err := normalizeStringOp(params.SourceOp)
	if err != nil {
		return listParams{}, err
	}
	params.SourceOp = sourceOp

	if value := strings.TrimSpace(query.Get("active")); value != "" {
		switch value {
		case "true":
			active := true
			params.Active = &active
		case "false":
			active := false
			params.Active = &active
		default:
			return listParams{}, apperror.NewBadRequestError("Invalid active filter", nil)
		}
	}

	if value := strings.TrimSpace(query.Get("stages")); value != "" {
		count, convErr := strconv.Atoi(value)
		if convErr != nil || count < 0 {
			return listParams{}, apperror.NewBadRequestError("Invalid stages filter", nil)
		}
		params.Stages = &count
		op := params.StagesOp
		if op == "" {
			op = opEq
		}
		if op != opEq && op != opGte && op != opLte {
			return listParams{}, apperror.NewBadRequestError("Invalid stages filter operator", nil)
		}
		params.StagesOp = op
	}

	if params.NetworkID != "" && !uuid.Valid(params.NetworkID) {
		return listParams{}, apperror.NewBadRequestError("Invalid network ID", nil)
	}

	if value := strings.TrimSpace(query.Get("page")); value != "" {
		page, convErr := strconv.Atoi(value)
		if convErr != nil || page < 1 {
			return listParams{}, apperror.NewBadRequestError("Invalid page", nil)
		}
		params.Page = page
	}

	if value := strings.TrimSpace(query.Get("pageSize")); value != "" {
		pageSize, convErr := strconv.Atoi(value)
		if convErr != nil || pageSize < 1 {
			return listParams{}, apperror.NewBadRequestError("Invalid page size", nil)
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

func normalizeStringOp(op string) (string, error) {
	if op == "" {
		return opContains, nil
	}
	switch op {
	case opContains, opEq, opStartsWith:
		return op, nil
	default:
		return "", apperror.NewBadRequestError("Invalid string filter operator", nil)
	}
}

type queryBuilder struct {
	args []any
}

func (b *queryBuilder) add(value any) string {
	b.args = append(b.args, value)
	return fmt.Sprintf("$%d", len(b.args))
}

func escapeLike(value string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(value)
}

func applyStringFilter(b *queryBuilder, where *[]string, column, value, op string) {
	pattern := escapeLike(value)
	switch op {
	case opEq:
	case opStartsWith:
		pattern += "%"
	default:
		pattern = "%" + pattern + "%"
	}
	*where = append(*where, fmt.Sprintf("%s ILIKE %s ESCAPE '%s'", column, b.add(pattern), likeEscapeChar))
}

func buildListQuery(params listParams) (countSQL, listSQL string, countArgs, listArgs []any) {
	b := &queryBuilder{}
	where := []string{"pd.deleted_at IS NULL", "n.deleted_at IS NULL"}

	if params.UserID != "" {
		where = append(where, "pd.user_id = "+b.add(params.UserID))
	}
	if params.NetworkID != "" {
		where = append(where, "pd.network_id = "+b.add(params.NetworkID))
	}
	if params.Active != nil {
		where = append(where, "pd.active = "+b.add(*params.Active))
	}
	if params.Query != "" {
		pattern := "%" + escapeLike(params.Query) + "%"
		namePlaceholder := b.add(pattern)
		slugPlaceholder := b.add(pattern)
		sourcePlaceholder := b.add(pattern)
		networkPlaceholder := b.add(pattern)
		where = append(where, fmt.Sprintf(
			"(pd.name ILIKE %s ESCAPE '%s' OR pd.slug ILIKE %s ESCAPE '%s' OR (%s) ILIKE %s ESCAPE '%s' OR n.name ILIKE %s ESCAPE '%s')",
			namePlaceholder, likeEscapeChar,
			slugPlaceholder, likeEscapeChar,
			sourceExpr, sourcePlaceholder, likeEscapeChar,
			networkPlaceholder, likeEscapeChar,
		))
	}
	if params.Name != "" {
		applyStringFilter(b, &where, "pd.name", params.Name, params.NameOp)
	}
	if params.Slug != "" {
		applyStringFilter(b, &where, "pd.slug", params.Slug, params.SlugOp)
	}
	if params.Network != "" {
		applyStringFilter(b, &where, "n.name", params.Network, params.NetworkOp)
	}
	if params.Source != "" {
		applyStringFilter(b, &where, "("+sourceExpr+")", params.Source, params.SourceOp)
	}
	if params.Stages != nil {
		operator := "="
		switch params.StagesOp {
		case opGte:
			operator = ">="
		case opLte:
			operator = "<="
		}
		where = append(where, fmt.Sprintf("(%s) %s %s", stageCountExpr, operator, b.add(*params.Stages)))
	}

	whereSQL := strings.Join(where, " AND ")
	fromSQL := `
		FROM public.pipeline_definitions pd
		JOIN public.networks n ON n.id = pd.network_id
		WHERE ` + whereSQL

	countSQL = "SELECT count(*)" + fromSQL
	countArgs = append([]any{}, b.args...)

	orderColumn := listSortColumns[params.Sort]
	order := "ASC"
	if params.Order == "desc" {
		order = "DESC"
	}

	listSQL = "SELECT" + pipelineListSelectColumns + fromSQL + fmt.Sprintf(" ORDER BY %s %s, pd.id ASC", orderColumn, order)
	listArgs = append([]any{}, b.args...)
	if params.PageSize > 0 {
		offset := (params.Page - 1) * params.PageSize
		listSQL += " LIMIT " + b.add(params.PageSize) + " OFFSET " + b.add(offset)
		listArgs = append([]any{}, b.args...)
	}

	return countSQL, listSQL, countArgs, listArgs
}
