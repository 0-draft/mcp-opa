package rbac

# Role-based access control.
#
# Query:  data.rbac.allow
# Input:  {"user": "alice", "action": "read", "resource": "doc-1"}

default allow := false

roles := {
	"alice": {"editor"},
	"bob":   {"viewer"},
	"carol": {"viewer", "auditor"},
}

permissions := {
	"editor":  {"read", "write", "delete"},
	"viewer":  {"read"},
	"auditor": {"read", "audit"},
}

allow if {
	some role in roles[input.user]
	some perm in permissions[role]
	perm == input.action
}
