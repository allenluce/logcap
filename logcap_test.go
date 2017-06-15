package logcap

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LogCap", func() {
	Describe("generally", func() {
		var (
			logHook  *LogCap
			oldLevel logrus.Level
		)
		BeforeEach(func() {
			oldLevel = logrus.GetLevel()
			logrus.SetLevel(logrus.DebugLevel)
			logHook = NewLogHook()
			logHook.Start()
		})
		AfterEach(func() {
			logHook.Stop()
			logrus.SetLevel(oldLevel)
			Ω(logHook).Should(HaveNoLogs())
		})

		It("logs a warning message", func() {
			logrus.Warning("This is a warning")
			Ω(logHook).Should(HaveLogs("This is a warning"))
		})

		It("logs a debug message, with fields ", func() {
			logrus.WithFields(logrus.Fields{"time": "long time ago"}).
				Debug("This is for debugging")
			Ω(logHook).Should(HaveLogs("This is for debugging", logrus.Fields{
				"fields.time": "long time ago",
			}))
		})

		It("logs with fields", func() {
			logrus.WithFields(logrus.Fields{"time": "now"}).
				Info("first")
			logrus.WithFields(logrus.Fields{"time": "now"}).
				Info("second")
			Ω(logHook).Should(HaveLogs("first", "second", logrus.Fields{"time": "now"}))
		})
		It("logs more", func() {
			logrus.WithFields(logrus.Fields{"time": "then"}).
				Info("first")
			logrus.WithFields(logrus.Fields{"time": "some other time"}).
				Info("second")
			Ω(logHook).Should(HaveLogs("first", logrus.Fields{"time": "then"}, "second"))
		})
		It("ignores callers", func() {
			logHook.IgnoreCaller("logcap_test.go")
			logrus.Info("I need some pancakes")
			h := HaveLogs("I need some moolah")
			h.Match(logHook)
			Ω(h.FailureMessage(logHook)).Should(ContainSubstring(`leafnodes/runner.go`))
			Ω(logHook).Should(HaveLogs("I need some pancakes"))
		})
		It("composes with Gomega matchers", func() {
			logrus.Warning("This is a number: 23984329 yeah")
			Ω(logHook).Should(HaveLogs(MatchRegexp(`number: \d+ yeah`)))
		})
		It("signals failure on HaveNoLogs when it has logs", func() {
			logrus.Warning("This is a warning.")
			Ω(logHook).ShouldNot(HaveNoLogs())
			Ω(logHook).Should(HaveLogs("This is a warning."))
		})
		It("signals failure on HaveNoLogs when it has logs", func() {
			logrus.Warning("This is a warning.")
			h := HaveNoLogs()
			h.Match(logHook)
			Ω(h.FailureMessage(logHook)).Should(ContainSubstring("Expected no logs. Instead, got 1:"))
			Ω(logHook).Should(HaveLogs("This is a warning."))
		})
		It("signals negated failure on HaveNoLogs when it has logs", func() {
			logrus.Warning("This is a warning.")
			h := HaveNoLogs()
			h.Match(logHook)
			Ω(h.NegatedFailureMessage(logHook)).Should(ContainSubstring("Did not expect 0 logs"))
			Ω(h.NegatedFailureMessage(logHook)).Should(ContainSubstring("This is a warning"))
			Ω(logHook).Should(HaveLogs("This is a warning."))
		})
	})

	Describe("Display", func() {
		var (
			oldStderr, r *os.File
			logHook      *LogCap
			oldLevel     logrus.Level
		)
		BeforeEach(func() {
			oldLevel = logrus.GetLevel()
			logrus.SetLevel(logrus.DebugLevel)
			logHook = NewLogHook()
			logHook.Start()
			var w *os.File
			r, w, _ = os.Pipe()
			oldStderr = os.Stderr
			os.Stderr = w
		})
		AfterEach(func() {
			os.Stderr = oldStderr
			logHook.Stop()
			logrus.SetLevel(oldLevel)
			r.Close()
		})
		It("displays nothing by default", func(done Done) {
			logrus.Debug("This the debug log")
			logrus.Info("This the info log")
			logrus.Warning("This the warning log")
			logrus.Error("This the error log")
			os.Stderr.Close()
			stderr, _ := ioutil.ReadAll(r)
			Ω(string(stderr)).Should(Equal(""))
			close(done)
		})

		It("will display only debug", func(done Done) {
			logHook.Display(logrus.DebugLevel)
			logrus.Debug("This the debug log")
			logrus.Info("This the info log")
			logrus.Warning("This the warning log")
			logrus.Error("This the error log")
			os.Stderr.Close()
			stderr, _ := ioutil.ReadAll(r)
			Ω(string(stderr)).Should(ContainSubstring(`level=debug msg="This the debug log"`))
			Ω(string(stderr)).ShouldNot(ContainSubstring(`level=info msg="This the info log"`))
			Ω(string(stderr)).ShouldNot(ContainSubstring(`level=warning msg="This the warning log"`))
			Ω(string(stderr)).ShouldNot(ContainSubstring(`level=error msg="This the error log"`))
			close(done)
		})

		It("will display everything", func(done Done) {
			logHook.Display(logrus.DebugLevel, logrus.InfoLevel, logrus.WarnLevel, logrus.ErrorLevel)
			logrus.Debug("This the debug log")
			logrus.Info("This the info log")
			logrus.Warning("This the warning log")
			logrus.Error("This the error log")
			os.Stderr.Close()
			stderr, _ := ioutil.ReadAll(r)
			Ω(string(stderr)).Should(ContainSubstring(`level=debug msg="This the debug log"`))
			Ω(string(stderr)).Should(ContainSubstring(`level=info msg="This the info log"`))
			Ω(string(stderr)).Should(ContainSubstring(`level=warning msg="This the warning log"`))
			Ω(string(stderr)).Should(ContainSubstring(`level=error msg="This the error log"`))
			close(done)
		})
	})
	Describe("Local loggers", func() {
		It("captures local logs", func(done Done) {
			local := logrus.New()
			local.Level = logrus.DebugLevel
			localHook := NewLogHook(local)
			localHook.Start()
			defer localHook.Stop()
			local.Info("An info log")
			Ω(localHook).Should(HaveLogs("An info log"))
			close(done)
		})
	})
})

func TestLogcap(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logcap Suite")
}
