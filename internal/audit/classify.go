package audit

import (
	"regexp"
	"strings"
)

var (
	shaRefPattern       = regexp.MustCompile(`(?i)^[a-f0-9]{40,64}$`)
	majorTagPattern     = regexp.MustCompile(`^v?\d+$`)
	exactTagPattern     = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`)
	semverishRefPattern = regexp.MustCompile(`^v?\d+(?:\.\d+){0,2}(?:[-+][0-9A-Za-z.-]+)?$`)
	tagLikePattern      = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

func classifyReference(kind Kind, raw string) (name string, ref string, refType RefType, risk Risk) {
	switch kind {
	case KindContainer:
		return classifyContainerReference(raw)
	case KindDockerAction:
		return classifyContainerReference(strings.TrimPrefix(raw, "docker://"))
	case KindAction, KindReusableWorkflow:
		return classifyActionReference(raw)
	default:
		return raw, "", RefTypeUnknown, RiskReview
	}
}

func classifyActionReference(raw string) (name string, ref string, refType RefType, risk Risk) {
	if strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") {
		return raw, "", RefTypeLocal, RiskSafe
	}

	name, ref, hasRef := strings.Cut(raw, "@")
	if !hasRef {
		return raw, "", RefTypeNone, RiskFloating
	}

	switch {
	case shaRefPattern.MatchString(ref):
		return name, ref, RefTypeSHA, RiskSafe
	case ref == "main" || ref == "master" || ref == "latest":
		return name, ref, RefTypeLatest, RiskFloating
	case strings.HasPrefix(ref, "refs/heads/"):
		return name, ref, RefTypeBranch, RiskFloating
	case strings.HasPrefix(ref, "refs/tags/"):
		return name, ref, RefTypeTag, RiskReview
	case majorTagPattern.MatchString(ref):
		return name, ref, RefTypeMajor, RiskReview
	case exactTagPattern.MatchString(ref):
		return name, ref, RefTypeExact, RiskReview
	case tagLikePattern.MatchString(ref):
		return name, ref, RefTypeBranch, RiskFloating
	default:
		return name, ref, RefTypeUnknown, RiskFloating
	}
}

func classifyContainerReference(raw string) (name string, ref string, refType RefType, risk Risk) {
	if strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") {
		return raw, "", RefTypeLocal, RiskSafe
	}

	if idx := strings.LastIndex(raw, "@"); idx >= 0 {
		name = raw[:idx]
		ref = raw[idx+1:]
		if strings.HasPrefix(ref, "sha256:") {
			return name, ref, RefTypeDigest, RiskSafe
		}
		return name, ref, RefTypeTag, RiskReview
	}

	slash := strings.LastIndex(raw, "/")
	colon := strings.LastIndex(raw, ":")
	if colon <= slash {
		return raw, "latest", RefTypeImplicit, RiskFloating
	}

	name = raw[:colon]
	ref = raw[colon+1:]
	switch {
	case ref == "" || ref == "latest":
		return name, ref, RefTypeLatest, RiskFloating
	case exactTagPattern.MatchString(ref):
		return name, ref, RefTypeExact, RiskReview
	case majorTagPattern.MatchString(ref):
		return name, ref, RefTypeMajor, RiskReview
	default:
		return name, ref, RefTypeTag, RiskFloating
	}
}

func pinningForOccurrence(occurrence Occurrence) (Pinning, bool) {
	switch occurrence.RefType {
	case RefTypeSHA, RefTypeDigest:
		return PinningSHA, true
	case RefTypeLocal:
		return "", false
	}

	ref := strings.TrimPrefix(occurrence.Ref, "refs/tags/")
	if semverishRefPattern.MatchString(ref) {
		return PinningSemver, true
	}
	if occurrence.Risk == RiskFloating || occurrence.RefType == RefTypeTag || occurrence.RefType == RefTypeNone || occurrence.RefType == RefTypeUnknown {
		return PinningFloating, true
	}

	return "", false
}
