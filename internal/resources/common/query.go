package common

type QueryOptions struct {
	Namespace     string
	LabelSelector string
	FieldSelector string
	AllNamespaces bool
	Limit         int64
	Continue      string
	Wide          bool
	Output        string
}
