package k8s.admission

# Kubernetes admission control: every Pod must carry an `app` label.
#
# Query:  data.k8s.admission.violations[_]
# Input:  the AdmissionReview request (or just the Pod object's metadata)

violations contains msg if {
	input.kind.kind == "Pod"
	not input.object.metadata.labels.app
	msg := sprintf("Pod %s/%s is missing required label `app`", [
		input.object.metadata.namespace,
		input.object.metadata.name,
	])
}
