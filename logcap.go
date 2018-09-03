/*
Package logcap is for capturing Logrus log entries and comparing them
with Gomega matchers.

It's designed to work within Ginkgo test suites:

  package main

  import (
  	"testing"

  	"github.com/sirupsen/logrus"
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
	"errors"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// Logcap is the base type that implements a Logrus hook.
type LogCap struct {
	oldOut   io.Writer
	entries  chan *logrus.Entry
	ignores  []string
	logger   *logrus.Logger
	display  map[logrus.Level]interface{}
	cache    []*markedEntry
	cacheMut sync.Mutex
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
// []string{"sirupsen/logrus"} to elide all files in the Logrus
// library. If you have your logging module in a subsidiary file, add
// it with IgnoreCaller() so the original call site will be displayed.
func (hook *LogCap) IgnoreCaller(s string) {
	hook.ignores = append(hook.ignores, s)
}

var outMutex sync.Mutex

// Fire is required to implement the Logrus hook interface
func (hook *LogCap) Fire(e *logrus.Entry) error {
	entry := logrus.Entry{
		Logger:  e.Logger,
		Time:    e.Time,
		Level:   e.Level,
		Message: e.Message,
		Buffer:  e.Buffer,
		Data:    logrus.Fields{},
	}
	// Copy data into new struct
	for k, v := range e.Data {
		entry.Data[k] = v
	}

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
	outMutex.Lock()
	e.Logger.Out = ioutil.Discard
	if _, ok := hook.display[entry.Level]; ok {
		e.Logger.Out = os.Stderr
	}
	outMutex.Unlock()
	select {
	case hook.entries <- &entry:
	default:
		return errors.New("internal buffer full, use a higher entryCount value")
	}
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

// NewLogHook creates a new LogCap hook. If one of the supplied
// arguments is a *logrus.Logger, it'll attach the hook to that
// logger. Otherwise it'll attach to the logrus.StandardLogger(). If
// one of the supplied arguments is an int, it will be used as the
// entryCount, the number of logs that can be held in the internal
// buffer. If that limit is reached, logrus will error.
func NewLogHook(args ...interface{}) *LogCap {
	logger := logrus.StandardLogger()
	entryCount := 1000

	for _, arg := range args {
		switch a := arg.(type) {
		case *logrus.Logger:
			logger = a
		case int:
			entryCount = a
		}
	}

	logger.Hooks = make(logrus.LevelHooks)
	return &LogCap{
		logger:  logger,
		entries: make(chan *logrus.Entry, entryCount),
		display: make(map[logrus.Level]interface{}),
		ignores: []string{"sirupsen/logrus"}, // trim Logrus callers from chain
	}
}
