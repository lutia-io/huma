package authz

import "fmt"

// MemberFilter is true when userParam belongs to the network identified by
// networkIDExpr (for example "n.id" or "s.network_id"). The network creator
// (created_by) or any group membership counts.
func MemberFilter(networkIDExpr, userParam string) string {
	return fmt.Sprintf(
		`(EXISTS (SELECT 1 FROM public.networks _hn_n WHERE _hn_n.id = %[1]s AND _hn_n.created_by = %[2]s) OR EXISTS (SELECT 1 FROM public.network_groups _hn_g JOIN public.network_group_members _hn_m ON _hn_m.group_id = _hn_g.id WHERE _hn_g.network_id = %[1]s AND _hn_g.deleted_at IS NULL AND _hn_m.user_id = %[2]s))`,
		networkIDExpr, userParam,
	)
}
