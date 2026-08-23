package organizationuser

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseListParams_defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/organization-user", nil)
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
	if params.NameOp != opContains || params.EmailOp != opContains {
		t.Fatalf("got nameOp=%s emailOp=%s", params.NameOp, params.EmailOp)
	}
}

func TestParseListParams_paginationAndFilters(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/organization-user?page=2&pageSize=10&q=ada&sort=email&order=desc&name=Ada&nameOp=startsWith&email=ada@&emailOp=contains&organization=DHL&organizationOp=contains", nil)
	params, err := parseListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.Page != 2 || params.PageSize != 10 {
		t.Fatalf("got page=%d pageSize=%d", params.Page, params.PageSize)
	}
	if params.Query != "ada" || params.Sort != "email" || params.Order != "desc" {
		t.Fatalf("got q=%s sort=%s order=%s", params.Query, params.Sort, params.Order)
	}
	if params.Name != "Ada" || params.NameOp != opStartsWith {
		t.Fatalf("got name=%s nameOp=%s", params.Name, params.NameOp)
	}
	if params.Email != "ada@" || params.EmailOp != opContains {
		t.Fatalf("got email=%s emailOp=%s", params.Email, params.EmailOp)
	}
	if params.Organization != "DHL" || params.OrganizationOp != opContains {
		t.Fatalf("got organization=%s organizationOp=%s", params.Organization, params.OrganizationOp)
	}
}

func TestParseListParams_rejectsInvalidSort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/organization-user?sort=password", nil)
	if _, err := parseListParams(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildListQuery_searchAndPagination(t *testing.T) {
	params := listParams{
		UserID:   "user-1",
		Query:    "ada_lovelace",
		Sort:     "name",
		Order:    "asc",
		Page:     2,
		PageSize: 20,
	}
	countSQL, listSQL, countArgs, listArgs := buildListQuery(params)
	if !strings.Contains(countSQL, "n.user_id = $1") {
		t.Fatalf("count SQL missing user filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, organizationUserNameExpr) || !strings.Contains(countSQL, "ESCAPE '!'") {
		t.Fatalf("count SQL missing search: %s", countSQL)
	}
	if got, want := countArgs[1], "%ada!_lovelace%"; got != want {
		t.Fatalf("escaped like pattern = %v want %v", got, want)
	}
	if !strings.Contains(listSQL, "ORDER BY "+organizationUserNameExpr+" ASC") {
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
	if !strings.Contains(countSQL, "ou.network_id = $2") {
		t.Fatalf("missing network filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "ou.organization_id = $3") {
		t.Fatalf("missing organization filter: %s", countSQL)
	}
}

func TestBuildListQuery_emailAndOrganizationName(t *testing.T) {
	params := listParams{
		UserID:         "user-1",
		Email:          "ada@",
		EmailOp:        opStartsWith,
		Organization:   "DHL",
		OrganizationOp: opContains,
		Sort:           "organization",
		Order:          "desc",
	}
	countSQL, listSQL, _, _ := buildListQuery(params)
	if !strings.Contains(countSQL, "ou.email ILIKE") {
		t.Fatalf("missing email filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "o.name ILIKE") {
		t.Fatalf("missing organization name filter: %s", countSQL)
	}
	if !strings.Contains(listSQL, "ORDER BY COALESCE(o.name, '') DESC") {
		t.Fatalf("missing organization sort: %s", listSQL)
	}
}
