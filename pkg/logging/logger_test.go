package logging_test

import (
	"bytes"
	"io"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gruntwork-io/boilerplate/pkg/logging"
)

func TestLevelFiltering(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	l := logging.New(&buf, logging.LevelInfo)

	l.Debugf("debug-message")

	if buf.Len() != 0 {
		t.Fatalf("Debugf at LevelInfo wrote %q, want empty", buf.String())
	}

	l.Infof("info-message")
	l.Warnf("warn-message")
	l.Errorf("error-message")

	out := buf.Bytes()
	for _, want := range []string{"info-message", "warn-message", "error-message"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Fatalf("output missing %q: %q", want, string(out))
		}
	}
}

func TestErrorOnlyAtLevelError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	l := logging.New(&buf, logging.LevelError)

	l.Debugf("d")
	l.Infof("i")
	l.Warnf("w")

	if buf.Len() != 0 {
		t.Fatalf("levels below Error wrote output: %q", buf.String())
	}

	l.Errorf("e")

	if !bytes.Contains(buf.Bytes(), []byte("e")) {
		t.Fatalf("Errorf at LevelError produced no output: %q", buf.String())
	}
}

func TestFormatPreservation(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	l := logging.New(&buf, logging.LevelDebug)

	l.Infof("hello %s", "world")

	pattern := regexp.MustCompile(`^\[boilerplate\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} hello world\n$`)
	if !pattern.Match(buf.Bytes()) {
		t.Fatalf("output %q does not match expected format", buf.String())
	}
}

type countingWriter struct {
	w      io.Writer
	writes atomic.Int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.writes.Add(1)
	return c.w.Write(p)
}

func TestOneRecordPerCall(t *testing.T) {
	t.Parallel()

	cw := &countingWriter{w: io.Discard}

	l := logging.New(cw, logging.LevelDebug)

	l.Infof("a")
	l.Infof("b")
	l.Infof("c")

	if got := cw.writes.Load(); got != 3 {
		t.Fatalf("got %d Write calls, want 3", got)
	}
}

func TestNewNilWriterPanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("New(nil, ...) did not panic")
		}
	}()

	_ = logging.New(nil, logging.LevelInfo)
}

func TestDiscardDropsEverything(t *testing.T) {
	t.Parallel()

	l := logging.Discard()

	l.Debugf("d")
	l.Infof("i")
	l.Warnf("w")
	l.Errorf("e")
}

func TestLevelString(t *testing.T) {
	t.Parallel()

	cases := map[logging.Level]string{
		logging.LevelDebug: "debug",
		logging.LevelInfo:  "info",
		logging.LevelWarn:  "warn",
		logging.LevelError: "error",
		logging.Level(99):  "level(99)",
	}

	for lvl, want := range cases {
		if got := lvl.String(); got != want {
			t.Errorf("Level(%d).String() = %q, want %q", int(lvl), got, want)
		}
	}
}

func TestConcurrentEmit(t *testing.T) {
	t.Parallel()

	l := logging.New(io.Discard, logging.LevelDebug)

	var wg sync.WaitGroup

	for range 50 {
		wg.Go(func() {
			l.Infof("concurrent")
		})
	}

	wg.Wait()
}
