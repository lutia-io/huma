package node

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseListParams_defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/node-definition", nil)
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
}

func TestParseListParams_paginationAndFilters(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/node-definition?page=2&pageSize=10&q=http&sort=type&order=asc&active=true&name=Fetch&nameOp=startsWith&type=HTTP&typeOp=eq", nil)
	params, err := parseListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.Page != 2 || params.PageSize != 10 {
		t.Fatalf("got page=%d pageSize=%d", params.Page, params.PageSize)
	}
	if params.Query != "http" || params.Sort != "type" || params.Order != "asc" {
		t.Fatalf("got q=%s sort=%s order=%s", params.Query, params.Sort, params.Order)
	}
	if params.Type != "HTTP" || params.TypeOp != opEq {
		t.Fatalf("got type=%s typeOp=%s", params.Type, params.TypeOp)
	}
}

func TestBuildListQuery_searchAndPagination(t *testing.T) {
	params := listParams{
		UserID:   "user-1",
		Query:    "fetch_users",
		Sort:     "name",
		Order:    "asc",
		Page:     2,
		PageSize: 20,
	}
	countSQL, listSQL, countArgs, listArgs := buildListQuery(params)
	if !strings.Contains(countSQL, "nd.user_id = $1") {
		t.Fatalf("count SQL missing user filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "nd.name ILIKE") {
		t.Fatalf("count SQL missing search: %s", countSQL)
	}
	if got, want := countArgs[1], "%fetch!_users%"; got != want {
		t.Fatalf("escaped like pattern = %v want %v", got, want)
	}
	if !strings.Contains(listSQL, "ORDER BY nd.name ASC") {
		t.Fatalf("list SQL missing order: %s", listSQL)
	}
	if len(listArgs) != len(countArgs)+2 {
		t.Fatalf("list args=%d count args=%d", len(listArgs), len(countArgs))
	}
}
