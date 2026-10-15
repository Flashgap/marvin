package marvin_test

import (
	"errors"
	"net/http"

	gogithub "github.com/google/go-github/v90/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/service/marvin"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
)

var _ = Describe("RepoConfigProvider's .gitattributes", func() {
	const defaultBranch = "main"

	var (
		mockGithub *mock_github.MockClient
		repo       *gogithub.Repository
		provider   marvin.RepoConfigProvider
	)

	// blobs lists .gitattributes files on the default branch, by path to blob SHA
	blobs := func(shas map[string]string) *gogithub.Tree {
		tree := &gogithub.Tree{}
		for path, sha := range shas {
			tree.Entries = append(tree.Entries, &gogithub.TreeEntry{Path: new(path), Type: new("blob"), SHA: new(sha)})
		}
		return tree
	}
	generated := blobs(map[string]string{"go/.gitattributes": "generated"})

	// expectBlob downloads the .gitattributes file of blob sha, at most once
	expectBlob := func(sha, content string, err error) {
		mockGithub.EXPECT().GetBlobRaw(gomock.Any(), gomock.Any(), sha).Return([]byte(content), &gogithub.Response{}, err).MaxTimes(1)
	}

	// expectMarvinYAML answers every .marvin.yaml fetch with a valid file, or fails it with err
	expectMarvinYAML := func(err error) {
		res := &gogithub.Response{Response: &http.Response{StatusCode: http.StatusOK}}
		if err != nil {
			res.StatusCode = http.StatusInternalServerError
		}
		mockGithub.EXPECT().GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [auto_merge]", res, err).AnyTimes()
	}

	BeforeEach(func() {
		mockGithub = mock_github.NewMockClient(gomock.NewController(GinkgoT()))
		repo = &gogithub.Repository{Name: new(repoName), FullName: new("acme/" + repoName), DefaultBranch: new(defaultBranch)}
		provider = marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		mockGithub.EXPECT().ListInstalledRepos(gomock.Any(), gomock.Any()).
			Return(&gogithub.ListRepositories{Repositories: []*gogithub.Repository{repo}}, &gogithub.Response{}, nil).AnyTimes()
		expectBlob("generated", "*.pb.go linguist-generated\n", nil)
	})

	It("reads them off the default branch of a repository without .marvin.yaml, downloading each blob once", func(ctx SpecContext) {
		mockGithub.EXPECT().GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("", &gogithub.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, errors.New("404 Not Found")).Times(2)
		mockGithub.EXPECT().GetTree(gomock.Any(), gomock.Any(), defaultBranch, true).
			Return(blobs(map[string]string{".gitattributes": "root", "go/.gitattributes": "generated"}), &gogithub.Response{}, nil).Times(2)
		expectBlob("root", "*.go text eol=lf\n", nil)

		provider.Start(ctx, 0)
		provider.Start(ctx, 0)

		matcher := provider.Gitattributes(repo)
		Expect(matcher).NotTo(BeNil())
		Expect(matcher.Excludes("go/api.pb.go")).To(BeTrue())
	})

	DescribeTable("keeps the last matcher read when a later poll can't read them",
		func(ctx SpecContext, later *gogithub.Tree, treeErr error) {
			expectMarvinYAML(nil)
			first := mockGithub.EXPECT().GetTree(gomock.Any(), gomock.Any(), defaultBranch, true).Return(generated, &gogithub.Response{}, nil)
			mockGithub.EXPECT().GetTree(gomock.Any(), gomock.Any(), defaultBranch, true).Return(later, &gogithub.Response{}, treeErr).After(first)
			expectBlob("changed", "", errors.New("boom"))

			provider.Start(ctx, 0)
			read := provider.Gitattributes(repo)
			Expect(read).NotTo(BeNil())

			provider.Start(ctx, 0)
			Expect(provider.Gitattributes(repo)).To(BeIdenticalTo(read))
		},
		Entry("the tree failing", nil, errors.New("boom")),
		Entry("the tree truncated", &gogithub.Tree{Truncated: new(true), Entries: generated.Entries}, nil),
		Entry("a changed file failing", blobs(map[string]string{"go/.gitattributes": "changed"}), nil),
	)

	DescribeTable("reads them and .marvin.yaml independently",
		func(ctx SpecContext, marvinYAMLErr, treeErr error) {
			expectMarvinYAML(marvinYAMLErr)
			mockGithub.EXPECT().GetTree(gomock.Any(), gomock.Any(), defaultBranch, true).Return(generated, &gogithub.Response{}, treeErr)

			provider.Start(ctx, 0)

			cfg, _ := provider.Get(&gogithub.PullRequestEvent{Repo: repo})
			Expect(cfg == nil).To(Equal(marvinYAMLErr != nil), "no .marvin.yaml read")
			// None read yet: every file counts
			Expect(provider.Gitattributes(repo) == nil).To(Equal(treeErr != nil), "no .gitattributes read")
		},
		Entry(".marvin.yaml failing", errors.New("boom"), nil),
		Entry("the .gitattributes files failing", nil, errors.New("boom")),
	)
})
