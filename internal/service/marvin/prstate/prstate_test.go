package prstate_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/internal/service/marvin/prstate"
)

func TestPRState(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "prstate Suite")
}

type transitionCase struct {
	name    string
	from    prstate.State
	event   prstate.Event
	allowed bool
	to      prstate.State
	ok      bool
}

var _ = Describe("Transition", func() {
	cases := []transitionCase{
		{"opened draft from None", prstate.None, prstate.OpenedDraft, false, prstate.WorkInProgress, true},
		{"converted to draft from None", prstate.None, prstate.ConvertedToDraft, false, prstate.WorkInProgress, true},
		{"ready requested from None, gate open", prstate.None, prstate.ReadyRequested, true, prstate.ReadyForReview, true},
		{"ready requested from None, gate blocked", prstate.None, prstate.ReadyRequested, false, prstate.WorkInProgress, true},

		{"ready requested from WIP, gate open", prstate.WorkInProgress, prstate.ReadyRequested, true, prstate.ReadyForReview, true},
		{"ready requested from WIP, gate blocked", prstate.WorkInProgress, prstate.ReadyRequested, false, prstate.WorkInProgress, true},
		{"converted to draft from WIP is a no-op", prstate.WorkInProgress, prstate.ConvertedToDraft, false, prstate.WorkInProgress, false},

		{"converted to draft from ReadyForReview", prstate.ReadyForReview, prstate.ConvertedToDraft, false, prstate.WorkInProgress, true},
		{"changes requested from ReadyForReview", prstate.ReadyForReview, prstate.ReviewChangesRequested, false, prstate.ChangesRequired, true},
		{"approved from ReadyForReview, enough approvals", prstate.ReadyForReview, prstate.ReviewApproved, true, prstate.Approved, true},
		{"approved from ReadyForReview, not enough approvals is a no-op", prstate.ReadyForReview, prstate.ReviewApproved, false, prstate.ReadyForReview, false},

		{"converted to draft from ChangesRequired", prstate.ChangesRequired, prstate.ConvertedToDraft, false, prstate.WorkInProgress, true},
		{"ready requested from ChangesRequired, gate open", prstate.ChangesRequired, prstate.ReadyRequested, true, prstate.ReadyForReview, true},
		{"ready requested from ChangesRequired, gate blocked", prstate.ChangesRequired, prstate.ReadyRequested, false, prstate.WorkInProgress, true},
		{"approved from ChangesRequired, enough approvals", prstate.ChangesRequired, prstate.ReviewApproved, true, prstate.Approved, true},

		{"converted to draft from Approved", prstate.Approved, prstate.ConvertedToDraft, false, prstate.WorkInProgress, true},
		{"changes requested from Approved", prstate.Approved, prstate.ReviewChangesRequested, false, prstate.ChangesRequired, true},
		{"ready requested from Approved is a no-op (not in table)", prstate.Approved, prstate.ReadyRequested, true, prstate.Approved, false},
	}

	for _, c := range cases {
		c := c
		It(c.name, func() {
			to, ok := prstate.Transition(c.from, c.event, prstate.Guards{Allowed: c.allowed})
			Expect(to).To(Equal(c.to))
			Expect(ok).To(Equal(c.ok))
		})
	}
})
