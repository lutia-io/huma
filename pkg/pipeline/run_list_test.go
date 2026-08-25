package pipeline

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseRunListParams_defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/pipeline", nil)
	params, err := parseRunListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.Sort != "createdAt" || params.Order != "desc" {
		t.Fatalf("got sort=%s order=%s", params.Sort, params.Order)
	}
}

func TestParseRunListParams_filters(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/pipeline?page=2&pageSize=10&sort=currentLevel&order=asc&status=running&name=Sync", nil)
	params, err := parseRunListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.Page != 2 || params.PageSize != 10 || params.Sort != "currentLevel" || params.Status != "running" {
		t.Fatalf("got %+v", params)
	}
}

func TestParseRunListParams_rejectsInvalidStatus(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/pipeline?status=queued", nil)
	if _, err := parseRunListParams(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildRunListQuery_search(t *testing.T) {
	params := runListParams{
		UserID:   "user-1",
		Query:    "sync",
		Sort:     "name",
		Order:    "asc",
		Page:     1,
		PageSize: 20,
	}
	countSQL, listSQL, _, _ := buildRunListQuery(params)
	if !strings.Contains(countSQL, "n.user_id = $1") {
		t.Fatalf("missing user filter: %s", countSQL)
	}
	if !strings.Contains(listSQL, "ORDER BY COALESCE(pd.name, '') ASC") {
		t.Fatalf("missing order: %s", listSQL)
	}
}
