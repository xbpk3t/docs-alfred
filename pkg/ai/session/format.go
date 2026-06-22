package session

import (
	"fmt"
	"strings"
)

// FormatMessages renders messages into a Turn-structured markdown document.
//
// Processing steps:
//  1. Merge consecutive same-role messages (assistant text blocks that were
//     split across multiple JSONL events get recombined into one).
//  2. Pair messages into (user, assistant) turns.
//  3. Format each turn with headers and proper wrapping.
//
// Format:
//
//	# Turn 1
//
//	```markdown
//	<user content>
//	```
//
//	<assistant content>
//
//
//	# Turn 2 [YYYY-MM-DD]
//
//	```markdown
//	<user content>
//	```
//
//	<assistant content>
func FormatMessages(messages []Message) string {
	if len(messages) == 0 {
		return ""
	}

	// Step 1: Merge consecutive same-role messages
	merged := mergeSameRole(messages)

	// Step 2: Pair into turns and format
	var sb strings.Builder
	turnNum := 1
	var lastDate string

	i := 0
	for i < len(merged) {
		// Complete turn: user + assistant
		if i+1 < len(merged) && merged[i].Role == roleUser && merged[i+1].Role == roleAssistant {
			userMsg := merged[i]
			asstMsg := merged[i+1]

			writeTurnHeader(&sb, turnNum, userMsg.Timestamp, lastDate)
			sb.WriteString("\n\n")
			writeUserContent(&sb, userMsg.Content)
			sb.WriteString("\n\n")
			sb.WriteString(asstMsg.Content)
			sb.WriteString("\n\n\n")

			lastDate = coalesceDate(userMsg.Timestamp, lastDate)
			i += 2
		} else {
			// Orphan message (no partner to pair with)
			msg := merged[i]

			writeTurnHeader(&sb, turnNum, msg.Timestamp, lastDate)
			sb.WriteString("\n\n")
			if msg.Role == roleUser {
				writeUserContent(&sb, msg.Content)
			} else {
				sb.WriteString(msg.Content)
			}
			sb.WriteString("\n\n\n")

			lastDate = coalesceDate(msg.Timestamp, lastDate)
			i++
		}
		turnNum++
	}

	return sb.String()
}

// --- pairing helper ----------------------------------------------------------

// mergeSameRole merges consecutive messages that share the same role.
// This recombines assistant text blocks that were split across multiple
// JSONL events (separated by tool_results and thinking blocks in the stream).
func mergeSameRole(messages []Message) []Message {
	if len(messages) <= 1 {
		return messages
	}

	merged := make([]Message, 0, len(messages))
	current := messages[0]

	for i := 1; i < len(messages); i++ {
		if messages[i].Role == current.Role {
			// Merge: concatenate content, keep the later timestamp
			current.Content = current.Content + "\n\n" + messages[i].Content
			current.Timestamp = messages[i].Timestamp
		} else {
			merged = append(merged, current)
			current = messages[i]
		}
	}
	merged = append(merged, current)

	return merged
}

// --- formatting helpers ------------------------------------------------------

// writeTurnHeader writes "# Turn N\n" with an optional "[YYYY-MM-DD]" annotation
// when the turn's date differs from the previous turn's date.
//
// Date annotations are NOT added to Turn 1 — they start from Turn 2 when
// the date changes.
func writeTurnHeader(sb *strings.Builder, num int, timestamp, lastDate string) {
	date := extractDatePart(timestamp)
	if num > 1 && date != "" && date != lastDate {
		fmt.Fprintf(sb, "# Turn %d [%s]", num, date)
	} else {
		fmt.Fprintf(sb, "# Turn %d", num)
	}
}

// writeUserContent wraps user content in a markdown code block.
func writeUserContent(sb *strings.Builder, content string) {
	sb.WriteString("```markdown\n")
	sb.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("```")
}

// --- date helpers ------------------------------------------------------------

// extractDatePart extracts the YYYY-MM-DD date from an ISO 8601 timestamp.
// Returns empty string if the timestamp cannot be parsed or has no date.
func extractDatePart(timestamp string) string {
	// ISO 8601 format: 2026-06-21T10:00:00Z or 2026-06-21T10:00:00.000Z or 2026-06-21
	if len(timestamp) < 10 {
		return ""
	}

	return timestamp[:10]
}

// coalesceDate returns the date from timestamp, falling back to lastDate
// if the timestamp is empty.
func coalesceDate(timestamp, lastDate string) string {
	date := extractDatePart(timestamp)
	if date == "" {
		return lastDate
	}

	return date
}
