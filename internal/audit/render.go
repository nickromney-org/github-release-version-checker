package audit

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	colour "github.com/fatih/color"
)

func Render(w io.Writer, result *Result, format Format, view View) error {
	switch format {
	case FormatJSON:
		return renderJSON(w, result)
	case FormatCSV:
		return renderCSV(w, result, view)
	case FormatMarkdown:
		return renderMarkdown(w, result, view)
	default:
		return renderTable(w, result, view)
	}
}

func renderJSON(w io.Writer, result *Result) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func renderCSV(w io.Writer, result *Result, view View) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if view == ViewOccurrences {
		header := []string{"workflow_path", "job", "step", "kind", "name", "current_ref", "latest_ref"}
		if showLatestAgeInOccurrences(result) {
			header = append(header, "latest_age_days")
		}
		if showLatestSHAInOccurrences(result) {
			header = append(header, "latest_sha")
		}
		header = append(header, "pinning")
		if showPinnedUses(result) {
			header = append(header, "pinned_uses")
		}
		header = append(header, "line")
		if showRepoColumn(result) {
			header = append([]string{"repo"}, header...)
		}
		if err := writer.Write(header); err != nil {
			return err
		}

		for _, occurrence := range result.Occurrences {
			row := []string{
				workflowDisplayPath(occurrence),
				occurrence.Job,
				occurrence.Step,
				string(occurrence.Kind),
				occurrence.Name,
				occurrence.Ref,
				occurrence.LatestRef,
			}
			if showLatestAgeInOccurrences(result) {
				row = append(row, displayAgeValue(occurrence.LatestAgeDays))
			}
			if showLatestSHAInOccurrences(result) {
				row = append(row, occurrence.LatestSHA)
			}
			row = append(row, string(occurrence.Pinning))
			if showPinnedUses(result) {
				row = append(row, occurrence.PinnedUses)
			}
			row = append(row, strconv.Itoa(occurrence.Line))
			if showRepoColumn(result) {
				row = append([]string{repoDisplayName(occurrence)}, row...)
			}
			if err := writer.Write(row); err != nil {
				return err
			}
		}

		return writer.Error()
	}

	header := []string{"kind", "name", "current_ref", "latest_ref"}
	if showLatestAgeInSummary(result) {
		header = append(header, "latest_age_days")
	}
	if showLatestSHAInSummary(result) {
		header = append(header, "latest_sha")
	}
	header = append(header, "pinning")
	if showRepoCount(result) {
		header = append(header, "repo_count")
	}
	header = append(header, "occurrence_count")
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, row := range result.Summary {
		record := []string{
			string(row.Kind),
			row.Name,
			row.Ref,
			row.LatestRef,
		}
		if showLatestAgeInSummary(result) {
			record = append(record, displayAgeValue(row.LatestAgeDays))
		}
		if showLatestSHAInSummary(result) {
			record = append(record, row.LatestSHA)
		}
		record = append(record, string(row.Pinning))
		if showRepoCount(result) {
			record = append(record, strconv.Itoa(row.RepoCount))
		}
		record = append(record, strconv.Itoa(row.OccurrenceCount))
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return writer.Error()
}

func renderTable(w io.Writer, result *Result, view View) error {
	if view == ViewOccurrences {
		header := []tableCell{
			plainTableCell("WORKFLOW"),
			plainTableCell("JOB"),
			plainTableCell("STEP"),
			plainTableCell("KIND"),
			plainTableCell("NAME"),
			plainTableCell("CURRENT"),
			plainTableCell("LATEST"),
		}
		if showRepoColumn(result) {
			header = append([]tableCell{plainTableCell("REPO")}, header...)
		}
		if showLatestAgeInOccurrences(result) {
			header = append(header, plainTableCell("LATEST AGE"))
		}
		if showLatestSHAInOccurrences(result) {
			header = append(header, plainTableCell("LATEST SHA"))
		}
		header = append(header, plainTableCell("PINNING"))
		if showPinnedUses(result) {
			header = append(header, plainTableCell("PIN"))
		}
		header = append(header, plainTableCell("LINE"))

		rows := make([][]tableCell, 0, len(result.Occurrences)+1)
		rows = append(rows, header)

		for _, occurrence := range result.Occurrences {
			row := []tableCell{
				plainTableCell(workflowDisplayPath(occurrence)),
				plainTableCell(occurrence.Job),
				plainTableCell(occurrence.Step),
				plainTableCell(string(occurrence.Kind)),
				plainTableCell(occurrence.Name),
				plainTableCell(displayValue(occurrence.Ref)),
				latestTableCell(occurrence.Ref, occurrence.LatestRef),
			}
			if showRepoColumn(result) {
				row = append([]tableCell{plainTableCell(repoDisplayName(occurrence))}, row...)
			}
			if showLatestAgeInOccurrences(result) {
				row = append(row, plainTableCell(displayAgeValue(occurrence.LatestAgeDays)))
			}
			if showLatestSHAInOccurrences(result) {
				row = append(row, plainTableCell(displayValue(shortSHA(occurrence.LatestSHA))))
			}
			row = append(row, plainTableCell(displayValue(string(occurrence.Pinning))))
			if showPinnedUses(result) {
				row = append(row, plainTableCell(displayValue(occurrence.PinnedUses)))
			}
			row = append(row, plainTableCell(strconv.Itoa(occurrence.Line)))
			rows = append(rows, row)
		}

		return renderAlignedTable(w, rows)
	}

	header := []tableCell{
		plainTableCell("KIND"),
		plainTableCell("NAME"),
		plainTableCell("CURRENT"),
		plainTableCell("LATEST"),
	}
	if showLatestAgeInSummary(result) {
		header = append(header, plainTableCell("LATEST AGE"))
	}
	if showLatestSHAInSummary(result) {
		header = append(header, plainTableCell("LATEST SHA"))
	}
	header = append(header, plainTableCell("PINNING"))
	if showRepoCount(result) {
		header = append(header, plainTableCell("REPOS"))
	}
	header = append(header, plainTableCell("OCCURRENCES"))

	rows := make([][]tableCell, 0, len(result.Summary)+1)
	rows = append(rows, header)

	for _, row := range result.Summary {
		record := []tableCell{
			plainTableCell(string(row.Kind)),
			plainTableCell(row.Name),
			plainTableCell(displayValue(row.Ref)),
			latestTableCell(row.Ref, row.LatestRef),
		}
		if showLatestAgeInSummary(result) {
			record = append(record, plainTableCell(displayAgeValue(row.LatestAgeDays)))
		}
		if showLatestSHAInSummary(result) {
			record = append(record, plainTableCell(displayValue(shortSHA(row.LatestSHA))))
		}
		record = append(record, plainTableCell(displayValue(string(row.Pinning))))
		if showRepoCount(result) {
			record = append(record, plainTableCell(strconv.Itoa(row.RepoCount)))
		}
		record = append(record, plainTableCell(strconv.Itoa(row.OccurrenceCount)))
		rows = append(rows, record)
	}

	return renderAlignedTable(w, rows)
}

type tableCell struct {
	raw     string
	display string
}

func plainTableCell(value string) tableCell {
	return tableCell{raw: value, display: value}
}

func latestTableCell(current, latest string) tableCell {
	value := displayValue(latest)
	cell := plainTableCell(value)
	if !shouldHighlightLatest(current, latest) {
		return cell
	}
	return tableCell{
		raw:     value,
		display: highlightLatest(value),
	}
}

func shouldHighlightLatest(current, latest string) bool {
	currentValue := displayValue(current)
	latestValue := displayValue(latest)
	return latestValue != "-" && currentValue != latestValue
}

func highlightLatest(value string) string {
	if colour.NoColor {
		return value
	}
	return "\x1b[1;31m" + value + "\x1b[0m"
}

func renderAlignedTable(w io.Writer, rows [][]tableCell) error {
	if len(rows) == 0 {
		return nil
	}

	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			if width := utf8.RuneCountInString(cell.raw); width > widths[i] {
				widths[i] = width
			}
		}
	}

	for _, row := range rows {
		for i, cell := range row {
			if _, err := io.WriteString(w, cell.display); err != nil {
				return err
			}
			if i == len(row)-1 {
				continue
			}
			padding := widths[i] - utf8.RuneCountInString(cell.raw) + 2
			if _, err := io.WriteString(w, strings.Repeat(" ", padding)); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}

	return nil
}

func renderMarkdown(w io.Writer, result *Result, view View) error {
	if view == ViewOccurrences {
		header := []string{"Workflow", "Job", "Step", "Kind", "Name", "Current", "Latest"}
		if showRepoColumn(result) {
			header = append([]string{"Repo"}, header...)
		}
		if showLatestAgeInOccurrences(result) {
			header = append(header, "Latest Age")
		}
		if showLatestSHAInOccurrences(result) {
			header = append(header, "Latest SHA")
		}
		header = append(header, "Pinning")
		if showPinnedUses(result) {
			header = append(header, "Pin")
		}
		header = append(header, "Line")
		if _, err := fmt.Fprintln(w, markdownRow(header)); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, markdownDivider(len(header))); err != nil {
			return err
		}

		for _, occurrence := range result.Occurrences {
			row := []string{
				markdownLink(workflowDisplayPath(occurrence), workflowLink(occurrence, result.Metadata.Host)),
				markdownText(occurrence.Job),
				markdownText(occurrence.Step),
				markdownText(string(occurrence.Kind)),
				markdownLink(occurrence.Name, occurrence.UpstreamURL),
				markdownText(displayValue(occurrence.Ref)),
				markdownLink(displayValue(occurrence.LatestRef), occurrence.LatestRefURL),
			}
			if showRepoColumn(result) {
				row = append([]string{markdownLink(repoDisplayName(occurrence), repoLink(occurrence, result.Metadata.Host))}, row...)
			}
			if showLatestAgeInOccurrences(result) {
				row = append(row, markdownText(displayAgeValue(occurrence.LatestAgeDays)))
			}
			if showLatestSHAInOccurrences(result) {
				row = append(row, markdownCode(occurrence.LatestSHA))
			}
			row = append(row, markdownText(displayValue(string(occurrence.Pinning))))
			if showPinnedUses(result) {
				row = append(row, markdownCode(occurrence.PinnedUses))
			}
			row = append(row, strconv.Itoa(occurrence.Line))
			if _, err := fmt.Fprintln(w, markdownRow(row)); err != nil {
				return err
			}
		}

		return nil
	}

	header := []string{"Kind", "Name", "Current", "Latest"}
	if showLatestAgeInSummary(result) {
		header = append(header, "Latest Age")
	}
	if showLatestSHAInSummary(result) {
		header = append(header, "Latest SHA")
	}
	header = append(header, "Pinning")
	if showRepoCount(result) {
		header = append(header, "Repos")
	}
	header = append(header, "Occurrences")
	if _, err := fmt.Fprintln(w, markdownRow(header)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, markdownDivider(len(header))); err != nil {
		return err
	}

	for _, row := range result.Summary {
		record := []string{
			markdownText(string(row.Kind)),
			markdownLink(row.Name, row.UpstreamURL),
			markdownText(displayValue(row.Ref)),
			markdownLink(displayValue(row.LatestRef), row.LatestRefURL),
		}
		if showLatestAgeInSummary(result) {
			record = append(record, markdownText(displayAgeValue(row.LatestAgeDays)))
		}
		if showLatestSHAInSummary(result) {
			record = append(record, markdownCode(row.LatestSHA))
		}
		record = append(record, markdownText(displayValue(string(row.Pinning))))
		if showRepoCount(result) {
			record = append(record, strconv.Itoa(row.RepoCount))
		}
		record = append(record, strconv.Itoa(row.OccurrenceCount))
		if _, err := fmt.Fprintln(w, markdownRow(record)); err != nil {
			return err
		}
	}

	return nil
}

func showRepoCount(result *Result) bool {
	return result.Metadata.RepositoriesScanned > 1
}

func showRepoColumn(result *Result) bool {
	return result.Metadata.RepositoriesScanned > 1
}

func repoDisplayName(occurrence Occurrence) string {
	if occurrence.RepoFullName != "" {
		return occurrence.RepoFullName
	}
	if occurrence.RepoPathOrFullName != "" {
		return occurrence.RepoPathOrFullName
	}
	if occurrence.RepoName != "" {
		return occurrence.RepoName
	}
	return "-"
}

func repoLink(occurrence Occurrence, host string) string {
	if occurrence.RepoPath != "" {
		return occurrence.RepoPath
	}
	if occurrence.RepoFullName != "" {
		return fmt.Sprintf("https://%s/%s", normaliseHost(host), occurrence.RepoFullName)
	}
	return ""
}

func workflowLink(occurrence Occurrence, host string) string {
	if occurrence.RepoPath != "" {
		return filepath.Join(occurrence.RepoPath, filepath.FromSlash(occurrence.WorkflowPath))
	}
	if occurrence.RepoFullName != "" && occurrence.RepoDefaultBranch != "" {
		return fmt.Sprintf("https://%s/%s/blob/%s/%s", normaliseHost(host), occurrence.RepoFullName, occurrence.RepoDefaultBranch, occurrence.WorkflowPath)
	}
	return ""
}

func workflowDisplayPath(occurrence Occurrence) string {
	if occurrence.RepoPath != "" {
		return filepath.Join(occurrence.RepoPath, filepath.FromSlash(occurrence.WorkflowPath))
	}
	return occurrence.WorkflowPath
}

func normaliseHost(host string) string {
	if host == "" {
		return "github.com"
	}
	return strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
}

func markdownLink(label, target string) string {
	if target == "" {
		return markdownText(label)
	}
	return "[" + markdownText(label) + "](" + target + ")"
}

func markdownText(value string) string {
	if value == "" {
		return "-"
	}
	replacer := strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ")
	return replacer.Replace(value)
}

func markdownCode(value string) string {
	if value == "" {
		return "-"
	}
	value = strings.ReplaceAll(value, "`", "\\`")
	return "`" + value + "`"
}

func markdownRow(cells []string) string {
	return "| " + strings.Join(cells, " | ") + " |"
}

func markdownDivider(columns int) string {
	parts := make([]string, columns)
	for i := range parts {
		parts[i] = "---"
	}
	return markdownRow(parts)
}

func displayValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func displayAgeValue(value *int) string {
	if value == nil {
		return "-"
	}
	return strconv.Itoa(*value)
}

func showLatestAgeInSummary(result *Result) bool {
	for _, row := range result.Summary {
		if row.LatestAgeDays != nil {
			return true
		}
	}
	return false
}

func showLatestAgeInOccurrences(result *Result) bool {
	for _, occurrence := range result.Occurrences {
		if occurrence.LatestAgeDays != nil {
			return true
		}
	}
	return false
}

func showLatestSHAInSummary(result *Result) bool {
	for _, row := range result.Summary {
		if row.LatestSHA != "" {
			return true
		}
	}
	return false
}

func showLatestSHAInOccurrences(result *Result) bool {
	for _, occurrence := range result.Occurrences {
		if occurrence.LatestSHA != "" {
			return true
		}
	}
	return false
}

func showPinnedUses(result *Result) bool {
	for _, occurrence := range result.Occurrences {
		if occurrence.PinnedUses != "" {
			return true
		}
	}
	return false
}

func shortSHA(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}
