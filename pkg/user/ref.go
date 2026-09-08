package user

import "fmt"

// Ref is a public user summary nested on created-by / updated-by relations.
type Ref struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
}

// SelectSQL is created-by then updated-by user columns (cb / ub aliases).
const SelectSQL = `cb.id, cb.first_name, cb.last_name, cb.email, ub.id, ub.first_name, ub.last_name, ub.email`

// JoinSQL joins created-by and updated-by users for the given table alias.
func JoinSQL(alias string) string {
	return fmt.Sprintf(
		" JOIN public.users cb ON cb.id = %[1]s.created_by JOIN public.users ub ON ub.id = %[1]s.updated_by",
		alias,
	)
}
