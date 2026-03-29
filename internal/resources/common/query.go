package common

type QueryOptions struct {
	Namespace     string
	LabelSelector string
	FieldSelector string
	AllNamespaces bool
	Wide          bool
	Output        string
}
