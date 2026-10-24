package node

// Category groups node types for authoring palettes.
type Category string

const (
	CategoryTransform   Category = "Transform"
	CategoryIntegration Category = "Integration"
	CategoryData        Category = "Data"
	CategoryFiles       Category = "Files"
	CategoryWorkflows   Category = "Workflows"
)

// CatalogEntry describes a node type for UIs and docs.
type CatalogEntry struct {
	Type        Type     `json:"type"`
	Name        string   `json:"name"`
	Category    Category `json:"category"`
	Description string   `json:"description"`
	// OutputShape is a short note of the JSON object the node returns.
	OutputShape string `json:"outputShape"`
}

// Catalog is the source of truth for display names, categories, and output shapes.
var Catalog = []CatalogEntry{
	{
		Type:        TypeNoop,
		Name:        "Debug",
		Category:    CategoryTransform,
		Description: "Echo a message without side effects. Useful for testing templates.",
		OutputShape: `{ "message": string }`,
	},
	{
		Type:        TypeMapper,
		Name:        "Set fields",
		Category:    CategoryTransform,
		Description: "Build one object from mapped fields and templates. Use to reshape data or pass fields across levels.",
		OutputShape: `{ ...mapped fields }`,
	},
	{
		Type:        TypeListMapper,
		Name:        "Map list",
		Category:    CategoryTransform,
		Description: "Map each item in a list into a new shape.",
		OutputShape: `{ "items": [ ... ] }`,
	},
	{
		Type:        TypeHTTP,
		Name:        "HTTP",
		Category:    CategoryIntegration,
		Description: "Call an HTTP API with method, URL, headers, and body.",
		OutputShape: `{ "status": number, "headers": object, "body": any }`,
	},
	{
		Type:        TypeFile,
		Name:        "File",
		Category:    CategoryFiles,
		Description: "Read or write a file in the workspace.",
		OutputShape: `WRITE: { fileId, filename, contentType, sizeBytes }; READ: same + content`,
	},
	{
		Type:        TypeRecord,
		Name:        "Record",
		Category:    CategoryData,
		Description: "List, get, create, update, or upsert a record.",
		OutputShape: `LIST: { items, total }; GET: { record }; CREATE/UPDATE/UPSERT: { id }`,
	},
	{
		Type:        TypeBulk,
		Name:        "Bulk records",
		Category:    CategoryData,
		Description: "Create or upsert many records from one or more lists.",
		OutputShape: `{ "created": [ { schemaId, ids, created, updated, total } ], "total": number }`,
	},
	{
		Type:        TypeTriggerWorkflow,
		Name:        "Trigger workflow",
		Category:    CategoryWorkflows,
		Description: "Enqueue another workflow by id or slug with a mapped input.",
		OutputShape: `{ "id": string, "input": object }`,
	},
}

// CatalogByType returns the catalog entry for t, if known.
func CatalogByType(t Type) (CatalogEntry, bool) {
	for _, e := range Catalog {
		if e.Type == t {
			return e, true
		}
	}
	return CatalogEntry{}, false
}
