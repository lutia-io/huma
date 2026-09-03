package schema

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
	opEmpty      = "empty"
	opGte        = "gte"
	opLte        = "lte"

	propertyCountExpr = `CASE WHEN json_typeof(s.definition->'properties') = 'object' THEN (SELECT count(*) FROM json_object_keys(s.definition->'properties')) ELSE 0 END`
)

var listSortColumns = map[string]string{
	"name":       "s.name",
	"slug":       "s.slug",
	"status":     "s.active",
	"active":     "s.active",
	"scope":      "COALESCE(o.name, 'Network')",
	"properties": "(" + propertyCountExpr + ")",
	"createdAt":  "s.created_at",
	"updatedAt":  "s.updated_at",
}

func parseListParams(r *http.Request) (listParams, error) {
	query := r.URL.Query()
	params := listParams{
		Query:          strings.TrimSpace(query.Get("q")),
		Sort:           strings.TrimSpace(query.Get("sort")),
		Order:          strings.ToLower(strings.TrimSpace(query.Get("order"))),
		Scope:          strings.TrimSpace(query.Get("scope")),
		Name:           strings.TrimSpace(query.Get("name")),
		NameOp:         strings.TrimSpace(query.Get("nameOp")),
		Slug:           strings.TrimSpace(query.Get("slug")),
		SlugOp:         strings.TrimSpace(query.Get("slugOp")),
		PropertiesOp:   strings.TrimSpace(query.Get("propertiesOp")),
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

	if params.Scope != "" && params.Scope != "network" && params.Scope != "organization" {
		return listParams{}, apperror.NewBadRequestError("Invalid scope", nil)
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

	if params.PropertiesOp == opEmpty {
		// value is optional for empty
	} else if value := strings.TrimSpace(query.Get("properties")); value != "" {
		count, convErr := strconv.Atoi(value)
		if convErr != nil || count < 0 {
			return listParams{}, apperror.NewBadRequestError("Invalid properties filter", nil)
		}
		params.Properties = &count
		op := params.PropertiesOp
		if op == "" {
			op = opEq
		}
		if op != opEq && op != opGte && op != opLte {
			return listParams{}, apperror.NewBadRequestError("Invalid properties filter operator", nil)
		}
		params.PropertiesOp = op
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
	case opContains, opEq, opStartsWith, opEmpty:
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
	if op == opEmpty {
		*where = append(*where, fmt.Sprintf("(%s IS NULL OR BTRIM((%s)::text) = '')", column, column))
		return
	}
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

func hasStringFilter(value, op string) bool {
	return op == opEmpty || value != ""
}

func buildListQuery(params listParams) (countSQL, listSQL string, countArgs, listArgs []any) {
	b := &queryBuilder{}
	where := []string{"s.deleted_at IS NULL"}

	if params.UserID != "" {
		where = append(where, "s.user_id = "+b.add(params.UserID))
	}
	if params.NetworkID != "" {
		where = append(where, "s.network_id = "+b.add(params.NetworkID))
	}
	if params.OrganizationID != "" {
		where = append(where, fmt.Sprintf("(s.organization_id IS NULL OR s.organization_id = %s)", b.add(params.OrganizationID)))
	}
	if params.Scope == "network" {
		where = append(where, "s.organization_id IS NULL")
	}
	if params.Scope == "organization" {
		where = append(where, "s.organization_id IS NOT NULL")
	}
	if params.Active != nil {
		where = append(where, "s.active = "+b.add(*params.Active))
	}
	if params.Query != "" {
		pattern := "%" + escapeLike(params.Query) + "%"
		namePlaceholder := b.add(pattern)
		slugPlaceholder := b.add(pattern)
		where = append(where, fmt.Sprintf("(s.name ILIKE %s ESCAPE '%s' OR s.slug ILIKE %s ESCAPE '%s')", namePlaceholder, likeEscapeChar, slugPlaceholder, likeEscapeChar))
	}
	if hasStringFilter(params.Name, params.NameOp) {
		applyStringFilter(b, &where, "s.name", params.Name, params.NameOp)
	}
	if hasStringFilter(params.Slug, params.SlugOp) {
		applyStringFilter(b, &where, "s.slug", params.Slug, params.SlugOp)
	}
	if params.PropertiesOp == opEmpty {
		where = append(where, "("+propertyCountExpr+") = 0")
	} else if params.Properties != nil {
		operator := "="
		switch params.PropertiesOp {
		case opGte:
			operator = ">="
		case opLte:
			operator = "<="
		}
		where = append(where, fmt.Sprintf("(%s) %s %s", propertyCountExpr, operator, b.add(*params.Properties)))
	}

	whereSQL := strings.Join(where, " AND ")
	fromSQL := `
		FROM public.schemas s
		LEFT JOIN public.organizations o ON o.id = s.organization_id AND o.deleted_at IS NULL
		WHERE ` + whereSQL

	countSQL = "SELECT count(*)" + fromSQL
	countArgs = append([]any{}, b.args...)

	orderColumn := listSortColumns[params.Sort]
	order := "ASC"
	if params.Order == "desc" {
		order = "DESC"
	}

	listSQL = "SELECT" + schemaListSelectColumns + fromSQL + fmt.Sprintf(" ORDER BY %s %s, s.id ASC", orderColumn, order)
	listArgs = append([]any{}, b.args...)
	if params.PageSize > 0 {
		offset := (params.Page - 1) * params.PageSize
		listSQL += " LIMIT " + b.add(params.PageSize) + " OFFSET " + b.add(offset)
		listArgs = append([]any{}, b.args...)
	}

	return countSQL, listSQL, countArgs, listArgs
}
