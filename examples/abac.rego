package abac

# Attribute-based access control: subject + resource attributes must align.
#
# Query:  data.abac.allow
# Input:  {
#   "subject":  {"id": "alice", "clearance": "secret"},
#   "resource": {"id": "report-99", "classification": "secret"},
#   "action":   "read"
# }

default allow := false

levels := {"public": 0, "internal": 1, "confidential": 2, "secret": 3}

allow if {
	input.action == "read"
	levels[input.subject.clearance] >= levels[input.resource.classification]
}
