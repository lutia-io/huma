package pipeline

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseListParams_defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/pipeline-definition", nil)
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
	r := httptest.NewRequest(http.MethodGet, "/pipeline-definition?page=2&pageSize=10&q=manifest&sort=slug&order=desc&active=true&name=Man&nameOp=startsWith&slug=man&source=API&sourceOp=contains&stages=3&stagesOp=gte", nil)
	params, err := parseListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.Page != 2 || params.PageSize != 10 {
		t.Fatalf("got page=%d pageSize=%d", params.Page, params.PageSize)
	}
	if params.Query != "manifest" || params.Sort != "slug" || params.Order != "desc" {
		t.Fatalf("got q=%s sort=%s order=%s", params.Query, params.Sort, params.Order)
	}
	if params.Active == nil || !*params.Active {
		t.Fatalf("got active=%v", params.Active)
	}
	if params.Name != "Man" || params.NameOp != opStartsWith {
		t.Fatalf("got name=%s nameOp=%s", params.Name, params.NameOp)
	}
	if params.Source != "API" || params.SourceOp != opContains {
		t.Fatalf("got source=%s sourceOp=%s", params.Source, params.SourceOp)
	}
	if params.Stages == nil || *params.Stages != 3 || params.StagesOp != opGte {
		t.Fatalf("got stages=%v op=%s", params.Stages, params.StagesOp)
	}
}

func TestParseListParams_rejectsInvalidSort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/pipeline-definition?sort=definition", nil)
	if _, err := parseListParams(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildListQuery_searchAndPagination(t *testing.T) {
	params := listParams{
		UserID:   "user-1",
		Query:    "man_ifest",
		Sort:     "name",
		Order:    "asc",
		Page:     2,
		PageSize: 20,
	}
	countSQL, listSQL, countArgs, listArgs := buildListQuery(params)
	if !strings.Contains(countSQL, "pd.user_id = $1") {
		t.Fatalf("count SQL missing user filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "pd.name ILIKE") || !strings.Contains(countSQL, "ESCAPE '!'") {
		t.Fatalf("count SQL missing search: %s", countSQL)
	}
	if got, want := countArgs[1], "%man!_ifest%"; got != want {
		t.Fatalf("escaped like pattern = %v want %v", got, want)
	}
	if !strings.Contains(listSQL, "ORDER BY pd.name ASC") {
		t.Fatalf("list SQL missing order: %s", listSQL)
	}
	if !strings.Contains(listSQL, "LIMIT") || !strings.Contains(listSQL, "OFFSET") {
		t.Fatalf("list SQL missing pagination: %s", listSQL)
	}
	if len(listArgs) != len(countArgs)+2 {
		t.Fatalf("list args=%d count args=%d", len(listArgs), len(countArgs))
	}
}

func TestBuildListQuery_networkSourceAndStages(t *testing.T) {
	count := 4
	params := listParams{
		UserID:    "user-1",
		NetworkID: "net-1",
		Source:    "API",
		SourceOp:  opContains,
		Stages:    &count,
		StagesOp:  opGte,
		Sort:      "stages",
		Order:     "desc",
	}
	countSQL, listSQL, _, _ := buildListQuery(params)
	if !strings.Contains(countSQL, "pd.network_id = $2") {
		t.Fatalf("missing network filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, sourceExpr) {
		t.Fatalf("missing source filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, stageCountExpr) {
		t.Fatalf("missing stages filter: %s", countSQL)
	}
	if !strings.Contains(listSQL, "ORDER BY ("+stageCountExpr+") DESC") {
		t.Fatalf("missing stages sort: %s", listSQL)
	}
}
