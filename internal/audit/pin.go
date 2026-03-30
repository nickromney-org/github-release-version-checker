package audit

type summaryMatchKey struct {
	Kind Kind
	Name string
	Ref  string
}

func summaryKey(kind Kind, name, ref string) summaryMatchKey {
	return summaryMatchKey{
		Kind: kind,
		Name: name,
		Ref:  ref,
	}
}

func pinnedUsesValue(occurrence Occurrence) string {
	if occurrence.LatestSHA == "" || occurrence.LatestRef == "" {
		return ""
	}

	switch occurrence.Kind {
	case KindAction, KindReusableWorkflow:
		return occurrence.Name + "@" + occurrence.LatestSHA + " # pin@" + occurrence.LatestRef
	default:
		return ""
	}
}
