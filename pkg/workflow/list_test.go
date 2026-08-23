package workflow

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseListParams_defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/workflow-definition", nil)
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
	r := httptest.NewRequest(http.MethodGet, "/workflow-definition?page=2&pageSize=10&q=intake&sort=slug&order=desc&active=true&name=Int&nameOp=startsWith&slug=int&schema=Shipment&schemaOp=contains&network=DHL&actions=3&actionsOp=gte", nil)
	params, err := parseListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.Page != 2 || params.PageSize != 10 {
		t.Fatalf("got page=%d pageSize=%d", params.Page, params.PageSize)
	}
	if params.Query != "intake" || params.Sort != "slug" || params.Order != "desc" {
		t.Fatalf("got q=%s sort=%s order=%s", params.Query, params.Sort, params.Order)
	}
	if params.Active == nil || !*params.Active {
		t.Fatalf("got active=%v", params.Active)
	}
	if params.Name != "Int" || params.NameOp != opStartsWith {
		t.Fatalf("got name=%s nameOp=%s", params.Name, params.NameOp)
	}
	if params.Schema != "Shipment" || params.SchemaOp != opContains {
		t.Fatalf("got schema=%s schemaOp=%s", params.Schema, params.SchemaOp)
	}
	if params.Network != "DHL" || params.NetworkOp != opContains {
		t.Fatalf("got network=%s networkOp=%s", params.Network, params.NetworkOp)
	}
	if params.Actions == nil || *params.Actions != 3 || params.ActionsOp != opGte {
		t.Fatalf("got actions=%v op=%s", params.Actions, params.ActionsOp)
	}
}

func TestParseListParams_rejectsInvalidSort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/workflow-definition?sort=definition", nil)
	if _, err := parseListParams(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildListQuery_searchAndPagination(t *testing.T) {
	params := listParams{
		UserID:   "user-1",
		Query:    "int_ake",
		Sort:     "name",
		Order:    "asc",
		Page:     2,
		PageSize: 20,
	}
	countSQL, listSQL, countArgs, listArgs := buildListQuery(params)
	if !strings.Contains(countSQL, "wd.user_id = $1") {
		t.Fatalf("count SQL missing user filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "wd.name ILIKE") || !strings.Contains(countSQL, "ESCAPE '!'") {
		t.Fatalf("count SQL missing search: %s", countSQL)
	}
	if got, want := countArgs[1], "%int!_ake%"; got != want {
		t.Fatalf("escaped like pattern = %v want %v", got, want)
	}
	if !strings.Contains(listSQL, "ORDER BY wd.name ASC") {
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
	if !strings.Contains(countSQL, "wd.network_id = $2") {
		t.Fatalf("missing network filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "(s.organization_id IS NULL OR s.organization_id = $3)") {
		t.Fatalf("missing organization visibility filter: %s", countSQL)
	}
}

func TestBuildListQuery_schemaNetworkAndActions(t *testing.T) {
	count := 4
	params := listParams{
		UserID:    "user-1",
		Schema:    "Ship",
		SchemaOp:  opStartsWith,
		Network:   "DHL",
		NetworkOp: opContains,
		Actions:   &count,
		ActionsOp: opGte,
		Sort:      "actions",
		Order:     "desc",
	}
	countSQL, listSQL, _, _ := buildListQuery(params)
	if !strings.Contains(countSQL, "s.name ILIKE") {
		t.Fatalf("missing schema filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "n.name ILIKE") {
		t.Fatalf("missing network name filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, actionCountExpr) {
		t.Fatalf("missing actions filter: %s", countSQL)
	}
	if !strings.Contains(listSQL, "ORDER BY ("+actionCountExpr+") DESC") {
		t.Fatalf("missing actions sort: %s", listSQL)
	}
}
