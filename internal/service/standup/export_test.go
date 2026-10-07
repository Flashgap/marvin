package standup

// Exposes the pure message helpers to message_test.go, which lives in
// standup_test: an internal test file can't dot-import Ginkgo, whose Report
// would clash with this package's.
var (
	IsStandupMessage = isStandupMessage
	LatestUpdates    = latestUpdates
	QuotableText     = quotableText
	FormatReminder   = formatReminder
)
