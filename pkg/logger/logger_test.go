package logger_test

import (
	"testing"

	"github.com/Flashgap/logrus"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/pkg/logger"
)

func TestInstrumentation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logger pkg test suite")
}

var _ = BeforeSuite(func() {
	logger.Init(logger.Config{
		DefaultLogLevel: logrus.InfoLevel,
		IsDevEnv:        false,
	})
})

type hook struct {
	entries []*logrus.Entry
}

func (h *hook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.CriticalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
		logrus.TraceLevel,
	}
}

func (h *hook) Fire(entry *logrus.Entry) error {
	h.entries = append(h.entries, entry)
	return nil
}

var _ = Describe("logger", Serial, func() { // Serial, only one stdout
	Context("WithContext", func() {
		It("should log using default level", func(ctx SpecContext) {
			log := logger.WithContext(ctx)

			testHook := &hook{}
			log.Logger.AddHook(testHook)
			log.Debug("hello")
			log.Info("world")

			Expect(testHook.entries).To(HaveLen(1))
			Expect(testHook.entries[0].Message).To(Equal("world"))
		})
	})
})
