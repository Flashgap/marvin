package marvin_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/Flashgap/marvin/internal/microservice/marvin"
	"github.com/Flashgap/marvin/internal/migrations"
	"github.com/Flashgap/marvin/internal/service/github"
	mock_jira "github.com/Flashgap/marvin/internal/service/jira/mock"
	mock_marvin "github.com/Flashgap/marvin/internal/service/marvin/mock"
	mock_database "github.com/Flashgap/marvin/pkg/database/mock"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
)

var _ = Describe("Services database wiring", func() {
	It("leaves Services.DB nil when DB_HOST is not set", func(ctx SpecContext) {
		cfg := marvin.Config{}
		mockCtrl := gomock.NewController(GinkgoT())
		services := &marvin.Services{
			MarvinService: mock_marvin.NewMockService(mockCtrl),
			GithubService: github.NewService(mock_github.NewMockClient(mockCtrl)),
			JiraService:   mock_jira.NewMockService(mockCtrl),
		}
		_, err := marvin.NewServer(ctx, &cfg, services)
		Expect(err).ToNot(HaveOccurred())
		Expect(services.DB).To(BeNil())
	})

	It("preserves a pre-injected DB client during initialize", func(ctx SpecContext) {
		cfg := marvin.Config{}
		mockCtrl := gomock.NewController(GinkgoT())
		dbMock := mock_database.NewMockClient(mockCtrl)
		dbMock.EXPECT().Migrate(gomock.Any(), migrations.FS).Return(nil)
		services := &marvin.Services{
			DB:            dbMock,
			MarvinService: mock_marvin.NewMockService(mockCtrl),
			GithubService: github.NewService(mock_github.NewMockClient(mockCtrl)),
			JiraService:   mock_jira.NewMockService(mockCtrl),
		}
		_, err := marvin.NewServer(ctx, &cfg, services)
		Expect(err).ToNot(HaveOccurred())
		Expect(services.DB).To(BeIdenticalTo(dbMock))
	})

	It("applies the migrations once, however many services use the database", func(ctx SpecContext) {
		cfg := marvin.Config{}
		cfg.StandupChannelID = "C1"
		mockCtrl := gomock.NewController(GinkgoT())
		dbMock := mock_database.NewMockClient(mockCtrl)
		dbMock.EXPECT().Migrate(gomock.Any(), migrations.FS).Return(nil).Times(1)
		services := &marvin.Services{
			DB:            dbMock,
			MarvinService: mock_marvin.NewMockService(mockCtrl),
			GithubService: github.NewService(mock_github.NewMockClient(mockCtrl)),
			JiraService:   mock_jira.NewMockService(mockCtrl),
		}
		_, err := marvin.NewServer(ctx, &cfg, services)
		Expect(err).ToNot(HaveOccurred())
		Expect(services.LockService).ToNot(BeNil())
		Expect(services.StandupService).ToNot(BeNil())
	})

	It("fails to start when the migrations fail", func(ctx SpecContext) {
		cfg := marvin.Config{}
		mockCtrl := gomock.NewController(GinkgoT())
		migrateErr := errors.New("migrate: applying 20261007000000_init_standup.sql")
		dbMock := mock_database.NewMockClient(mockCtrl)
		dbMock.EXPECT().Migrate(gomock.Any(), migrations.FS).Return(migrateErr)
		services := &marvin.Services{
			DB:            dbMock,
			MarvinService: mock_marvin.NewMockService(mockCtrl),
			GithubService: github.NewService(mock_github.NewMockClient(mockCtrl)),
			JiraService:   mock_jira.NewMockService(mockCtrl),
		}
		_, err := marvin.NewServer(ctx, &cfg, services)
		Expect(err).To(MatchError(migrateErr))
	})

	It("leaves the standup service nil when no standup channel is set", func(ctx SpecContext) {
		cfg := marvin.Config{}
		mockCtrl := gomock.NewController(GinkgoT())
		dbMock := mock_database.NewMockClient(mockCtrl)
		dbMock.EXPECT().Migrate(gomock.Any(), migrations.FS).Return(nil)
		services := &marvin.Services{
			DB:            dbMock,
			MarvinService: mock_marvin.NewMockService(mockCtrl),
			GithubService: github.NewService(mock_github.NewMockClient(mockCtrl)),
			JiraService:   mock_jira.NewMockService(mockCtrl),
		}
		_, err := marvin.NewServer(ctx, &cfg, services)
		Expect(err).ToNot(HaveOccurred())
		Expect(services.StandupService).To(BeNil())
	})

	It("leaves the standup service nil without a database, even with a channel set", func(ctx SpecContext) {
		cfg := marvin.Config{}
		cfg.StandupChannelID = "C1"
		mockCtrl := gomock.NewController(GinkgoT())
		services := &marvin.Services{
			MarvinService: mock_marvin.NewMockService(mockCtrl),
			GithubService: github.NewService(mock_github.NewMockClient(mockCtrl)),
			JiraService:   mock_jira.NewMockService(mockCtrl),
		}
		_, err := marvin.NewServer(ctx, &cfg, services)
		Expect(err).ToNot(HaveOccurred())
		Expect(services.StandupService).To(BeNil())
	})

})
