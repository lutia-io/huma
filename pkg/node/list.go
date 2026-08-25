package node

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
)

var listSortColumns = map[string]string{
	"name":      "nd.name",
	"slug":      "nd.slug",
	"status":    "nd.active",
	"active":    "nd.active",
	"type":      "nd.type",
	"network":   "COALESCE(n.name, '')",
	"createdAt": "nd.created_at",
	"updatedAt": "nd.updated_at",
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
		Type:      strings.TrimSpace(query.Get("type")),
		TypeOp:    strings.TrimSpace(query.Get("typeOp")),
		Network:   strings.TrimSpace(query.Get("network")),
		NetworkOp: strings.TrimSpace(query.Get("networkOp")),
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

	typeOp, err := normalizeStringOp(params.TypeOp)
	if err != nil {
		return listParams{}, err
	}
	params.TypeOp = typeOp

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
	where := []string{"nd.deleted_at IS NULL", "n.deleted_at IS NULL"}

	if params.UserID != "" {
		where = append(where, "nd.user_id = "+b.add(params.UserID))
	}
	if params.NetworkID != "" {
		where = append(where, "nd.network_id = "+b.add(params.NetworkID))
	}
	if params.Active != nil {
		where = append(where, "nd.active = "+b.add(*params.Active))
	}
	if params.Query != "" {
		pattern := "%" + escapeLike(params.Query) + "%"
		namePlaceholder := b.add(pattern)
		slugPlaceholder := b.add(pattern)
		typePlaceholder := b.add(pattern)
		networkPlaceholder := b.add(pattern)
		where = append(where, fmt.Sprintf(
			"(nd.name ILIKE %s ESCAPE '%s' OR nd.slug ILIKE %s ESCAPE '%s' OR nd.type ILIKE %s ESCAPE '%s' OR n.name ILIKE %s ESCAPE '%s')",
			namePlaceholder, likeEscapeChar,
			slugPlaceholder, likeEscapeChar,
			typePlaceholder, likeEscapeChar,
			networkPlaceholder, likeEscapeChar,
		))
	}
	if hasStringFilter(params.Name, params.NameOp) {
		applyStringFilter(b, &where, "nd.name", params.Name, params.NameOp)
	}
	if hasStringFilter(params.Slug, params.SlugOp) {
		applyStringFilter(b, &where, "nd.slug", params.Slug, params.SlugOp)
	}
	if hasStringFilter(params.Type, params.TypeOp) {
		applyStringFilter(b, &where, "nd.type", params.Type, params.TypeOp)
	}
	if hasStringFilter(params.Network, params.NetworkOp) {
		applyStringFilter(b, &where, "n.name", params.Network, params.NetworkOp)
	}

	whereSQL := strings.Join(where, " AND ")
	fromSQL := `
		FROM public.node_definitions nd
		JOIN public.networks n ON n.id = nd.network_id
		WHERE ` + whereSQL

	countSQL = "SELECT count(*)" + fromSQL
	countArgs = append([]any{}, b.args...)

	orderColumn := listSortColumns[params.Sort]
	order := "ASC"
	if params.Order == "desc" {
		order = "DESC"
	}

	listSQL = "SELECT" + nodeListSelectColumns + fromSQL + fmt.Sprintf(" ORDER BY %s %s, nd.id ASC", orderColumn, order)
	listArgs = append([]any{}, b.args...)
	if params.PageSize > 0 {
		offset := (params.Page - 1) * params.PageSize
		listSQL += " LIMIT " + b.add(params.PageSize) + " OFFSET " + b.add(offset)
		listArgs = append([]any{}, b.args...)
	}

	return countSQL, listSQL, countArgs, listArgs
}
