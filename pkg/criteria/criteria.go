// Package criteria defines the reusable condition tree, together with Match,
// which evaluates a tree against data. It is pure computation with no I/O.
package criteria

type LogicalOp string

const (
	LogicAnd LogicalOp = "AND"
	LogicOr  LogicalOp = "OR"
	LogicNot LogicalOp = "NOT"
)

type CompareOp string

const (
	OpEq  CompareOp = "eq"
	OpNeq CompareOp = "neq"
	OpGt  CompareOp = "gt"
	OpGte CompareOp = "gte"
	OpLt  CompareOp = "lt"
	OpLte CompareOp = "lte"
	OpIn  CompareOp = "in"
)

// Criteria is either a group (Logic + Conditions) or a leaf (Field + Operator + Value).
type Criteria struct {
	Logic      LogicalOp  `json:"logic,omitempty"`
	Conditions []Criteria `json:"conditions,omitempty"`
	Field      string     `json:"field,omitempty"`
	Operator   CompareOp  `json:"operator,omitempty"`
	Value      any        `json:"value,omitempty"`
}
