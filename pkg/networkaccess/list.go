package networkaccess

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
	defaultPageSize = 50
	maxPageSize     = 100
	likeEscapeChar  = "!"
)

func parseListParams(r *http.Request) (listParams, error) {
	query := r.URL.Query()
	params := listParams{
		NetworkID:                strings.TrimSpace(query.Get("networkId")),
		Query:                    strings.TrimSpace(query.Get("q")),
		ExcludeAssignedToGroupID: strings.TrimSpace(query.Get("excludeAssignedToGroupId")),
		Page:                     defaultPage,
		PageSize:                 defaultPageSize,
	}

	if params.ExcludeAssignedToGroupID != "" && !uuid.Valid(params.ExcludeAssignedToGroupID) {
		return listParams{}, apperror.NewBadRequestError("Invalid excludeAssignedToGroupId", nil)
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

	return params, nil
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

func buildGroupListQuery(params listParams) (countSQL, listSQL string, countArgs, listArgs []any) {
	b := &queryBuilder{}
	where := []string{
		"g.network_id = " + b.add(params.NetworkID),
		"g.deleted_at IS NULL",
	}
	if params.Query != "" {
		pattern := "%" + escapeLike(params.Query) + "%"
		namePlaceholder := b.add(pattern)
		descPlaceholder := b.add(pattern)
		where = append(where, fmt.Sprintf(
			"(g.name ILIKE %s ESCAPE '%s' OR g.description ILIKE %s ESCAPE '%s')",
			namePlaceholder, likeEscapeChar,
			descPlaceholder, likeEscapeChar,
		))
	}

	fromSQL := `
		FROM public.network_groups g
		WHERE ` + strings.Join(where, " AND ")

	countSQL = "SELECT count(*)" + fromSQL
	countArgs = append([]any{}, b.args...)

	offset := (params.Page - 1) * params.PageSize
	listSQL = "SELECT" + groupSelect + fromSQL +
		" ORDER BY g.system DESC, g.name ASC, g.id ASC" +
		" LIMIT " + b.add(params.PageSize) + " OFFSET " + b.add(offset)
	listArgs = append([]any{}, b.args...)
	return countSQL, listSQL, countArgs, listArgs
}

func buildPermissionListQuery(params listParams) (countSQL, listSQL string, countArgs, listArgs []any) {
	b := &queryBuilder{}
	where := []string{
		"p.network_id = " + b.add(params.NetworkID),
		"p.deleted_at IS NULL",
	}
	if params.Query != "" {
		pattern := "%" + escapeLike(params.Query) + "%"
		namePlaceholder := b.add(pattern)
		descPlaceholder := b.add(pattern)
		where = append(where, fmt.Sprintf(
			"(p.name ILIKE %s ESCAPE '%s' OR p.description ILIKE %s ESCAPE '%s')",
			namePlaceholder, likeEscapeChar,
			descPlaceholder, likeEscapeChar,
		))
	}
	if params.ExcludeAssignedToGroupID != "" {
		where = append(where, "p.id NOT IN (SELECT permission_id FROM public.network_group_permissions WHERE group_id = "+b.add(params.ExcludeAssignedToGroupID)+")")
	}

	fromSQL := `
		FROM public.network_permissions p
		WHERE ` + strings.Join(where, " AND ")

	countSQL = "SELECT count(*)" + fromSQL
	countArgs = append([]any{}, b.args...)

	offset := (params.Page - 1) * params.PageSize
	listSQL = `
		SELECT p.id, p.network_id, p.name, p.slug, p.description, p.system, p.created_at, p.updated_at` + fromSQL +
		" ORDER BY p.system DESC, p.name ASC, p.id ASC" +
		" LIMIT " + b.add(params.PageSize) + " OFFSET " + b.add(offset)
	listArgs = append([]any{}, b.args...)
	return countSQL, listSQL, countArgs, listArgs
}
