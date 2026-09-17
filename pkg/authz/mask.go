package authz

import (
	"encoding/json"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/principal"
)

type FieldAccess struct {
	All    bool
	Fields map[string]string // name -> read|write
}

func (a FieldAccess) Readable(name string) bool {
	if a.All {
		return true
	}
	_, ok := a.Fields[name]
	return ok
}

func (a FieldAccess) Writable(name string) bool {
	if a.All {
		return true
	}
	return a.Fields[name] == FieldWrite
}

func (s *Snapshot) RecordAccess(schemaID string) FieldAccess {
	if s == nil || s.Principal.Type != principal.TypeOrganizationUser {
		return FieldAccess{All: true}
	}
	access := FieldAccess{Fields: map[string]string{}}
	matched := false
	for _, g := range s.Grants {
		if g.Resource != ResourceRecord {
			continue
		}
		if g.SchemaID != "" && g.SchemaID != schemaID {
			continue
		}
		if !g.hasAction(ActionRead) && !g.hasAction(ActionUpdate) {
			continue
		}
		matched = true
		if len(g.Fields) == 0 {
			access.All = true
			return access
		}
		for _, f := range g.Fields {
			if f.Access == FieldWrite || access.Fields[f.Name] != FieldWrite {
				access.Fields[f.Name] = f.Access
			}
		}
	}
	if !matched {
		return FieldAccess{Fields: map[string]string{}}
	}
	return access
}

func RecordView(data json.RawMessage, access FieldAccess) (json.RawMessage, error) {
	if access.All || len(data) == 0 || string(data) == "null" {
		return data, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	for key := range obj {
		if !access.Readable(key) {
			delete(obj, key)
		}
	}
	return json.Marshal(obj)
}

func RecordPatch(existing, incoming json.RawMessage, access FieldAccess) (json.RawMessage, error) {
	if access.All {
		return incoming, nil
	}
	var in map[string]json.RawMessage
	if err := json.Unmarshal(incoming, &in); err != nil {
		return nil, apperror.NewBadRequestError("Invalid record data", err)
	}
	for key := range in {
		if !access.Writable(key) {
			return nil, apperror.NewForbiddenError("Cannot write field "+key, nil)
		}
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(existing, &out); err != nil || out == nil {
		out = map[string]json.RawMessage{}
	}
	for key, value := range in {
		out[key] = value
	}
	return json.Marshal(out)
}

func (s *Snapshot) RejectHiddenListField(schemaID, field string) error {
	if s == nil || s.Principal.Type != principal.TypeOrganizationUser || field == "" {
		return nil
	}
	if !s.RecordAccess(schemaID).Readable(field) {
		return apperror.NewBadRequestError("Invalid field", nil)
	}
	return nil
}
