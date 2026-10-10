package network

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseListParams_defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/network", nil)
	params, err := parseListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.Sort != "createdAt" || params.Order != "desc" {
		t.Fatalf("got sort=%s order=%s", params.Sort, params.Order)
	}
	if params.Page != 1 || params.PageSize != 0 {
		t.Fatalf("got page=%d pageSize=%d", params.Page, params.PageSize)
	}
	if params.NameOp != opContains || params.SlugOp != opContains {
		t.Fatalf("got nameOp=%s slugOp=%s", params.NameOp, params.SlugOp)
	}
}

func TestParseListParams_paginationAndFilters(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/network?page=2&pageSize=12&q=dhl&sort=slug&order=desc&name=DHL&nameOp=startsWith&slug=dhl&slugOp=contains", nil)
	params, err := parseListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.Page != 2 || params.PageSize != 12 {
		t.Fatalf("got page=%d pageSize=%d", params.Page, params.PageSize)
	}
	if params.Query != "dhl" || params.Sort != "slug" || params.Order != "desc" {
		t.Fatalf("got q=%s sort=%s order=%s", params.Query, params.Sort, params.Order)
	}
	if params.Name != "DHL" || params.NameOp != opStartsWith {
		t.Fatalf("got name=%s nameOp=%s", params.Name, params.NameOp)
	}
	if params.Slug != "dhl" || params.SlugOp != opContains {
		t.Fatalf("got slug=%s slugOp=%s", params.Slug, params.SlugOp)
	}
}

func TestParseListParams_rejectsInvalidSort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/network?sort=color", nil)
	if _, err := parseListParams(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseListParams_emptyFilter(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/network?nameOp=empty&slugOp=empty", nil)
	params, err := parseListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.NameOp != opEmpty || params.SlugOp != opEmpty {
		t.Fatalf("got nameOp=%s slugOp=%s", params.NameOp, params.SlugOp)
	}
}

func TestBuildListQuery_searchAndPagination(t *testing.T) {
	params := listParams{
		UserID:   "user-1",
		Query:    "dhl_apac",
		Sort:     "name",
		Order:    "asc",
		Page:     2,
		PageSize: 12,
	}
	countSQL, listSQL, countArgs, listArgs := buildListQuery(params)
	if !strings.Contains(countSQL, "network_group_members") {
		t.Fatalf("count SQL missing membership filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "n.name ILIKE") || !strings.Contains(countSQL, "n.slug ILIKE") || !strings.Contains(countSQL, "ESCAPE '!'") {
		t.Fatalf("count SQL missing search: %s", countSQL)
	}
	if got, want := countArgs[1], "%dhl!_apac%"; got != want {
		t.Fatalf("escaped like pattern = %v want %v", got, want)
	}
	if !strings.Contains(listSQL, "ORDER BY n.name ASC") {
		t.Fatalf("list SQL missing order: %s", listSQL)
	}
	if !strings.Contains(listSQL, "LIMIT") || !strings.Contains(listSQL, "OFFSET") {
		t.Fatalf("list SQL missing pagination: %s", listSQL)
	}
	if len(listArgs) != len(countArgs)+2 {
		t.Fatalf("list args=%d count args=%d", len(listArgs), len(countArgs))
	}
	if listArgs[len(listArgs)-2] != 12 || listArgs[len(listArgs)-1] != 12 {
		t.Fatalf("limit/offset args = %v", listArgs[len(listArgs)-2:])
	}
}

func TestBuildListQuery_unpagedOmitsLimit(t *testing.T) {
	params := listParams{
		UserID: "user-1",
		Sort:   "createdAt",
		Order:  "desc",
		Page:   1,
	}
	_, listSQL, _, _ := buildListQuery(params)
	if strings.Contains(listSQL, "LIMIT") || strings.Contains(listSQL, "OFFSET") {
		t.Fatalf("unpaged list SQL should omit limit: %s", listSQL)
	}
	if !strings.Contains(listSQL, "ORDER BY n.created_at DESC") {
		t.Fatalf("list SQL missing default order: %s", listSQL)
	}
}

func TestBuildListQuery_networkID(t *testing.T) {
	params := listParams{
		NetworkID: "net-1",
		Sort:      "name",
		Order:     "asc",
	}
	countSQL, _, _, _ := buildListQuery(params)
	if !strings.Contains(countSQL, "n.id = $1") {
		t.Fatalf("missing network filter: %s", countSQL)
	}
	if strings.Contains(countSQL, "network_group_members") {
		t.Fatalf("organization user query should not add membership filter: %s", countSQL)
	}
}

func TestBuildListQuery_emptyName(t *testing.T) {
	params := listParams{
		UserID: "user-1",
		NameOp: opEmpty,
		Sort:   "name",
		Order:  "asc",
	}
	countSQL, _, _, _ := buildListQuery(params)
	if !strings.Contains(countSQL, "(n.name IS NULL OR BTRIM((n.name)::text) = '')") {
		t.Fatalf("missing empty name filter: %s", countSQL)
	}
}
