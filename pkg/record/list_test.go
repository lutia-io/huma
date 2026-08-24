package record

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseListParams_defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/record", nil)
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
	if params.OrganizationOp != opContains {
		t.Fatalf("got organizationOp=%s", params.OrganizationOp)
	}
}

func TestParseListParams_paginationAndFilters(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/record?page=2&pageSize=10&q=invoice&sort=status&order=asc&schemaId=11111111-1111-1111-1111-111111111111&networkId=22222222-2222-2222-2222-222222222222&organizationId=33333333-3333-3333-3333-333333333333&organization=DHL&organizationOp=contains&field.status=delivered&fieldOp.status=eq&field.declaredValue=100&fieldOp.declaredValue=gte", nil)
	params, err := parseListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.Page != 2 || params.PageSize != 10 {
		t.Fatalf("got page=%d pageSize=%d", params.Page, params.PageSize)
	}
	if params.Query != "invoice" || params.Sort != "status" || params.Order != "asc" {
		t.Fatalf("got q=%s sort=%s order=%s", params.Query, params.Sort, params.Order)
	}
	if params.Organization != "DHL" || params.OrganizationOp != opContains {
		t.Fatalf("got organization=%s organizationOp=%s", params.Organization, params.OrganizationOp)
	}
	if len(params.Fields) != 2 {
		t.Fatalf("got %d fields", len(params.Fields))
	}
	if params.Fields[0].Name != "declaredValue" || params.Fields[0].Value != "100" || params.Fields[0].Op != opGte {
		t.Fatalf("got first field %#v", params.Fields[0])
	}
	if params.Fields[1].Name != "status" || params.Fields[1].Value != "delivered" || params.Fields[1].Op != opEq {
		t.Fatalf("got second field %#v", params.Fields[1])
	}
}

func TestParseListParams_rejectsInvalidSort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/record?sort=data.status", nil)
	if _, err := parseListParams(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseListParams_rejectsInvalidFieldName(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/record?field.status%20code=open", nil)
	if _, err := parseListParams(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseSchemaFields(t *testing.T) {
	fields, err := parseSchemaFields(json.RawMessage(`{
		"properties": {
			"status": { "type": "string", "enum": ["open", "closed"] },
			"declaredValue": { "type": "number" },
			"signatureCaptured": { "type": "boolean" },
			"proofFileId": { "type": "string", "format": "file" }
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if fields["status"].Kind != fieldKindString {
		t.Fatalf("status kind=%s", fields["status"].Kind)
	}
	if fields["declaredValue"].Kind != fieldKindNumber {
		t.Fatalf("declaredValue kind=%s", fields["declaredValue"].Kind)
	}
	if fields["signatureCaptured"].Kind != fieldKindBoolean {
		t.Fatalf("signatureCaptured kind=%s", fields["signatureCaptured"].Kind)
	}
	if fields["proofFileId"].Kind != fieldKindFile {
		t.Fatalf("proofFileId kind=%s", fields["proofFileId"].Kind)
	}
}

func TestBuildListQuery_searchAndPagination(t *testing.T) {
	params := listParams{
		UserID:   "user-1",
		Query:    "inv_oice",
		Sort:     "createdAt",
		Order:    "desc",
		Page:     2,
		PageSize: 20,
	}
	countSQL, listSQL, countArgs, listArgs := buildListQuery(params)
	if !strings.Contains(countSQL, "n.user_id = $1") {
		t.Fatalf("count SQL missing user filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "r.data::text ILIKE") || !strings.Contains(countSQL, "ESCAPE '!'") {
		t.Fatalf("count SQL missing search: %s", countSQL)
	}
	if got, want := countArgs[1], "%inv!_oice%"; got != want {
		t.Fatalf("escaped like pattern = %v want %v", got, want)
	}
	if !strings.Contains(listSQL, "ORDER BY r.created_at DESC") {
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

func TestBuildListQuery_schemaNetworkAndOrganization(t *testing.T) {
	params := listParams{
		UserID:         "user-1",
		SchemaID:       "schema-1",
		NetworkID:      "net-1",
		OrganizationID: "org-1",
		Sort:           "createdAt",
		Order:          "asc",
	}
	countSQL, _, _, _ := buildListQuery(params)
	if !strings.Contains(countSQL, "r.schema_id = $2") {
		t.Fatalf("missing schema filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "r.network_id = $3") {
		t.Fatalf("missing network filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "r.organization_id = $4") {
		t.Fatalf("missing organization filter: %s", countSQL)
	}
}

func TestBuildListQuery_dynamicFields(t *testing.T) {
	value := 100.0
	captured := true
	params := listParams{
		UserID: "user-1",
		Fields: []fieldFilter{
			{Name: "declaredValue", Value: "100", Op: opGte, Kind: fieldKindNumber, NumberValue: &value},
			{Name: "proofFileId", Value: "pod", Op: opContains, Kind: fieldKindFile},
			{Name: "signatureCaptured", Value: "true", Op: opEq, Kind: fieldKindBoolean, BooleanValue: &captured},
			{Name: "status", Value: "delivered", Op: opEq, Kind: fieldKindString},
		},
		SchemaFields: map[string]schemaField{
			"declaredValue":     {Name: "declaredValue", Kind: fieldKindNumber},
			"proofFileId":       {Name: "proofFileId", Kind: fieldKindFile},
			"signatureCaptured": {Name: "signatureCaptured", Kind: fieldKindBoolean},
			"status":            {Name: "status", Kind: fieldKindString},
		},
		Sort:  "declaredValue",
		Order: "desc",
	}
	countSQL, listSQL, _, _ := buildListQuery(params)
	if !strings.Contains(countSQL, "(jsonb_typeof(r.data -> $2) = 'number' AND (r.data ->> $2)::numeric >= $3)") {
		t.Fatalf("missing number filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "r.data -> $6 = 'true'::jsonb") {
		t.Fatalf("missing boolean filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "r.data ->> $7 ILIKE") {
		t.Fatalf("missing string filter: %s", countSQL)
	}
	if !strings.Contains(countSQL, "SELECT f.filename FROM public.files") {
		t.Fatalf("missing file filter: %s", countSQL)
	}
	if !strings.Contains(listSQL, "ORDER BY (CASE WHEN jsonb_typeof(r.data -> $9) = 'number' THEN (r.data ->> $9)::numeric END) DESC") {
		t.Fatalf("missing numeric sort: %s", listSQL)
	}
}

func TestParseListParams_emptyFieldFilter(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/record?fieldOp.status=empty&organizationOp=empty", nil)
	params, err := parseListParams(r)
	if err != nil {
		t.Fatal(err)
	}
	if params.OrganizationOp != opEmpty {
		t.Fatalf("got organizationOp=%s", params.OrganizationOp)
	}
	if len(params.Fields) != 1 || params.Fields[0].Name != "status" || params.Fields[0].Op != opEmpty {
		t.Fatalf("got fields %#v", params.Fields)
	}
}

func TestBuildListQuery_emptyField(t *testing.T) {
	params := listParams{
		UserID: "user-1",
		Fields: []fieldFilter{
			{Name: "status", Op: opEmpty, Kind: fieldKindString},
		},
		Sort:  "createdAt",
		Order: "desc",
	}
	countSQL, _, _, _ := buildListQuery(params)
	if !strings.Contains(countSQL, "jsonb_typeof(r.data -> $2) = 'null'") {
		t.Fatalf("missing empty json filter: %s", countSQL)
	}
}
