package slack

import (
	"strconv"
	"strings"
	"time"
)

// ParseTS returns the UTC time of a Slack ts such as "1759820000.123456",
// truncated to the second. A malformed ts gives the zero time.
//
// Slack documents a ts as a message ID, unique within its channel, that is
// partly made of seconds since the epoch; only that part is read here:
// https://docs.slack.dev/messaging/retrieving-messages
func ParseTS(ts string) time.Time {
	sec, _, _ := strings.Cut(ts, ".")
	unix, err := strconv.ParseInt(sec, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(unix, 0).UTC()
}
