package slack_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/pkg/slack"
)

func TestSlack(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "slack suite")
}

var _ = DescribeTable("ParseTS",
	func(ts string, want time.Time) {
		Expect(slack.ParseTS(ts)).To(Equal(want))
	},
	Entry("seconds and micros", "1791274320.123456", time.Unix(1791274320, 0).UTC()),
	Entry("seconds only", "1791274320", time.Unix(1791274320, 0).UTC()),
	Entry("malformed", "nope", time.Time{}),
	Entry("empty", "", time.Time{}),
)
