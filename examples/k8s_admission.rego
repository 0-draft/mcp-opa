package k8s.admission

# Kubernetes admission control: every Pod must carry an `app` label.
#
# Query:  data.k8s.admission.violations[_]
# Input:  the AdmissionReview request (or just the Pod object's metadata)

violations contains msg if {
	input.kind.kind == "Pod"
	not input.object.metadata.labels.app
	# Use object.get with fallbacks so an undefined name/namespace doesn't make
	# the whole sprintf evaluate to undefined and silently swallow the
	# violation.
	namespace := object.get(input.object.metadata, "namespace", "default")
	name := object.get(input.object.metadata, "name", "<unnamed>")
	msg := sprintf("Pod %s/%s is missing required label `app`", [namespace, name])
}
