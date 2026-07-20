package pipeline

type node struct {
	Slug string `json:"slug"`
}
type definition struct {
	Nodes [][]node `json:"nodes"`
}
