package linguist_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/pkg/linguist"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Linguist pkg test suite")
}

var _ = Describe("Matcher", func() {
	files := map[string]string{
		".gitattributes": `
[attr]generated linguist-generated
**/mock/*.go linguist-generated
third_party/** linguist-vendored
*.min.js linguist-vendored=yes
docs/*.json generated
`,
		// The extra -diff makes go-git's Matcher let the broader line above overwrite the unset when it is
		// asked for linguist-generated directly
		"go/.gitattributes": `
*.pb.go linguist-generated
legacy.pb.go -linguist-generated -diff
mock/*.go linguist-generated=false
`,
		"go/api/.gitattributes": "legacy.pb.go linguist-generated\n",
		"py/.gitattributes":     "specs/**/*_pb2.py linguist-generated\n",
	}

	DescribeTable("Excludes",
		func(file string, excluded bool) {
			Expect(linguist.NewMatcher(files).Excludes(file)).To(Equal(excluded))
		},
		Entry("a nested file's pattern under its directory", "py/specs/v1/user_pb2.py", true),
		Entry("a nested file's pattern nowhere else", "specs/v1/user_pb2.py", false),
		Entry("unset by a later line", "go/a/legacy.pb.go", false),
		Entry("false in a deeper file", "go/mock/mock.go", false),
		Entry("set back in an even deeper file", "go/api/legacy.pb.go", true),
		Entry("vendored", "third_party/lib/zlib.c", true),
		Entry("any value but false, as Linguist", "web/app.min.js", true),
		Entry("through a macro", "docs/schema.json", true),
	)

	It("skips a line it can't parse and keeps the others", func() {
		m := linguist.NewMatcher(map[string]string{
			// go-git's parser panics on ""
			".gitattributes": "\"\"\n*.pb.go linguist-generated\n",
			// git only allows macros in the root file
			"go/.gitattributes": "[attr]gen linguist-generated\n*_gen.go linguist-generated\n",
		})
		Expect(m.Excludes("api.pb.go")).To(BeTrue())
		Expect(m.Excludes("go/user_gen.go")).To(BeTrue())
	})
})
