package standup

import (
	"fmt"
	"regexp"
	"strings"

	slacksvc "github.com/Flashgap/marvin/internal/service/slack"
)

// maxQuoteRunes caps the quoted update so a long message can't push the
// template past Slack's recommended 4,000-character message length.
const maxQuoteRunes = 3000

var (
	// todayHeaderRE matches the "Today:" header line, tolerating markup such
	// as "*Today:*", "_Today_:" or "> Today:", whose ">" Slack sends as "&gt;".
	todayHeaderRE = regexp.MustCompile(`(?i)^(?:[\s>*_~]|&gt;)*today[\s*_~]*:`)
	// headerRE matches any header of the standup template, which ends the
	// "Today:" section.
	headerRE = regexp.MustCompile(`(?i)^(?:[\s>*_~]|&gt;)*(yesterday|today|blockers?)[\s*_~]*:`)
)

// isStandupMessage reports whether m is a person's top-level post, as opposed
// to a system event, a bot or integration post, or a file without text.
func isStandupMessage(m slacksvc.Message) bool {
	return (m.SubType == "" || m.SubType == "file_share") &&
		m.BotID == "" &&
		m.UserID != "" &&
		strings.TrimSpace(m.Text) != ""
}

// latestUpdates maps each user to their most recent standup message. history
// is newest first, as returned by Slack.
func latestUpdates(history []slacksvc.Message) map[string]slacksvc.Message {
	latest := make(map[string]slacksvc.Message)
	for _, m := range history {
		if _, seen := latest[m.UserID]; !seen && isStandupMessage(m) {
			latest[m.UserID] = m
		}
	}
	return latest
}

// quotableText returns the "Today:" section of a standup update, or the whole
// text when there is no such header or its section is empty.
func quotableText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		loc := todayHeaderRE.FindStringIndex(line)
		if loc == nil {
			continue
		}
		section := []string{strings.TrimLeft(line[loc[1]:], " \t*_~")}
		for _, next := range lines[i+1:] {
			if headerRE.MatchString(next) {
				break
			}
			section = append(section, next)
		}
		if s := strings.TrimSpace(strings.Join(section, "\n")); s != "" {
			return s
		}
		break
	}
	return strings.TrimSpace(text)
}

// formatReminder builds the reminder DM. lastTS and lastText are the stored
// last update; an empty lastTS means there is none to quote.
func formatReminder(channelID, lastTS, lastText string) string {
	var b strings.Builder
	fmt.Fprintf(&b, ":wave: Time for your standup in <#%s>!\n", channelID)
	if lastTS != "" {
		// Slack renders the date token in the reader's timezone; the fallback is UTC.
		postedAt := slacksvc.ParseTS(lastTS)
		fmt.Fprintf(&b, "On <!date^%d^{date_long}|%s> you said:\n", postedAt.Unix(), postedAt.Format("Mon, 02 Jan 2006"))
		for line := range strings.SplitSeq(capQuote(quotableText(lastText)), "\n") {
			b.WriteString("> " + line + "\n")
		}
	}
	fmt.Fprintf(&b, "\nCopy, fill in and post in <#%s>:\nYesterday: …\nToday: …\nBlockers: …", channelID)
	return b.String()
}

// capQuote truncates s to maxQuoteRunes, at the last newline before the cap
// when there is one, and marks the cut with an ellipsis.
func capQuote(s string) string {
	runes := []rune(s)
	if len(runes) <= maxQuoteRunes {
		return s
	}
	cut := string(runes[:maxQuoteRunes])
	if i := strings.LastIndex(cut, "\n"); i > 0 {
		cut = cut[:i]
	}
	return cut + "…"
}
