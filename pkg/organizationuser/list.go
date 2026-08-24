package organizationuser

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

	organizationUserNameExpr = `(ou.first_name || ' ' || ou.last_name)`
)

var listSortColumns = map[string]string{
	"name":         organizationUserNameExpr,
	"email":        "ou.email",
	"organization": "COALESCE(o.name, '')",
	"createdAt":    "ou.created_at",
	"updatedAt":    "ou.updated_at",
}

func parseListParams(r *http.Request) (listParams, error) {
	query := r.URL.Query()
	params := listParams{
		Query:          strings.TrimSpace(query.Get("q")),
		Sort:           strings.TrimSpace(query.Get("sort")),
		Order:          strings.ToLower(strings.TrimSpace(query.Get("order"))),
		Name:           strings.TrimSpace(query.Get("name")),
		NameOp:         strings.TrimSpace(query.Get("nameOp")),
		Email:          strings.TrimSpace(query.Get("email")),
		EmailOp:        strings.TrimSpace(query.Get("emailOp")),
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

	emailOp, err := normalizeStringOp(params.EmailOp)
	if err != nil {
		return listParams{}, err
	}
	params.EmailOp = emailOp

	organizationOp, err := normalizeStringOp(params.OrganizationOp)
	if err != nil {
		return listParams{}, err
	}
	params.OrganizationOp = organizationOp

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
	where := []string{"ou.deleted_at IS NULL"}

	if params.UserID != "" {
		where = append(where, "n.user_id = "+b.add(params.UserID))
	}
	if params.NetworkID != "" {
		where = append(where, "ou.network_id = "+b.add(params.NetworkID))
	}
	if params.OrganizationID != "" {
		where = append(where, "ou.organization_id = "+b.add(params.OrganizationID))
	}
	if params.Query != "" {
		pattern := "%" + escapeLike(params.Query) + "%"
		namePlaceholder := b.add(pattern)
		emailPlaceholder := b.add(pattern)
		organizationPlaceholder := b.add(pattern)
		where = append(where, fmt.Sprintf(
			"(%s ILIKE %s ESCAPE '%s' OR ou.email ILIKE %s ESCAPE '%s' OR o.name ILIKE %s ESCAPE '%s')",
			organizationUserNameExpr, namePlaceholder, likeEscapeChar,
			emailPlaceholder, likeEscapeChar,
			organizationPlaceholder, likeEscapeChar,
		))
	}
	if hasStringFilter(params.Name, params.NameOp) {
		applyStringFilter(b, &where, organizationUserNameExpr, params.Name, params.NameOp)
	}
	if hasStringFilter(params.Email, params.EmailOp) {
		applyStringFilter(b, &where, "ou.email", params.Email, params.EmailOp)
	}
	if hasStringFilter(params.Organization, params.OrganizationOp) {
		applyStringFilter(b, &where, "o.name", params.Organization, params.OrganizationOp)
	}

	whereSQL := strings.Join(where, " AND ")
	fromSQL := `
		FROM public.organization_users ou
		JOIN public.networks n ON n.id = ou.network_id AND n.deleted_at IS NULL
		LEFT JOIN public.organizations o ON o.id = ou.organization_id AND o.deleted_at IS NULL
		WHERE ` + whereSQL

	countSQL = "SELECT count(*)" + fromSQL
	countArgs = append([]any{}, b.args...)

	orderColumn := listSortColumns[params.Sort]
	order := "ASC"
	if params.Order == "desc" {
		order = "DESC"
	}

	listSQL = "SELECT" + organizationUserSelectColumns + fromSQL + fmt.Sprintf(" ORDER BY %s %s, ou.id ASC", orderColumn, order)
	listArgs = append([]any{}, b.args...)
	if params.PageSize > 0 {
		offset := (params.Page - 1) * params.PageSize
		listSQL += " LIMIT " + b.add(params.PageSize) + " OFFSET " + b.add(offset)
		listArgs = append([]any{}, b.args...)
	}

	return countSQL, listSQL, countArgs, listArgs
}
