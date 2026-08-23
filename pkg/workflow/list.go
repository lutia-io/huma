package workflow

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

	actionCountExpr = `CASE WHEN jsonb_typeof(wd.definition->'actions') = 'array' THEN jsonb_array_length(wd.definition->'actions') ELSE 0 END`
)

var listSortColumns = map[string]string{
	"name":      "wd.name",
	"slug":      "wd.slug",
	"status":    "wd.active",
	"active":    "wd.active",
	"schema":    "s.name",
	"network":   "COALESCE(n.name, '')",
	"actions":   "(" + actionCountExpr + ")",
	"createdAt": "wd.created_at",
	"updatedAt": "wd.updated_at",
}

func parseListParams(r *http.Request) (listParams, error) {
	query := r.URL.Query()
	params := listParams{
		Query:          strings.TrimSpace(query.Get("q")),
		Sort:           strings.TrimSpace(query.Get("sort")),
		Order:          strings.ToLower(strings.TrimSpace(query.Get("order"))),
		Name:           strings.TrimSpace(query.Get("name")),
		NameOp:         strings.TrimSpace(query.Get("nameOp")),
		Slug:           strings.TrimSpace(query.Get("slug")),
		SlugOp:         strings.TrimSpace(query.Get("slugOp")),
		Schema:         strings.TrimSpace(query.Get("schema")),
		SchemaOp:       strings.TrimSpace(query.Get("schemaOp")),
		Network:        strings.TrimSpace(query.Get("network")),
		NetworkOp:      strings.TrimSpace(query.Get("networkOp")),
		ActionsOp:      strings.TrimSpace(query.Get("actionsOp")),
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

	schemaOp, err := normalizeStringOp(params.SchemaOp)
	if err != nil {
		return listParams{}, err
	}
	params.SchemaOp = schemaOp

	networkOp, err := normalizeStringOp(params.NetworkOp)
	if err != nil {
		return listParams{}, err
	}
	params.NetworkOp = networkOp

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

	if value := strings.TrimSpace(query.Get("actions")); value != "" {
		count, convErr := strconv.Atoi(value)
		if convErr != nil || count < 0 {
			return listParams{}, apperror.NewBadRequestError("Invalid actions filter", nil)
		}
		params.Actions = &count
		op := params.ActionsOp
		if op == "" {
			op = opEq
		}
		if op != opEq && op != opGte && op != opLte {
			return listParams{}, apperror.NewBadRequestError("Invalid actions filter operator", nil)
		}
		params.ActionsOp = op
	}

	if params.NetworkID != "" && !uuid.Valid(params.NetworkID) {
		return listParams{}, apperror.NewBadRequestError("Invalid network ID", nil)
	}
	if params.OrganizationID != "" && !uuid.Valid(params.OrganizationID) {
		return listParams{}, apperror.NewBadRequestError("Invalid organization ID", nil)
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
	where := []string{"wd.deleted_at IS NULL", "s.deleted_at IS NULL"}

	if params.UserID != "" {
		where = append(where, "wd.user_id = "+b.add(params.UserID))
	}
	if params.NetworkID != "" {
		where = append(where, "wd.network_id = "+b.add(params.NetworkID))
	}
	if params.OrganizationID != "" {
		where = append(where, fmt.Sprintf("(s.organization_id IS NULL OR s.organization_id = %s)", b.add(params.OrganizationID)))
	}
	if params.Active != nil {
		where = append(where, "wd.active = "+b.add(*params.Active))
	}
	if params.Query != "" {
		pattern := "%" + escapeLike(params.Query) + "%"
		namePlaceholder := b.add(pattern)
		slugPlaceholder := b.add(pattern)
		schemaPlaceholder := b.add(pattern)
		networkPlaceholder := b.add(pattern)
		where = append(where, fmt.Sprintf(
			"(wd.name ILIKE %s ESCAPE '%s' OR wd.slug ILIKE %s ESCAPE '%s' OR s.name ILIKE %s ESCAPE '%s' OR n.name ILIKE %s ESCAPE '%s')",
			namePlaceholder, likeEscapeChar,
			slugPlaceholder, likeEscapeChar,
			schemaPlaceholder, likeEscapeChar,
			networkPlaceholder, likeEscapeChar,
		))
	}
	if params.Name != "" {
		applyStringFilter(b, &where, "wd.name", params.Name, params.NameOp)
	}
	if params.Slug != "" {
		applyStringFilter(b, &where, "wd.slug", params.Slug, params.SlugOp)
	}
	if params.Schema != "" {
		applyStringFilter(b, &where, "s.name", params.Schema, params.SchemaOp)
	}
	if params.Network != "" {
		applyStringFilter(b, &where, "n.name", params.Network, params.NetworkOp)
	}
	if params.Actions != nil {
		operator := "="
		switch params.ActionsOp {
		case opGte:
			operator = ">="
		case opLte:
			operator = "<="
		}
		where = append(where, fmt.Sprintf("(%s) %s %s", actionCountExpr, operator, b.add(*params.Actions)))
	}

	whereSQL := strings.Join(where, " AND ")
	fromSQL := `
		FROM public.workflow_definitions wd
		JOIN public.schemas s ON s.id = wd.schema_id
		LEFT JOIN public.networks n ON n.id = wd.network_id AND n.deleted_at IS NULL
		WHERE ` + whereSQL

	countSQL = "SELECT count(*)" + fromSQL
	countArgs = append([]any{}, b.args...)

	orderColumn := listSortColumns[params.Sort]
	order := "ASC"
	if params.Order == "desc" {
		order = "DESC"
	}

	listSQL = "SELECT" + workflowDefinitionListSelectColumns + fromSQL + fmt.Sprintf(" ORDER BY %s %s, wd.id ASC", orderColumn, order)
	listArgs = append([]any{}, b.args...)
	if params.PageSize > 0 {
		offset := (params.Page - 1) * params.PageSize
		listSQL += " LIMIT " + b.add(params.PageSize) + " OFFSET " + b.add(offset)
		listArgs = append([]any{}, b.args...)
	}

	return countSQL, listSQL, countArgs, listArgs
}
