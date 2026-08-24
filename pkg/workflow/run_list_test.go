package workflow

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseRunListParams_defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/workflow", nil)
	params, err := parseRunListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.Sort != "createdAt" || params.Order != "desc" {
		t.Fatalf("got sort=%s order=%s", params.Sort, params.Order)
	}
	if params.Page != 1 || params.PageSize != 0 {
		t.Fatalf("got page=%d pageSize=%d", params.Page, params.PageSize)
	}
	if params.NameOp != opContains || params.OrganizationOp != opContains {
		t.Fatalf("got nameOp=%s organizationOp=%s", params.NameOp, params.OrganizationOp)
	}
}

func TestParseRunListParams_paginationAndFilters(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/workflow?page=2&pageSize=10&q=intake&sort=name&order=asc&name=Int&nameOp=startsWith&status=running&organization=DHL&organizationOp=contains", nil)
	params, err := parseRunListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.Page != 2 || params.PageSize != 10 {
		t.Fatalf("got page=%d pageSize=%d", params.Page, params.PageSize)
	}
	if params.Query != "intake" || params.Sort != "name" || params.Order != "asc" {
		t.Fatalf("got q=%s sort=%s order=%s", params.Query, params.Sort, params.Order)
	}
	if params.Name != "Int" || params.NameOp != opStartsWith {
		t.Fatalf("got name=%s nameOp=%s", params.Name, params.NameOp)
	}
	if params.Status != "running" {
		t.Fatalf("got status=%s", params.Status)
	}
	if params.Organization != "DHL" || params.OrganizationOp != opContains {
		t.Fatalf("got organization=%s organizationOp=%s", params.Organization, params.OrganizationOp)
	}
}

func TestParseRunListParams_rejectsInvalidSort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/workflow?sort=blob", nil)
	if _, err := parseRunListParams(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRunListParams_rejectsInvalidStatus(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/workflow?status=paused", nil)
	if _, err := parseRunListParams(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildRunListQuery_searchAndPagination(t *testing.T) {
	params := runListParams{
		UserID:   "user-1",
		Query:    "int_ake",
		Sort:     "name",
		Order:    "asc",
		Page:     2,
		PageSize: 20,
	}
	countSQL, listSQL, countArgs, listArgs := buildRunListQuery(params)
	if !strings.Contains(countSQL, "n.user_id = $1") {
		t.Fatalf("count SQL missing user filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "wd.name ILIKE") || !strings.Contains(countSQL, "ESCAPE '!'") {
		t.Fatalf("count SQL missing search: %s", countSQL)
	}
	if got, want := countArgs[1], "%int!_ake%"; got != want {
		t.Fatalf("escaped like pattern = %v want %v", got, want)
	}
	if !strings.Contains(listSQL, "ORDER BY COALESCE(wd.name, '') ASC") {
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

func TestBuildRunListQuery_networkOrganizationAndStatus(t *testing.T) {
	params := runListParams{
		UserID:         "user-1",
		NetworkID:      "net-1",
		OrganizationID: "org-1",
		Status:         "failed",
		Organization:   "DHL",
		OrganizationOp: opContains,
		Sort:           "status",
		Order:          "desc",
	}
	countSQL, listSQL, _, _ := buildRunListQuery(params)
	if !strings.Contains(countSQL, "w.network_id = $2") {
		t.Fatalf("missing network filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "w.organization_id = $3") {
		t.Fatalf("missing organization filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "w.status = $4") {
		t.Fatalf("missing status filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "o.name ILIKE") {
		t.Fatalf("missing organization name filter: %s", countSQL)
	}
	if !strings.Contains(listSQL, "ORDER BY w.status DESC") {
		t.Fatalf("missing status sort: %s", listSQL)
	}
}
