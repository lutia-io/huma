package schema

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseListParams_defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/schema", nil)
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
	r := httptest.NewRequest(http.MethodGet, "/schema?page=2&pageSize=10&q=invoice&sort=slug&order=desc&scope=network&active=true&name=Inv&nameOp=startsWith&slug=inv&properties=3&propertiesOp=gte", nil)
	params, err := parseListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.Page != 2 || params.PageSize != 10 {
		t.Fatalf("got page=%d pageSize=%d", params.Page, params.PageSize)
	}
	if params.Query != "invoice" || params.Sort != "slug" || params.Order != "desc" {
		t.Fatalf("got q=%s sort=%s order=%s", params.Query, params.Sort, params.Order)
	}
	if params.Scope != "network" || params.Active == nil || !*params.Active {
		t.Fatalf("got scope=%s active=%v", params.Scope, params.Active)
	}
	if params.Name != "Inv" || params.NameOp != opStartsWith {
		t.Fatalf("got name=%s nameOp=%s", params.Name, params.NameOp)
	}
	if params.Properties == nil || *params.Properties != 3 || params.PropertiesOp != opGte {
		t.Fatalf("got properties=%v op=%s", params.Properties, params.PropertiesOp)
	}
}

func TestParseListParams_rejectsInvalidSort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/schema?sort=definition", nil)
	if _, err := parseListParams(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildListQuery_searchAndPagination(t *testing.T) {
	params := listParams{
		UserID:   "user-1",
		Query:    "inv_oice",
		Sort:     "name",
		Order:    "asc",
		Page:     2,
		PageSize: 20,
	}
	countSQL, listSQL, countArgs, listArgs := buildListQuery(params)
	if !strings.Contains(countSQL, "s.user_id = $1") {
		t.Fatalf("count SQL missing user filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "s.name ILIKE") || !strings.Contains(countSQL, "ESCAPE '!'") {
		t.Fatalf("count SQL missing search: %s", countSQL)
	}
	if got, want := countArgs[1], "%inv!_oice%"; got != want {
		t.Fatalf("escaped like pattern = %v want %v", got, want)
	}
	if !strings.Contains(listSQL, "ORDER BY s.name ASC") {
		t.Fatalf("list SQL missing order: %s", listSQL)
	}
	if !strings.Contains(listSQL, "LIMIT") || !strings.Contains(listSQL, "OFFSET") {
		t.Fatalf("list SQL missing pagination: %s", listSQL)
	}
	if len(listArgs) != len(countArgs)+2 {
		t.Fatalf("list args=%d count args=%d", len(listArgs), len(countArgs))
	}
	if listArgs[len(listArgs)-2] != 20 || listArgs[len(listArgs)-1] != 20 {
		t.Fatalf("limit/offset args = %v", listArgs[len(listArgs)-2:])
	}
}

func TestBuildListQuery_networkAndOrganization(t *testing.T) {
	params := listParams{
		UserID:         "user-1",
		NetworkID:      "net-1",
		OrganizationID: "org-1",
		Sort:           "name",
		Order:          "asc",
	}
	countSQL, _, _, _ := buildListQuery(params)
	if !strings.Contains(countSQL, "s.network_id = $2") {
		t.Fatalf("missing network filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "(s.organization_id IS NULL OR s.organization_id = $3)") {
		t.Fatalf("missing organization visibility filter: %s", countSQL)
	}
	if strings.Contains(countSQL, "Visible") {
		t.Fatalf("unexpected visible field in SQL: %s", countSQL)
	}
}

func TestBuildListQuery_scopeAndProperties(t *testing.T) {
	count := 4
	params := listParams{
		UserID:       "user-1",
		Scope:        "organization",
		Properties:   &count,
		PropertiesOp: opGte,
		Sort:         "properties",
		Order:        "desc",
	}
	countSQL, listSQL, _, _ := buildListQuery(params)
	if !strings.Contains(countSQL, "s.organization_id IS NOT NULL") {
		t.Fatalf("missing scope filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, propertyCountExpr) {
		t.Fatalf("missing properties filter: %s", countSQL)
	}
	if !strings.Contains(listSQL, "ORDER BY ("+propertyCountExpr+") DESC") {
		t.Fatalf("missing properties sort: %s", listSQL)
	}
}
