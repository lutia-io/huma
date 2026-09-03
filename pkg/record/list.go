package record

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
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

	fieldKindString  = "string"
	fieldKindNumber  = "number"
	fieldKindBoolean = "boolean"
	fieldKindFile    = "file"
	fieldKindForeign = "foreign"
	fieldKindAddress = "address"

	fieldPrefix   = "field."
	fieldOpPrefix = "fieldOp."
)

var (
	reservedSortColumns = map[string]string{
		"organization": "COALESCE(o.name, '')",
		"createdAt":    "r.created_at",
		"updatedAt":    "r.updated_at",
	}
	fieldNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func parseListParams(r *http.Request) (listParams, error) {
	query := r.URL.Query()
	params := listParams{
		Query:          strings.TrimSpace(query.Get("q")),
		Sort:           strings.TrimSpace(query.Get("sort")),
		Order:          strings.ToLower(strings.TrimSpace(query.Get("order"))),
		SchemaID:       strings.TrimSpace(query.Get("schemaId")),
		NetworkID:      strings.TrimSpace(query.Get("networkId")),
		OrganizationID: strings.TrimSpace(query.Get("organizationId")),
		Organization:   strings.TrimSpace(query.Get("organization")),
		OrganizationOp: strings.TrimSpace(query.Get("organizationOp")),
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
	if !validSortField(params.Sort) {
		return listParams{}, apperror.NewBadRequestError("Invalid sort field", nil)
	}
	if params.Order != "asc" && params.Order != "desc" {
		return listParams{}, apperror.NewBadRequestError("Invalid sort order", nil)
	}

	organizationOp, err := normalizeStringOp(params.OrganizationOp)
	if err != nil {
		return listParams{}, err
	}
	params.OrganizationOp = organizationOp

	if params.SchemaID != "" && !uuid.Valid(params.SchemaID) {
		return listParams{}, apperror.NewBadRequestError("Invalid schema ID", nil)
	}
	if params.NetworkID != "" && !uuid.Valid(params.NetworkID) {
		return listParams{}, apperror.NewBadRequestError("Invalid network ID", nil)
	}
	if params.OrganizationID != "" && !uuid.Valid(params.OrganizationID) {
		return listParams{}, apperror.NewBadRequestError("Invalid organization ID", nil)
	}

	fields, err := parseFieldFilters(query)
	if err != nil {
		return listParams{}, err
	}
	params.Fields = fields

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

func parseFieldFilters(query url.Values) ([]fieldFilter, error) {
	names := make([]string, 0)
	seen := make(map[string]struct{})
	addName := func(name string) error {
		if name == "" {
			return nil
		}
		if !validFieldName(name) {
			return apperror.NewBadRequestError("Invalid filter field", nil)
		}
		if _, ok := seen[name]; ok {
			return nil
		}
		seen[name] = struct{}{}
		names = append(names, name)
		return nil
	}

	for key := range query {
		switch {
		case strings.HasPrefix(key, fieldOpPrefix):
			if err := addName(key[len(fieldOpPrefix):]); err != nil {
				return nil, err
			}
		case strings.HasPrefix(key, fieldPrefix):
			if err := addName(key[len(fieldPrefix):]); err != nil {
				return nil, err
			}
		}
	}

	filters := make([]fieldFilter, 0)
	for _, name := range names {
		value := strings.TrimSpace(query.Get(fieldPrefix + name))
		op := strings.TrimSpace(query.Get(fieldOpPrefix + name))
		if op != "" && !validFilterOp(op) {
			return nil, apperror.NewBadRequestError("Invalid field filter operator", nil)
		}
		if value == "" && op != opEmpty {
			continue
		}
		filters = append(filters, fieldFilter{
			Name:  name,
			Value: value,
			Op:    op,
		})
	}
	sort.Slice(filters, func(i, j int) bool {
		return filters[i].Name < filters[j].Name
	})
	return filters, nil
}

func validSortField(name string) bool {
	if _, ok := reservedSortColumns[name]; ok {
		return true
	}
	return validFieldName(name)
}

func validFieldName(name string) bool {
	return fieldNamePattern.MatchString(name)
}

func validFilterOp(op string) bool {
	switch op {
	case opContains, opEq, opStartsWith, opEmpty, opGte, opLte:
		return true
	default:
		return false
	}
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

func normalizeFieldOp(kind, op string) (string, error) {
	if op == opEmpty {
		return opEmpty, nil
	}
	switch kind {
	case fieldKindNumber:
		if op == "" {
			return opEq, nil
		}
		if op == opEq || op == opGte || op == opLte {
			return op, nil
		}
		return "", apperror.NewBadRequestError("Invalid number filter operator", nil)
	case fieldKindBoolean:
		if op == "" {
			return opEq, nil
		}
		if op == opEq {
			return op, nil
		}
		return "", apperror.NewBadRequestError("Invalid boolean filter operator", nil)
	default:
		if op == "" {
			return opContains, nil
		}
		if op == opContains || op == opEq || op == opStartsWith {
			return op, nil
		}
		return "", apperror.NewBadRequestError("Invalid string filter operator", nil)
	}
}

func parseBooleanFilter(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "1":
		return true, nil
	case "false", "no", "0":
		return false, nil
	default:
		return false, apperror.NewBadRequestError("Invalid boolean filter", nil)
	}
}

func parseSchemaFields(definition json.RawMessage) (map[string]schemaField, error) {
	var doc struct {
		Properties map[string]struct {
			Type     json.RawMessage `json:"type"`
			Format   string          `json:"format"`
			SchemaID string          `json:"schemaId"`
		} `json:"properties"`
	}
	if len(definition) == 0 {
		return map[string]schemaField{}, nil
	}
	if err := json.Unmarshal(definition, &doc); err != nil {
		return nil, err
	}
	fields := make(map[string]schemaField, len(doc.Properties))
	for name, prop := range doc.Properties {
		if !validFieldName(name) {
			continue
		}
		fields[name] = schemaField{
			Name:     name,
			Kind:     fieldKindFromProp(prop.Type, prop.Format),
			SchemaID: strings.TrimSpace(prop.SchemaID),
		}
	}
	return fields, nil
}

func fieldKindFromProp(typeJSON json.RawMessage, format string) string {
	if strings.EqualFold(format, "file") {
		return fieldKindFile
	}
	if strings.EqualFold(format, "foreign") {
		return fieldKindForeign
	}
	if strings.EqualFold(format, "address") {
		return fieldKindAddress
	}
	typeName := jsonSchemaTypeName(typeJSON)
	switch typeName {
	case "number", "integer":
		return fieldKindNumber
	case "boolean":
		return fieldKindBoolean
	default:
		return fieldKindString
	}
}

func jsonSchemaTypeName(typeJSON json.RawMessage) string {
	if len(typeJSON) == 0 {
		return "string"
	}
	var asString string
	if err := json.Unmarshal(typeJSON, &asString); err == nil {
		return asString
	}
	var asList []string
	if err := json.Unmarshal(typeJSON, &asList); err == nil && len(asList) > 0 {
		return asList[0]
	}
	return "string"
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

func applyJSONEmptyFilter(where *[]string, key string) {
	*where = append(*where, fmt.Sprintf(
		`((r.data -> %s) IS NULL OR jsonb_typeof(r.data -> %s) = 'null' OR r.data ->> %s = '' OR (jsonb_typeof(r.data -> %s) = 'array' AND jsonb_array_length(r.data -> %s) = 0) OR r.data -> %s = '{}'::jsonb)`,
		key, key, key, key, key, key,
	))
}

func applyFieldFilter(b *queryBuilder, where *[]string, field fieldFilter) {
	key := b.add(field.Name)
	if field.Op == opEmpty {
		applyJSONEmptyFilter(where, key)
		return
	}
	switch field.Kind {
	case fieldKindNumber:
		operator := "="
		switch field.Op {
		case opGte:
			operator = ">="
		case opLte:
			operator = "<="
		}
		value := any(field.Value)
		if field.NumberValue != nil {
			value = *field.NumberValue
		}
		*where = append(*where, fmt.Sprintf(
			"(jsonb_typeof(r.data -> %s) = 'number' AND (r.data ->> %s)::numeric %s %s)",
			key, key, operator, b.add(value),
		))
	case fieldKindBoolean:
		literal := "false"
		if field.BooleanValue != nil && *field.BooleanValue {
			literal = "true"
		}
		*where = append(*where, fmt.Sprintf("r.data -> %s = '%s'::jsonb", key, literal))
	case fieldKindFile:
		applyStringFilter(b, where, fmt.Sprintf(
			"(SELECT f.filename FROM public.files f WHERE f.id::text = r.data ->> %s AND f.deleted_at IS NULL)",
			key,
		), field.Value, field.Op)
	case fieldKindForeign:
		applyStringFilter(b, where, relatedTitleExpr(b, key, field.TitleKey), field.Value, field.Op)
	case fieldKindAddress:
		applyStringFilter(b, where, addressTextExpr(key), field.Value, field.Op)
	default:
		applyStringFilter(b, where, fmt.Sprintf("r.data ->> %s", key), field.Value, field.Op)
	}
}

func sortExpression(params listParams, b *queryBuilder) string {
	if expr, ok := reservedSortColumns[params.Sort]; ok {
		return expr
	}
	key := b.add(params.Sort)
	meta, ok := params.SchemaFields[params.Sort]
	if !ok {
		return fmt.Sprintf("r.data ->> %s", key)
	}
	switch meta.Kind {
	case fieldKindNumber:
		return fmt.Sprintf("(CASE WHEN jsonb_typeof(r.data -> %s) = 'number' THEN (r.data ->> %s)::numeric END)", key, key)
	case fieldKindBoolean:
		return fmt.Sprintf("(CASE WHEN jsonb_typeof(r.data -> %s) = 'boolean' THEN (r.data ->> %s)::boolean END)", key, key)
	case fieldKindFile:
		return fmt.Sprintf("(SELECT f.filename FROM public.files f WHERE f.id::text = r.data ->> %s AND f.deleted_at IS NULL)", key)
	case fieldKindForeign:
		return relatedTitleExpr(b, key, meta.TitleKey)
	case fieldKindAddress:
		return addressTextExpr(key)
	default:
		return fmt.Sprintf("r.data ->> %s", key)
	}
}

func addressTextExpr(key string) string {
	return fmt.Sprintf(
		`concat_ws(' ', r.data -> %s ->> 'line1', r.data -> %s ->> 'line2', r.data -> %s ->> 'city', r.data -> %s ->> 'region', r.data -> %s ->> 'postalCode', r.data -> %s ->> 'country')`,
		key, key, key, key, key, key,
	)
}

func relatedTitleExpr(b *queryBuilder, fieldKeyPlaceholder, titleKey string) string {
	idMatch := fmt.Sprintf("related.id::text = r.data ->> %s AND related.deleted_at IS NULL", fieldKeyPlaceholder)
	if titleKey == "" {
		return fmt.Sprintf("(SELECT related.id::text FROM public.records related WHERE %s)", idMatch)
	}
	titlePlaceholder := b.add(titleKey)
	return fmt.Sprintf(
		"(SELECT COALESCE(NULLIF(related.data ->> %s, ''), related.id::text) FROM public.records related WHERE %s)",
		titlePlaceholder, idMatch,
	)
}

func buildListQuery(params listParams) (countSQL, listSQL string, countArgs, listArgs []any) {
	b := &queryBuilder{}
	where := []string{"r.deleted_at IS NULL"}

	if params.UserID != "" {
		where = append(where, "n.user_id = "+b.add(params.UserID))
	}
	if params.SchemaID != "" {
		where = append(where, "r.schema_id = "+b.add(params.SchemaID))
	}
	if params.NetworkID != "" {
		where = append(where, "r.network_id = "+b.add(params.NetworkID))
	}
	if params.OrganizationID != "" {
		where = append(where, "r.organization_id = "+b.add(params.OrganizationID))
	}
	if params.Query != "" {
		pattern := "%" + escapeLike(params.Query) + "%"
		idPlaceholder := b.add(pattern)
		orgPlaceholder := b.add(pattern)
		dataPlaceholder := b.add(pattern)
		filePlaceholder := b.add(pattern)
		where = append(where, fmt.Sprintf(
			`(r.id::text ILIKE %s ESCAPE '%s' OR o.name ILIKE %s ESCAPE '%s' OR r.data::text ILIKE %s ESCAPE '%s' OR EXISTS (
				SELECT 1
				FROM jsonb_each_text(r.data) kv
				JOIN public.files f ON f.id::text = kv.value AND f.deleted_at IS NULL
				WHERE f.filename ILIKE %s ESCAPE '%s'
			))`,
			idPlaceholder, likeEscapeChar,
			orgPlaceholder, likeEscapeChar,
			dataPlaceholder, likeEscapeChar,
			filePlaceholder, likeEscapeChar,
		))
	}
	if hasStringFilter(params.Organization, params.OrganizationOp) {
		applyStringFilter(b, &where, "o.name", params.Organization, params.OrganizationOp)
	}
	for _, field := range params.Fields {
		applyFieldFilter(b, &where, field)
	}

	whereSQL := strings.Join(where, " AND ")
	fromSQL := `
		FROM public.records r
		JOIN public.networks n ON n.id = r.network_id AND n.deleted_at IS NULL
		LEFT JOIN public.organizations o ON o.id = r.organization_id AND o.deleted_at IS NULL
		WHERE ` + whereSQL

	countSQL = "SELECT count(*)" + fromSQL
	countArgs = append([]any{}, b.args...)

	orderColumn := sortExpression(params, b)
	order := "ASC"
	if params.Order == "desc" {
		order = "DESC"
	}

	listSQL = "SELECT" + recordSelectColumns + fromSQL + fmt.Sprintf(" ORDER BY %s %s NULLS LAST, r.id ASC", orderColumn, order)
	listArgs = append([]any{}, b.args...)
	if params.PageSize > 0 {
		offset := (params.Page - 1) * params.PageSize
		listSQL += " LIMIT " + b.add(params.PageSize) + " OFFSET " + b.add(offset)
		listArgs = append([]any{}, b.args...)
	}

	return countSQL, listSQL, countArgs, listArgs
}
