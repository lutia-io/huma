package file

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseListParams_defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/file", nil)
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
	if params.FilenameOp != opContains || params.OrganizationOp != opContains {
		t.Fatalf("got filenameOp=%s organizationOp=%s", params.FilenameOp, params.OrganizationOp)
	}
}

func TestParseListParams_paginationAndFilters(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/file?page=2&pageSize=10&q=invoice&sort=filename&order=asc&filename=Inv&filenameOp=startsWith&contentType=PDF&sizeBytes=1024&sizeBytesOp=gte&organization=DHL&organizationOp=contains", nil)
	params, err := parseListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.Page != 2 || params.PageSize != 10 {
		t.Fatalf("got page=%d pageSize=%d", params.Page, params.PageSize)
	}
	if params.Query != "invoice" || params.Sort != "filename" || params.Order != "asc" {
		t.Fatalf("got q=%s sort=%s order=%s", params.Query, params.Sort, params.Order)
	}
	if params.Filename != "Inv" || params.FilenameOp != opStartsWith {
		t.Fatalf("got filename=%s filenameOp=%s", params.Filename, params.FilenameOp)
	}
	if params.ContentType != "PDF" {
		t.Fatalf("got contentType=%s", params.ContentType)
	}
	if params.SizeBytes == nil || *params.SizeBytes != 1024 || params.SizeBytesOp != opGte {
		t.Fatalf("got sizeBytes=%v op=%s", params.SizeBytes, params.SizeBytesOp)
	}
	if params.Organization != "DHL" || params.OrganizationOp != opContains {
		t.Fatalf("got organization=%s organizationOp=%s", params.Organization, params.OrganizationOp)
	}
}

func TestParseListParams_rejectsInvalidSort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/file?sort=blob", nil)
	if _, err := parseListParams(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseListParams_rejectsInvalidType(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/file?contentType=video", nil)
	if _, err := parseListParams(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildListQuery_searchAndPagination(t *testing.T) {
	params := listParams{
		UserID:   "user-1",
		Query:    "inv_oice",
		Sort:     "filename",
		Order:    "asc",
		Page:     2,
		PageSize: 20,
	}
	countSQL, listSQL, countArgs, listArgs := buildListQuery(params)
	if !strings.Contains(countSQL, "n.user_id = $1") {
		t.Fatalf("count SQL missing user filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "f.filename ILIKE") || !strings.Contains(countSQL, "ESCAPE '!'") {
		t.Fatalf("count SQL missing search: %s", countSQL)
	}
	if got, want := countArgs[1], "%inv!_oice%"; got != want {
		t.Fatalf("escaped like pattern = %v want %v", got, want)
	}
	if !strings.Contains(listSQL, "ORDER BY f.filename ASC") {
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
		Sort:           "filename",
		Order:          "asc",
	}
	countSQL, _, _, _ := buildListQuery(params)
	if !strings.Contains(countSQL, "f.network_id = $2") {
		t.Fatalf("missing network filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "f.organization_id = $3") {
		t.Fatalf("missing organization filter: %s", countSQL)
	}
}

func TestBuildListQuery_typeSizeAndOrganization(t *testing.T) {
	size := int64(2048)
	params := listParams{
		UserID:         "user-1",
		ContentType:    "PDF",
		SizeBytes:      &size,
		SizeBytesOp:    opGte,
		Organization:   "DHL",
		OrganizationOp: opContains,
		Sort:           "contentType",
		Order:          "desc",
	}
	countSQL, listSQL, _, _ := buildListQuery(params)
	if !strings.Contains(countSQL, "("+fileKindExpr+") = ") {
		t.Fatalf("missing type filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "f.size_bytes >=") {
		t.Fatalf("missing size filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "o.name ILIKE") {
		t.Fatalf("missing organization name filter: %s", countSQL)
	}
	if !strings.Contains(listSQL, "ORDER BY "+fileKindExpr+" DESC") {
		t.Fatalf("missing type sort: %s", listSQL)
	}
}

func TestParseListParams_emptyFilter(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/file?filenameOp=empty&sizeBytesOp=empty", nil)
	params, err := parseListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.FilenameOp != opEmpty {
		t.Fatalf("got filenameOp=%s", params.FilenameOp)
	}
	if params.SizeBytesOp != opEmpty {
		t.Fatalf("got sizeBytesOp=%s", params.SizeBytesOp)
	}
}

func TestBuildListQuery_emptyFilters(t *testing.T) {
	params := listParams{
		UserID:      "user-1",
		FilenameOp:  opEmpty,
		SizeBytesOp: opEmpty,
		Sort:        "filename",
		Order:       "asc",
	}
	countSQL, _, _, _ := buildListQuery(params)
	if !strings.Contains(countSQL, "(f.filename IS NULL OR BTRIM((f.filename)::text) = '')") {
		t.Fatalf("missing empty filename filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "(f.size_bytes IS NULL OR f.size_bytes = 0)") {
		t.Fatalf("missing empty size filter: %s", countSQL)
	}
}
