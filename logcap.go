/*
Package logcap is for capturing Logrus log entries and comparing them
with Gomega matchers.

It's designed to work within Ginkgo test suites:

  package main

  import (
  	"testing"

  	"github.com/Sirupsen/logrus"
  	. "github.com/onsi/ginkgo"
  	. "github.com/onsi/gomega"
  )

  var _ = Describe("matches logs", func() {
  	It("matcher", func() {
  		logHook := NewLogHook()
  		logHook.Start()
  		defer logHook.Stop()
  		logrus.Info("This is a log entry")
  		Ω(logHook).Should(HaveLogs("This is a log entry"))
  	})
  })

  func TestThings(t *testing.T) {
  	RegisterFailHandler(Fail)
  	RunSpecs(t, "Logcap Suite")
  }

*/
package logcap

import (
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
)

// Logcap is the base type that implements a Logrus hook.
type LogCap struct {
	oldOut  io.Writer
	entries chan *logrus.Entry
	ignores []string
	logger  *logrus.Logger
	display map[logrus.Level]interface{}
	cache   []*markedEntry
}

// Display registers log levels to display to os.Stderr. Normally, all
// output is suppressed from the logs. Call Display with a list of
// levels (or call it multiple times) to print logs for that level.
func (hook *LogCap) Display(levels ...logrus.Level) {
	for _, level := range levels {
		hook.display[level] = true
	}
}

// IgnoreCaller registers filenames (or parts of filenames) that
// shouldn't be included when tracing the call stack back to find the
// file and line number to display with log failures. It defaults to
// []string{"Sirupsen/logrus"} to elide all files in the Logrus
// library. If you have your logging module in a subsidiary file, add
// it with IgnoreCaller() so the original call site will be displayed.
func (hook *LogCap) IgnoreCaller(s string) {
	hook.ignores = append(hook.ignores, s)
}

// Fire is required to implement the Logrus hook interface
func (hook *LogCap) Fire(entry *logrus.Entry) error {
EntryLoop:
	for i := 1; ; i++ {
		if _, file, line, ok := runtime.Caller(i); ok {
			for _, substring := range hook.ignores {
				if strings.Contains(file, substring) {
					continue EntryLoop
				}
			}
			entry.Data["file"] = file
			entry.Data["line"] = line
		}
		break
	}
	entry.Logger.Out = ioutil.Discard
	if _, ok := hook.display[entry.Level]; ok {
		entry.Logger.Out = os.Stderr
	}
	hook.entries <- entry
	return nil
}

// Levels is required to implement the Logrus hook interface
func (hook *LogCap) Levels() []logrus.Level {
	return logrus.AllLevels
}

var hookMutex sync.Mutex

// Start starts the hook, attaching it to the given logger.
func (hook *LogCap) Start() {
	hookMutex.Lock()
	defer hookMutex.Unlock()
	hook.logger.Hooks.Add(hook)
	hook.oldOut = hook.logger.Out
}

// Stop stops the hook and removes ALL hooks from the logger.
func (hook *LogCap) Stop() {
	hookMutex.Lock()
	defer hookMutex.Unlock()
	hook.logger.Out = hook.oldOut
	hook.logger.Hooks = make(logrus.LevelHooks) // Remove any hooks
}

// NewLogHook creates a new LogCap hook. If supplied with a logger,
// it'll attach the hook to that logger. Otherwise it'll attach to the
// logrus.StandardLogger()
func NewLogHook(l ...*logrus.Logger) *LogCap {
	var logger *logrus.Logger
	if len(l) == 0 {
		logger = logrus.StandardLogger()
	} else {
		logger = l[0]
	}
	logger.Hooks = make(logrus.LevelHooks)
	hook := new(LogCap)
	hook.logger = logger
	hook.entries = make(chan *logrus.Entry, 100)
	hook.display = make(map[logrus.Level]interface{})
	hook.ignores = []string{"Sirupsen/logrus"} // trim Logrus callers from chain
	return hook
}
