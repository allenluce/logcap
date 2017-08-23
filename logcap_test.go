package logcap

import (
	"io"
	"io/ioutil"
	"os"
	"sync"
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
		It("logs a warning message", func() {
			logrus.Warning("This is a warning")
			Ω(logHook).Should(HaveLogs("This is a warning"))
		})
		It("logs a couple warning messages", func() {
			logrus.Warning("This is a warning")
			logrus.Warning("This is another warning")
			Ω(logHook).Should(HaveLogs("This is a warning"))
			Ω(logHook).Should(HaveLogs("This is another warning"))
		})
		It("logs a debug message, with fields ", func() {
			logrus.WithFields(logrus.Fields{"time": "long time ago"}).
				Debug("This is for debugging")
			Ω(logHook).Should(HaveLogs("This is for debugging", logrus.Fields{
				"time": "long time ago",
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
		It("lists call site", func() {
			logrus.Info("I need some pancakes")
			h := HaveLogs("I need some moolah", time.Millisecond*100)
			h.Match(logHook)
			Ω(h.FailureMessage(logHook)).Should(ContainSubstring(`logcap_test.go`))
			Ω(logHook).Should(HaveLogs("I need some pancakes", time.Millisecond*100))
		})
		It("ignores call site", func() {
			logHook.IgnoreCaller("logcap_test.go")
			logrus.Info("I need some pancakes")
			h := HaveLogs("I need some moolah", time.Millisecond*100)
			h.Match(logHook)
			Ω(h.FailureMessage(logHook)).ShouldNot(ContainSubstring(`logcap_test.go`))
			Ω(logHook).Should(HaveLogs("I need some pancakes", time.Millisecond*100))
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
		It("signals no failure on HaveNoLogs with a level when it has no logs of that level", func() {
			logrus.Warning("This is a warning.")
			Ω(logHook).Should(HaveNoLogs(logrus.ErrorLevel))
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
	Describe("with internal buffer", func() {
		var (
			logHook *LogCap
		)
		BeforeEach(func() {
			logHook = NewLogHook()
			logHook.Start()
		})
		AfterEach(func() {
			logHook.Stop()
		})
		It("will error when internal buffer is full", func() {
			ps := newPipeSuck()
			for i := 0; i < 1000; i++ {
				logrus.Info("This the info log")
			}
			logrus.Info("This one is too much")
			ps.finish()
			Ω(ps.s).Should(Equal("Failed to fire hook: internal buffer full, use a higher entryCount value\n"))
		})
		It("will accept a higher buffer count", func() {
			ps := newPipeSuck()
			logHook = NewLogHook(10000)
			logHook.Start()
			for i := 0; i < 10000; i++ {
				logrus.Info("This the info log")
			}
			logrus.Info("This one is too much")
			ps.finish()
			Ω(ps.s).Should(Equal("Failed to fire hook: internal buffer full, use a higher entryCount value\n"))
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
		It("displays nothing by default", func() {
			logrus.Debug("This the debug log")
			logrus.Info("This the info log")
			logrus.Warning("This the warning log")
			logrus.Error("This the error log")
			os.Stderr.Close()
			stderr, _ := ioutil.ReadAll(r)
			Ω(string(stderr)).Should(Equal(""))
		})
		It("will display only debug", func() {
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
		})
		It("will display everything", func() {
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
		})
	})
	Describe("Local loggers", func() {
		var (
			local *logrus.Logger
			hook  *LogCap
		)
		BeforeEach(func() {
			local = logrus.New()
			local.Level = logrus.DebugLevel
			hook = NewLogHook(local)
			hook.Start()
		})
		AfterEach(func() {
			hook.Stop()
		})
		It("captures local logs", func() {
			local.Info("An info log")
			Ω(hook).Should(HaveLogs("An info log"))
		})
		It("captures local logs and takes a buffer count", func() {
			hook.Stop()
			hook = NewLogHook(local, 1)
			hook.Start()
			ps := newPipeSuck()
			local.Info("An info log")
			local.Info("Another info log")
			Ω(hook).Should(HaveLogs("An info log"))
			ps.finish()
			Ω(ps.s).Should(Equal("Failed to fire hook: internal buffer full, use a higher entryCount value\n"))
		})
	})
})

func TestLogcap(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logcap Suite")
}

// pipeSuck is a background pipe reader. It'll fill up s with the
// content read from the pipe. This is so we don't run into blocking
// writes when an os.Pipe fills up (which it does at 65536 bytes).
type pipeSuck struct {
	wg           sync.WaitGroup
	s            string
	r, oldStderr *os.File
}

// newPipeSuck captures stderr and starts reading it in the
// background.
func newPipeSuck() (ps *pipeSuck) {
	ps = &pipeSuck{}
	var w *os.File
	ps.r, w, _ = os.Pipe()
	ps.oldStderr = os.Stderr
	os.Stderr = w
	ps.read()
	return
}

// read sucks from the pipe in the background and fills p.s with the
// contents.
func (p *pipeSuck) read() {
	p.wg.Add(1)
	buf := make([]byte, 0, 4*1024)
	go func() {
		for {
			n, err := p.r.Read(buf[:cap(buf)])
			p.s += string(buf[:n])
			if err == io.EOF {
				p.wg.Done()
				return
			}
			Ω(err).Should(BeNil())
		}
	}()
}

// wait will wait until the pipe is closed and the string full of
// data. It then restores stderr to the old state.
func (p *pipeSuck) finish() {
	os.Stderr.Close()
	p.wg.Wait()
	os.Stderr = p.oldStderr
	p.r.Close()
}
