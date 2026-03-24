package githubtest

import (
	"bytes"
	"text/template"

	"github.com/onsi/gomega"
)

var prTpl = `
## Description
{{ .Description }}

## Fixed issues
{{ .LinearLink }}

## Time spent
{{ .TimeSpent }} hours spent

## Reviewer's notes
{{ .ReviewersNotes }}

### Tests
{{ .Tests }}
`

type PrData struct {
	Description string
	// ex: 1.5 or 1,5
	TimeSpent      string
	LinearLink     string
	ReviewersNotes string
	Tests          string
}

// BuildPrBody returns the PR body from pr with filled sections.
func BuildPrBody(pr PrData) string {
	t, err := template.New("prBody").Parse(prTpl)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "cannot parse template")
	var buf bytes.Buffer
	err = t.Execute(&buf, pr)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "cannot execute template")

	return buf.String()
}
