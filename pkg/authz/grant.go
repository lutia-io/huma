package authz

type FieldGrant struct {
	Name   string `json:"name"`
	Access string `json:"access"`
}

type Grant struct {
	Resource   string       `json:"resource"`
	Actions    []string     `json:"actions"`
	ResourceID string       `json:"resourceId,omitempty"`
	SchemaID   string       `json:"schemaId,omitempty"`
	Fields     []FieldGrant `json:"fields,omitempty"`
}

func (g Grant) hasAction(action string) bool {
	for _, a := range g.Actions {
		if a == action {
			return true
		}
	}
	return false
}

func (g Grant) matches(resource, resourceID, schemaID string) bool {
	if g.Resource != resource {
		return false
	}
	if g.ResourceID != "" && g.ResourceID != resourceID {
		return false
	}
	if g.SchemaID != "" && g.SchemaID != schemaID {
		return false
	}
	return true
}
