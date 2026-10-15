package networkaccess

import (
	"net/http"
	"net/url"
	"testing"
)

func TestParseListParamsDefaults(t *testing.T) {
	req := &http.Request{URL: &url.URL{RawQuery: "networkId=11111111-1111-1111-1111-111111111111"}}
	params, err := parseListParams(req)
	if err != nil {
		t.Fatal(err)
	}
	if params.Page != defaultPage || params.PageSize != defaultPageSize {
		t.Fatalf("unexpected defaults: page=%d pageSize=%d", params.Page, params.PageSize)
	}
}

func TestParseListParamsQueryAndExclude(t *testing.T) {
	req := &http.Request{URL: &url.URL{RawQuery: "q=edit&page=2&pageSize=10&excludeAssignedToGroupId=22222222-2222-2222-2222-222222222222"}}
	params, err := parseListParams(req)
	if err != nil {
		t.Fatal(err)
	}
	if params.Query != "edit" || params.Page != 2 || params.PageSize != 10 {
		t.Fatalf("unexpected params: %+v", params)
	}
	if params.ExcludeAssignedToGroupID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("unexpected exclude: %s", params.ExcludeAssignedToGroupID)
	}
}

func TestParseListParamsInvalidExclude(t *testing.T) {
	req := &http.Request{URL: &url.URL{RawQuery: "excludeAssignedToGroupId=not-a-uuid"}}
	if _, err := parseListParams(req); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildPermissionListQueryExclude(t *testing.T) {
	params := listParams{
		NetworkID:                "11111111-1111-1111-1111-111111111111",
		Query:                    "edit",
		Page:                     1,
		PageSize:                 20,
		ExcludeAssignedToGroupID: "22222222-2222-2222-2222-222222222222",
	}
	countSQL, listSQL, _, _ := buildPermissionListQuery(params)
	if !containsAll(countSQL, "ILIKE", "NOT IN") {
		t.Fatalf("count SQL missing filters: %s", countSQL)
	}
	if !containsAll(listSQL, "LIMIT", "OFFSET") {
		t.Fatalf("list SQL missing pagination: %s", listSQL)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !contains(s, part) {
			return false
		}
	}
	return true
}

func contains(s, part string) bool {
	return len(s) >= len(part) && (s == part || len(part) == 0 ||
		(func() bool {
			for i := 0; i+len(part) <= len(s); i++ {
				if s[i:i+len(part)] == part {
					return true
				}
			}
			return false
		})())
}
