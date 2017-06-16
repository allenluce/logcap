package logcap

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

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

		It("logs a warning message", func(done Done) {
			logrus.Warning("This is a warning")
			Ω(logHook).Should(HaveLogs("This is a warning"))
			close(done)
		})

		It("logs a couple warning messages", func(done Done) {
			logrus.Warning("This is a warning")
			logrus.Warning("This is another warning")
			Ω(logHook).Should(HaveLogs("This is a warning"))
			Ω(logHook).Should(HaveLogs("This is another warning"))
			close(done)
		})

		It("logs a debug message, with fields ", func(done Done) {
			logrus.WithFields(logrus.Fields{"time": "long time ago"}).
				Debug("This is for debugging")
			Ω(logHook).Should(HaveLogs("This is for debugging", logrus.Fields{
				"time": "long time ago",
			}))
			close(done)
		})

		It("logs with fields", func(done Done) {
			logrus.WithFields(logrus.Fields{"time": "now"}).
				Info("first")
			logrus.WithFields(logrus.Fields{"time": "now"}).
				Info("second")
			Ω(logHook).Should(HaveLogs("first", "second", logrus.Fields{"time": "now"}))
			close(done)
		})
		It("logs more", func(done Done) {
			logrus.WithFields(logrus.Fields{"time": "then"}).
				Info("first")
			logrus.WithFields(logrus.Fields{"time": "some other time"}).
				Info("second")
			Ω(logHook).Should(HaveLogs("first", logrus.Fields{"time": "then"}, "second"))
			close(done)
		})
		It("lists call site", func(done Done) {
			logrus.Info("I need some pancakes")
			h := HaveLogs("I need some moolah", time.Millisecond*100)
			h.Match(logHook)
			Ω(h.FailureMessage(logHook)).Should(ContainSubstring(`logcap_test.go`))
			Ω(logHook).Should(HaveLogs("I need some pancakes", time.Millisecond*100))
			close(done)
		})
		It("ignores call site", func(done Done) {
			logHook.IgnoreCaller("logcap_test.go")
			logrus.Info("I need some pancakes")
			h := HaveLogs("I need some moolah", time.Millisecond*100)
			h.Match(logHook)
			Ω(h.FailureMessage(logHook)).ShouldNot(ContainSubstring(`logcap_test.go`))
			Ω(logHook).Should(HaveLogs("I need some pancakes", time.Millisecond*100))
			close(done)
		})
		It("composes with Gomega matchers", func(done Done) {
			logrus.Warning("This is a number: 23984329 yeah")
			Ω(logHook).Should(HaveLogs(MatchRegexp(`number: \d+ yeah`)))
			close(done)
		})
		It("signals failure on HaveNoLogs when it has logs", func(done Done) {
			logrus.Warning("This is a warning.")
			Ω(logHook).ShouldNot(HaveNoLogs())
			Ω(logHook).Should(HaveLogs("This is a warning."))
			close(done)
		})
		It("signals no failure on HaveNoLogs with a level when it has no logs of that level", func(done Done) {
			logrus.Warning("This is a warning.")
			Ω(logHook).Should(HaveNoLogs(logrus.ErrorLevel))
			Ω(logHook).Should(HaveLogs("This is a warning."))
			close(done)
		})
		It("signals failure on HaveNoLogs when it has logs", func(done Done) {
			logrus.Warning("This is a warning.")
			h := HaveNoLogs()
			h.Match(logHook)
			Ω(h.FailureMessage(logHook)).Should(ContainSubstring("Expected no logs. Instead, got 1:"))
			Ω(logHook).Should(HaveLogs("This is a warning."))
			close(done)
		})
		It("signals negated failure on HaveNoLogs when it has logs", func(done Done) {
			logrus.Warning("This is a warning.")
			h := HaveNoLogs()
			h.Match(logHook)
			Ω(h.NegatedFailureMessage(logHook)).Should(ContainSubstring("Did not expect 0 logs"))
			Ω(h.NegatedFailureMessage(logHook)).Should(ContainSubstring("This is a warning"))
			Ω(logHook).Should(HaveLogs("This is a warning."))
			close(done)
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
