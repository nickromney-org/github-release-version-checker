package audit

import (
	"fmt"
	"net/url"
)

func upstreamURL(kind Kind, name, host string) string {
	repo, ok := upstreamRepository(kind, name)
	if !ok || repo == "" {
		return ""
	}
	return fmt.Sprintf("https://%s/%s", normaliseHost(host), repo)
}

func latestRefURL(kind Kind, name, ref, host string) string {
	repo, ok := upstreamRepository(kind, name)
	if !ok || repo == "" || ref == "" {
		return ""
	}
	return fmt.Sprintf("https://%s/%s/tree/%s", normaliseHost(host), repo, url.PathEscape(ref))
}

func populateSummaryLinks(summary []SummaryRow, host string) {
	for i := range summary {
		summary[i].UpstreamURL = upstreamURL(summary[i].Kind, summary[i].Name, host)
		summary[i].LatestRefURL = latestRefURL(summary[i].Kind, summary[i].Name, summary[i].LatestRef, host)
	}
}
