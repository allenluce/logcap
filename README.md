# Logcap &nbsp;[![Build Status](https://travis-ci.org/allenluce/logcap.svg?branch=master)](https://travis-ci.org/allenluce/logcap)&nbsp;[![GoDoc](https://godoc.org/github.com/allenluce/logcap?status.svg)](https://godoc.org/github.com/allenluce/logcap)

Package logcap is for capturing Logrus log entries and comparing them with
Gomega matchers.

It's designed to work within Ginkgo test suites:

    package main

    import (
    	"testing"

    	"github.com/allenluce/logcap"
    	"github.com/Sirupsen/logrus"
    	. "github.com/onsi/ginkgo"
    	. "github.com/onsi/gomega"
    )

    var _ = Describe("matches logs", func() {
    	It("matcher", func() {
    		logHook := logcap.NewLogHook()
    		logHook.Start()
    		defer logHook.Stop()
    		logrus.Info("This is a log entry")
    		Ω(logHook).Should(logcap.HaveLogs("This is a log entry"))
    	})
    })

    func TestThings(t *testing.T) {
    	RegisterFailHandler(Fail)
    	RunSpecs(t, "Logcap Suite")
    }

## Usage

#### func  HaveLogs

```go
func HaveLogs(args ...interface{}) types.GomegaMatcher
```
HaveLogs takes a number of strings, Gomega matchers and/or logrus.Fields as
arguments. It attempts to match logs based on the strings/matchers given. If a
logrus.Fields{} argument is given, it is applied to all strings/matchers that
precede it up until the previous logrus.Fields{} argument. To succeed, all
strings/matchers must match along with their associated logrus.Fields{}
argument.

This matches three distinct log entries, each with a {"task": "exiting"} field
set:

    HaveLogs("culler", "tallier", "summer", logrus.Fields{"task": "exiting"})

This matches two log entries with empty field sets, and one without:

    HaveLogs("alpha", "beta", logrus.Fields{}, "gamma", logrus.Fields{"big": "whoop"})

An optional time.Duration added to the arguments will set the timeout for
HaveLogs giving up on waiting for a match.

    HaveLogs("summation", time.Seconds*100)

The default timeout is two seconds.

#### func  HaveNoLogs

```go
func HaveNoLogs() types.GomegaMatcher
```
HaveNoLogs is the inverse of HaveLogs(). It makes sure that there are no logs
that haven't been matched already.

#### type LogCap

```go
type LogCap struct {
}
```

Logcap is the base type that implements a Logrus hook.

#### func  NewLogHook

```go
func NewLogHook(l ...*logrus.Logger) *LogCap
```
NewLogHook creates a new LogCap hook. If supplied with a logger, it'll attach
the hook to that logger. Otherwise it'll attach to the logrus.StandardLogger()

#### func (*LogCap) Display

```go
func (hook *LogCap) Display(levels ...logrus.Level)
```
Display registers log levels to display to os.Stderr. Normally, all output is
suppressed from the logs. Call Display with a list of levels (or call it
multiple times) to print logs for that level.

#### func (*LogCap) Fire

```go
func (hook *LogCap) Fire(entry *logrus.Entry) error
```
Fire is required to implement the Logrus hook interface

#### func (*LogCap) IgnoreCaller

```go
func (hook *LogCap) IgnoreCaller(s string)
```
IgnoreCaller registers filenames (or parts of filenames) that shouldn't be
included when tracing the call stack back to find the file and line number to
display with log failures. It defaults to []string{"Sirupsen/logrus"} to elide
all files in the Logrus library. If you have your logging module in a subsidiary
file, add it with IgnoreCaller() so the original call site will be displayed.

#### func (*LogCap) Levels

```go
func (hook *LogCap) Levels() []logrus.Level
```
Levels is required to implement the Logrus hook interface

#### func (*LogCap) Start

```go
func (hook *LogCap) Start()
```
Start starts the hook, attaching it to the given logger.

#### func (*LogCap) Stop

```go
func (hook *LogCap) Stop()
```
Stop stops the hook and removes ALL hooks from the logger.

#### type Repeater

```go
type Repeater struct {
	M interface{}
	N int
}
```

Repeater allows for easy repeating of log matches. If you have something that's
going to log 30 times, just use a repeater:

    Ω(logHook).Should(HaveLogs(Repeater{MatchRegexp(`Log entry \d+`), 30}))
