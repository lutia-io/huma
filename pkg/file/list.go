package file

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

	fileKindExpr = `CASE WHEN f.content_type LIKE 'image/%' THEN 'Image' WHEN f.content_type = 'application/pdf' THEN 'PDF' WHEN f.content_type = 'text/csv' OR f.filename ILIKE '%.csv' THEN 'CSV' WHEN f.content_type LIKE '%spreadsheet%' OR f.filename ILIKE '%.xlsx' OR f.filename ILIKE '%.xls' THEN 'Spreadsheet' ELSE 'Other' END`
)

var listSortColumns = map[string]string{
	"filename":     "f.filename",
	"contentType":  fileKindExpr,
	"sizeBytes":    "f.size_bytes",
	"organization": "COALESCE(o.name, '')",
	"createdAt":    "f.created_at",
	"updatedAt":    "f.updated_at",
}

var fileContentTypes = map[string]struct{}{
	"Image":       {},
	"PDF":         {},
	"CSV":         {},
	"Spreadsheet": {},
	"Other":       {},
}

func parseListParams(r *http.Request) (listParams, error) {
	query := r.URL.Query()
	params := listParams{
		Query:          strings.TrimSpace(query.Get("q")),
		Sort:           strings.TrimSpace(query.Get("sort")),
		Order:          strings.ToLower(strings.TrimSpace(query.Get("order"))),
		Filename:       strings.TrimSpace(query.Get("filename")),
		FilenameOp:     strings.TrimSpace(query.Get("filenameOp")),
		ContentType:    strings.TrimSpace(query.Get("contentType")),
		SizeBytesOp:    strings.TrimSpace(query.Get("sizeBytesOp")),
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

	filenameOp, err := normalizeStringOp(params.FilenameOp)
	if err != nil {
		return listParams{}, err
	}
	params.FilenameOp = filenameOp

	organizationOp, err := normalizeStringOp(params.OrganizationOp)
	if err != nil {
		return listParams{}, err
	}
	params.OrganizationOp = organizationOp

	if params.ContentType != "" {
		if _, ok := fileContentTypes[params.ContentType]; !ok {
			return listParams{}, apperror.NewBadRequestError("Invalid type filter", nil)
		}
	}

	if params.SizeBytesOp == opEmpty {
		// value is optional for empty
	} else if value := strings.TrimSpace(query.Get("sizeBytes")); value != "" {
		size, convErr := strconv.ParseInt(value, 10, 64)
		if convErr != nil || size < 0 {
			return listParams{}, apperror.NewBadRequestError("Invalid size filter", nil)
		}
		params.SizeBytes = &size
		op := params.SizeBytesOp
		if op == "" {
			op = opEq
		}
		if op != opEq && op != opGte && op != opLte {
			return listParams{}, apperror.NewBadRequestError("Invalid size filter operator", nil)
		}
		params.SizeBytesOp = op
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
	where := []string{"f.deleted_at IS NULL"}

	if params.UserID != "" {
		where = append(where, "n.user_id = "+b.add(params.UserID))
	}
	if params.NetworkID != "" {
		where = append(where, "f.network_id = "+b.add(params.NetworkID))
	}
	if params.OrganizationID != "" {
		where = append(where, "f.organization_id = "+b.add(params.OrganizationID))
	}
	if params.Query != "" {
		pattern := "%" + escapeLike(params.Query) + "%"
		filenamePlaceholder := b.add(pattern)
		contentTypePlaceholder := b.add(pattern)
		kindPlaceholder := b.add(pattern)
		organizationPlaceholder := b.add(pattern)
		where = append(where, fmt.Sprintf(
			"(f.filename ILIKE %s ESCAPE '%s' OR f.content_type ILIKE %s ESCAPE '%s' OR (%s) ILIKE %s ESCAPE '%s' OR o.name ILIKE %s ESCAPE '%s')",
			filenamePlaceholder, likeEscapeChar,
			contentTypePlaceholder, likeEscapeChar,
			fileKindExpr, kindPlaceholder, likeEscapeChar,
			organizationPlaceholder, likeEscapeChar,
		))
	}
	if hasStringFilter(params.Filename, params.FilenameOp) {
		applyStringFilter(b, &where, "f.filename", params.Filename, params.FilenameOp)
	}
	if params.ContentType != "" {
		where = append(where, "("+fileKindExpr+") = "+b.add(params.ContentType))
	}
	if params.SizeBytesOp == opEmpty {
		where = append(where, "(f.size_bytes IS NULL OR f.size_bytes = 0)")
	} else if params.SizeBytes != nil {
		operator := "="
		switch params.SizeBytesOp {
		case opGte:
			operator = ">="
		case opLte:
			operator = "<="
		}
		where = append(where, fmt.Sprintf("f.size_bytes %s %s", operator, b.add(*params.SizeBytes)))
	}
	if hasStringFilter(params.Organization, params.OrganizationOp) {
		applyStringFilter(b, &where, "o.name", params.Organization, params.OrganizationOp)
	}

	whereSQL := strings.Join(where, " AND ")
	fromSQL := `
		FROM public.files f
		JOIN public.networks n ON n.id = f.network_id AND n.deleted_at IS NULL
		LEFT JOIN public.organizations o ON o.id = f.organization_id AND o.deleted_at IS NULL
		WHERE ` + whereSQL

	countSQL = "SELECT count(*)" + fromSQL
	countArgs = append([]any{}, b.args...)

	orderColumn := listSortColumns[params.Sort]
	order := "ASC"
	if params.Order == "desc" {
		order = "DESC"
	}

	listSQL = "SELECT" + fileSelectColumns + fromSQL + fmt.Sprintf(" ORDER BY %s %s, f.id ASC", orderColumn, order)
	listArgs = append([]any{}, b.args...)
	if params.PageSize > 0 {
		offset := (params.Page - 1) * params.PageSize
		listSQL += " LIMIT " + b.add(params.PageSize) + " OFFSET " + b.add(offset)
		listArgs = append([]any{}, b.args...)
	}

	return countSQL, listSQL, countArgs, listArgs
}
