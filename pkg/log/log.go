// Copyright 2020-2021 The OS-NVR Authors.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; version 2.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package log

// API inspired by zerolog https://github.com/rs/zerolog

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Level defines log level.
type Level uint8

// Logging constants, matching ffmpeg.
const (
	LevelError   Level = 16
	LevelWarning Level = 24
	LevelInfo    Level = 32
	LevelDebug   Level = 48
)

// Event defines log event.
type Event struct {
	level   Level
	time    time.Time // Timestamp.
	src     string    // Source.
	monitor string    // Source monitor id.

	logger *Logger
}

// Log defines log entry.
type Log struct {
	Level   Level
	Time    time.Time // Timestamp.
	Msg     string    // Message
	Src     string    // Source.
	Monitor string    // Source monitor id.
}

// Src sets event source.
func (e *Event) Src(source string) *Event {
	e.src = source
	return e
}

// Monitor sets event monitor.
func (e *Event) Monitor(monitorID string) *Event {
	e.monitor = monitorID
	return e
}

// Msg sends the *Event with msg added as the message field.
func (e *Event) Msg(msg string) {
	log := Log{
		Level:   e.level,
		Time:    e.time,
		Msg:     msg,
		Src:     e.src,
		Monitor: e.monitor,
	}

	e.logger.feed <- log
}

// Msgf sends the event with formatted msg added as the message field.
func (e *Event) Msgf(format string, v ...interface{}) {
	e.Msg(fmt.Sprintf(format, v...))
}

// Feed defines feed of logs.
type Feed <-chan Log
type logFeed chan Log

// Logger logs.
type Logger struct {
	feed  logFeed      // feed of logs.
	sub   chan logFeed // subscribe requests.
	unsub chan logFeed // unsubscribe requests.
}

// NewLogger starts and returns Logger.
func NewLogger() *Logger {
	return &Logger{
		feed:  make(logFeed),
		sub:   make(chan logFeed),
		unsub: make(chan logFeed),
	}
}

// NewMockLogger used for testing.
func NewMockLogger() *Logger {
	return &Logger{
		feed:  make(logFeed),
		sub:   make(chan logFeed),
		unsub: make(chan logFeed),
	}
}

// Start logger.
func (l *Logger) Start(ctx context.Context) {
	subs := map[logFeed]struct{}{}
	for {
		select {
		case <-ctx.Done():
			return

		case ch := <-l.sub:
			subs[ch] = struct{}{}

		case ch := <-l.unsub:
			close(ch)
			delete(subs, ch)

		case msg := <-l.feed:
			for ch := range subs {
				ch <- msg
			}
		}
	}
}

// CancelFunc cancels log feed subsciption.
type CancelFunc func()

// Subscribe returns a new chan with log feed and a CancelFunc.
func (l *Logger) Subscribe() (<-chan Log, CancelFunc) {
	feed := make(logFeed)
	l.sub <- feed

	cancel := func() {
		l.unSubscribe(feed)
	}
	return feed, cancel
}

func (l *Logger) unSubscribe(feed logFeed) {
	// Read feed until unsub request is accepted.
	for {
		select {
		case l.unsub <- feed:
			return
		case <-feed:
		}
	}
}

// LogToStdout prints log feed to Stdout.
func (l *Logger) LogToStdout(ctx context.Context) {
	feed, cancel := l.Subscribe()
	defer cancel()
	for {
		select {
		case log := <-feed:
			printLog(log)
		case <-ctx.Done():
			return
		}
	}
}

func printLog(log Log) {
	var output string

	switch log.Level {
	case LevelError:
		output += "[ERROR] "
	case LevelWarning:
		output += "[WARNING] "
	case LevelInfo:
		output += "[INFO] "
	case LevelDebug:
		output += "[DEBUG] "
	}

	if log.Monitor != "" {
		output += log.Monitor + ": "
	}
	if log.Src != "" {
		output += strings.Title(log.Src) + ": "
	}

	output += log.Msg
	fmt.Println(output)
}

// Error starts a new message with error level.
// You must call Msg on the returned event in order to send the event.
func (l *Logger) Error() *Event {
	return &Event{
		level:  LevelError,
		time:   time.Now(),
		logger: l,
	}
}

// Warn starts a new message with warn level.
// You must call Msg on the returned event in order to send the event.
func (l *Logger) Warn() *Event {
	return &Event{
		level:  LevelWarning,
		time:   time.Now(),
		logger: l,
	}
}

// Info starts a new message with info level.
// You must call Msg on the returned event in order to send the event.
func (l *Logger) Info() *Event {
	return &Event{
		level:  LevelInfo,
		time:   time.Now(),
		logger: l,
	}
}

// Debug starts a new message with debug level.
// You must call Msg on the returned event in order to send the event.
func (l *Logger) Debug() *Event {
	return &Event{
		level:  LevelDebug,
		time:   time.Now(),
		logger: l,
	}
}
