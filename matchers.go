package logcap

import (
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/onsi/gomega/matchers"
	"github.com/onsi/gomega/types"
)

var logMut sync.Mutex

// Repeater allows for easy repeating of log matches. If you have something that's going to log
// 30 times, just use a repeater:
//
//    Ω(logHook).Should(HaveLogs(Repeater{MatchRegexp(`Log entry \d+`), 30}))
//
type Repeater struct {
	M interface{}
	N int
}

type markedEntry struct {
	*logrus.Entry
	matched bool
}

type logsMatch struct {
	Expected types.GomegaMatcher
	matched  bool
	Fields   *logrus.Fields
	Entry    *markedEntry
}

type logsMatcher struct {
	Matches     []*logsMatch
	NonMatching *markedEntry
	timeout     time.Duration
}

type noLogsMatcher struct {
	matchers.EqualMatcher
	level *logrus.Level
	found int
}

// HaveLogs takes a number of strings, Gomega matchers and/or
// logrus.Fields as arguments. It attempts to match logs based on the
// strings/matchers given. If a logrus.Fields{} argument is given, it
// is applied to all strings/matchers that precede it up until the
// previous logrus.Fields{} argument.  To succeed, all
// strings/matchers must match along with their associated
// logrus.Fields{} argument.
//
// This matches three distinct log entries, each with a {"task":
// "exiting"} field set:
//
//   HaveLogs("culler", "tallier", "summer", logrus.Fields{"task": "exiting"})
//
// This matches two log entries with empty field sets, and one
// without:
//
//   HaveLogs("alpha", "beta", logrus.Fields{}, "gamma", logrus.Fields{"big": "whoop"})
//
// An optional time.Duration added to the arguments will set the
// timeout for HaveLogs giving up on waiting for a match.
//
//   HaveLogs("summation", time.Seconds*100)
//
// The default timeout is two seconds.
func HaveLogs(args ...interface{}) types.GomegaMatcher {
	m := &logsMatcher{timeout: time.Second * 2}
	parseMatchArgs(args, m)
	return m
}

// HaveNoLogs is the inverse of HaveLogs(). It makes sure that there
// are no logs that haven't been matched already.
//
// If given a level, it'll only make sure no logs of that level have
// been seen. E.g.:
//
//  Ω(logHook).Should(HaveNoLogs(logrus.ErrorLevel))
func HaveNoLogs(level ...logrus.Level) types.GomegaMatcher {
	m := &noLogsMatcher{
		EqualMatcher: matchers.EqualMatcher{Expected: 0},
	}
	if len(level) > 0 {
		m.level = &level[0]
	}
	return m
}

// matcherOrEqual if given a matcher will use it. Otherwise it'll use
// the stock EqualMatcher.
func matcherOrEqual(arg interface{}) *logsMatch {
	switch arg := arg.(type) {
	case types.GomegaMatcher:
		return &logsMatch{Expected: arg}
	default:
		return &logsMatch{Expected: &matchers.EqualMatcher{Expected: arg}}
	}
}

func parseMatchArgs(args []interface{}, m *logsMatcher) {
	for _, arg := range args {
		switch arg := arg.(type) {
		case logrus.Fields: // Go backwards through matches and add this to its fields arg.
			for i := len(m.Matches) - 1; i >= 0; i-- {
				if m.Matches[i].Fields != nil { // Only if they don't have one already.
					break
				}
				m.Matches[i].Fields = &arg
			}
		case Repeater:
			for i := 0; i < arg.N; i++ {
				m.Matches = append(m.Matches, matcherOrEqual(arg.M))
			}
		case time.Duration:
			m.timeout = arg
		default:
			m.Matches = append(m.Matches, matcherOrEqual(arg))
		}
	}
}

func (m *logsMatcher) numMatches() (count int) {
	for _, match := range m.Matches {
		if !match.matched {
			count++
		}
	}
	return
}

// Match will cache log entries internally for future matching.
func (m *logsMatcher) Match(actual interface{}) (success bool, err error) {
	// Reset match indicators
	for _, match := range m.Matches {
		match.matched = false
	}
	hook := actual.(*LogCap)
	var entry *markedEntry

	cacheTop := 0
MainLoop:
	// Loop until all matched or timeout.
	for m.numMatches() > 0 {
		if cacheTop < len(hook.cache) { // Look at old logs first.
			entry = hook.cache[cacheTop]
			cacheTop++
		} else {
			select {
			case e := <-hook.entries:
				entry = &markedEntry{e, false}
			case <-time.After(m.timeout):
				return false, nil
			}
			hook.cache = append(hook.cache, entry)
			cacheTop++
		}
		// fmt.Printf("I see %s [%d] with %+v [%d]\n", entry.Message, len(hook.entries), entry.Data, m.numMatches())
	MatchLoop:
		for _, matchItem := range m.Matches {
			if matchItem.matched { // Already matched it.
				continue MatchLoop
			}
			doesMatch, err := matchItem.Expected.Match(entry.Message)
			if err != nil {
				return false, err
			}
			if !doesMatch {
				continue MatchLoop
			}
			if matchItem.Fields != nil {
				logMut.Lock()
				data := entry.Data
				for key, value := range *matchItem.Fields {
					var matcher types.GomegaMatcher
					switch value := value.(type) {
					case types.GomegaMatcher:
						matcher = value
					default:
						matcher = &matchers.EqualMatcher{Expected: value}
					}
					if _, ok := data[key]; !ok {
						logMut.Unlock()
						continue MatchLoop // Not there, no match.
					}
					matched, err := matcher.Match(data[key])
					if err != nil {
						logMut.Unlock()
						return false, err
					}
					if !matched {
						logMut.Unlock()
						continue MatchLoop
					}
				}
				logMut.Unlock()
			}
			matchItem.matched = true
			entry.matched = true
			matchItem.Entry = entry
			continue MainLoop
		}
		m.NonMatching = entry
		return false, nil
	}
	return true, nil
}

func (m *logsMatcher) baseMessage(matched bool) (message string) {
	for _, matchEntry := range m.Matches {
		if m.NonMatching != nil {
			if matchEntry.matched { // Don't report on things I know about
				continue
			}
			moMessage := m.NonMatching.Message
			moMessage += fmt.Sprintf("\n    logged at %s:%d\n", m.NonMatching.Data["file"], m.NonMatching.Data["line"])

			if len(m.NonMatching.Data) > 2 {
				data := logrus.Fields{}
				for k, v := range m.NonMatching.Data {
					if k == "file" || k == "line" {
						continue
					}
					data[k] = v
				}
				moMessage += fmt.Sprintf("    with %#v", data)
			}
			message += matchEntry.Expected.FailureMessage(moMessage) + "\n"
			if matchEntry.Fields != nil {
				message += fmt.Sprintf("        with %#v\n", matchEntry.Fields)
			}
			return
		}
		if matchEntry.matched == matched {
			if matched {
				message += matchEntry.Expected.NegatedFailureMessage(matchEntry.Entry.Message) + "\n"
				message += fmt.Sprintf("logged at %s:%d\n", matchEntry.Entry.Data["file"], matchEntry.Entry.Data["line"])
			} else {
				message += matchEntry.Expected.FailureMessage(nil) + "\n"
			}
			if matchEntry.Fields != nil {
				message += fmt.Sprintf("with %#v\n", matchEntry.Fields)
			}
		}
	}
	if m.NonMatching != nil {
		message += "Nonmatching log:\n"
		message += "  " + m.NonMatching.Message + "\n"
		message += fmt.Sprintf("    logged at %s:%d\n", m.NonMatching.Data["file"], m.NonMatching.Data["line"])
		if len(m.NonMatching.Data) > 2 {
			data := logrus.Fields{}
			for k, v := range m.NonMatching.Data {
				if k == "file" || k == "line" {
					continue
				}
				data[k] = v
			}
			message += fmt.Sprintf("    with %#v\n", data)
		}
	}
	return
}

func (m *logsMatcher) FailureMessage(actual interface{}) (message string) {
	return m.baseMessage(false)
}

func (m *logsMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return m.baseMessage(true)
}

func (m *noLogsMatcher) Match(actual interface{}) (success bool, err error) {
	hook := actual.(*LogCap)
	l := len(hook.entries) + len(hook.cache)
	var entry *markedEntry
	cacheTop := 0
	for i := 0; i < l; i++ {
		if cacheTop < len(hook.cache) {
			entry, hook.cache = hook.cache[0], hook.cache[1:]
			cacheTop++
		} else {
			e := <-hook.entries
			entry = &markedEntry{e, false}
			hook.cache = append(hook.cache, entry)
			cacheTop++
		}
		if m.level != nil && entry.Level != *m.level {
			continue
		}
		if !entry.matched { // Count non-matched entries
			m.found++
		}
	}
	return m.EqualMatcher.Match(m.found)
}

func (m *noLogsMatcher) FailureMessage(actual interface{}) (message string) {
	hook := actual.(*LogCap)
	message = fmt.Sprintf("Expected no logs. Instead, got %d:", m.found)
	for _, entry := range hook.cache {
		if m.level != nil && entry.Level != *m.level {
			continue
		}
		if entry.matched {
			continue
		}
		extra := ""
		if len(entry.Data) > 2 {
			data := logrus.Fields{}
			for k, v := range entry.Data {
				if k == "file" || k == "line" {
					continue
				}
				data[k] = v
			}
			extra = fmt.Sprintf(" (%v)", data)
		}
		message = message + fmt.Sprintf("\n  %s%s\n  logged at %s:%d", entry.Message, extra, entry.Data["file"], entry.Data["line"])
	}
	return
}

func (m *noLogsMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	hook := actual.(*LogCap)
	message = fmt.Sprintf("Did not expect 0 logs\n")
	for _, entry := range hook.cache {
		if m.level != nil && entry.Level != *m.level {
			continue
		}
		if entry.matched {
			continue
		}
		message = message + fmt.Sprintf("\n%s\n  logged at %s:%d", entry.Message, entry.Data["file"], entry.Data["line"])
	}
	return
}
