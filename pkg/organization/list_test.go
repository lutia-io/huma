package organization

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseListParams_defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/organization", nil)
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
	r := httptest.NewRequest(http.MethodGet, "/organization?page=2&pageSize=10&q=dhl&sort=slug&order=desc&name=DHL&nameOp=startsWith&slug=dhl&network=Global&networkOp=contains", nil)
	params, err := parseListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.Page != 2 || params.PageSize != 10 {
		t.Fatalf("got page=%d pageSize=%d", params.Page, params.PageSize)
	}
	if params.Query != "dhl" || params.Sort != "slug" || params.Order != "desc" {
		t.Fatalf("got q=%s sort=%s order=%s", params.Query, params.Sort, params.Order)
	}
	if params.Name != "DHL" || params.NameOp != opStartsWith {
		t.Fatalf("got name=%s nameOp=%s", params.Name, params.NameOp)
	}
	if params.Network != "Global" || params.NetworkOp != opContains {
		t.Fatalf("got network=%s networkOp=%s", params.Network, params.NetworkOp)
	}
}

func TestParseListParams_rejectsInvalidSort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/organization?sort=color", nil)
	if _, err := parseListParams(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildListQuery_searchAndPagination(t *testing.T) {
	params := listParams{
		UserID:   "user-1",
		Query:    "dhl_apac",
		Sort:     "name",
		Order:    "asc",
		Page:     2,
		PageSize: 20,
	}
	countSQL, listSQL, countArgs, listArgs := buildListQuery(params)
	if !strings.Contains(countSQL, "o.user_id = $1") {
		t.Fatalf("count SQL missing user filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "o.name ILIKE") || !strings.Contains(countSQL, "ESCAPE '!'") {
		t.Fatalf("count SQL missing search: %s", countSQL)
	}
	if got, want := countArgs[1], "%dhl!_apac%"; got != want {
		t.Fatalf("escaped like pattern = %v want %v", got, want)
	}
	if !strings.Contains(listSQL, "ORDER BY o.name ASC") {
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
	if !strings.Contains(countSQL, "o.network_id = $2") {
		t.Fatalf("missing network filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "o.id = $3") {
		t.Fatalf("missing organization filter: %s", countSQL)
	}
}

func TestBuildListQuery_networkName(t *testing.T) {
	params := listParams{
		UserID:    "user-1",
		Network:   "DHL",
		NetworkOp: opContains,
		Sort:      "network",
		Order:     "desc",
	}
	countSQL, listSQL, _, _ := buildListQuery(params)
	if !strings.Contains(countSQL, "n.name ILIKE") {
		t.Fatalf("missing network name filter: %s", countSQL)
	}
	if !strings.Contains(listSQL, "ORDER BY COALESCE(n.name, '') DESC") {
		t.Fatalf("missing network sort: %s", listSQL)
	}
}

func TestParseListParams_emptyFilter(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/organization?nameOp=empty&slugOp=empty", nil)
	params, err := parseListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.NameOp != opEmpty || params.SlugOp != opEmpty {
		t.Fatalf("got nameOp=%s slugOp=%s", params.NameOp, params.SlugOp)
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
	if !strings.Contains(countSQL, "(o.name IS NULL OR BTRIM((o.name)::text) = '')") {
		t.Fatalf("missing empty name filter: %s", countSQL)
	}
}
