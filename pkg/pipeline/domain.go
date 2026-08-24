package pipeline

import "time"

type pipelineDefinition struct {
	ID string `json:"id"`

	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Active   bool   `json:"active"`
	Internal bool   `json:"internal"`

	Definition definition `json:"definition"`

	NetworkID string `json:"networkId"`
	UserID    string `json:"userId"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type insertPipelineDefinitionRequest struct {
	Name       string     `json:"name"`
	Active     bool       `json:"active"`
	Internal   bool       `json:"internal"`
	Definition definition `json:"definition"`
	SchemaID   string     `json:"schemaId"`
	NetworkID  string     `json:"networkId"`
	UserID     string     `json:"-"`
}

type listParams struct {
	UserID    string
	Query     string
	NetworkID string
	Active    *bool
	Name      string
	NameOp    string
	Slug      string
	SlugOp    string
	Network   string
	NetworkOp string
	Source    string
	SourceOp  string
	Stages    *int
	StagesOp  string
	Sort      string
	Order     string
	Page      int
	PageSize  int
}

type listResult struct {
	Items    []*pipelineDefinition `json:"items"`
	Total    int                   `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"pageSize"`
}
