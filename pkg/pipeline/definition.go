package pipeline

type nodeRef struct {
	ID string `json:"id"`
}

type definition struct {
	Nodes [][]nodeRef `json:"nodes"`
}

func collectNodeIDs(def definition) []string {
	ids := make([]string, 0)
	for _, level := range def.Nodes {
		for _, n := range level {
			ids = append(ids, n.ID)
		}
	}
	return ids
}
