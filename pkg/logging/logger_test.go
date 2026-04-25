package logging_test

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gruntwork-io/boilerplate/pkg/logging"
)

func TestPackageDefaults(t *testing.T) {
	t.Parallel()

	if got := logging.CurrentLevel(); got != logging.LevelInfo {
		t.Fatalf("default level: got %v, want %v", got, logging.LevelInfo)
	}

	if got := logging.CurrentWriter(); got != os.Stdout {
		t.Fatalf("default writer: got %v, want os.Stdout", got)
	}
}

func TestLevelFiltering(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	lg := logging.New(&buf, logging.LevelInfo)

	lg.Debugf("debug-message")

	if buf.Len() != 0 {
		t.Fatalf("Debugf at LevelInfo wrote %q, want empty", buf.String())
	}

	lg.Infof("info-message")
	lg.Warnf("warn-message")
	lg.Errorf("error-message")

	out := buf.Bytes()
	for _, want := range []string{"info-message", "warn-message", "error-message"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Fatalf("output missing %q: %q", want, string(out))
		}
	}

	buf.Reset()
	lg.SetLevel(logging.LevelError)
	lg.Debugf("d")
	lg.Infof("i")
	lg.Warnf("w")

	if buf.Len() != 0 {
		t.Fatalf("levels below Error wrote output: %q", buf.String())
	}

	lg.Errorf("e")

	if !bytes.Contains(buf.Bytes(), []byte("e")) {
		t.Fatalf("Errorf at LevelError produced no output: %q", buf.String())
	}
}

func TestFormatPreservation(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	lg := logging.New(&buf, logging.LevelDebug)

	lg.Infof("hello %s", "world")

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

	lg := logging.New(cw, logging.LevelDebug)

	lg.Infof("a")
	lg.Infof("b")
	lg.Infof("c")

	if got := cw.writes.Load(); got != 3 {
		t.Fatalf("got %d Write calls, want 3", got)
	}
}

func TestSetWriterRedirectsOutput(t *testing.T) {
	t.Parallel()

	var first, second bytes.Buffer

	lg := logging.New(&first, logging.LevelDebug)

	lg.Infof("to-first")
	lg.SetWriter(&second)
	lg.Infof("to-second")

	if !bytes.Contains(first.Bytes(), []byte("to-first")) {
		t.Fatalf("first writer missing first record: %q", first.String())
	}

	if bytes.Contains(first.Bytes(), []byte("to-second")) {
		t.Fatalf("first writer received post-redirect record: %q", first.String())
	}

	if !bytes.Contains(second.Bytes(), []byte("to-second")) {
		t.Fatalf("second writer missing redirected record: %q", second.String())
	}
}

func TestSetWriterNilPanics(t *testing.T) {
	t.Parallel()

	lg := logging.New(io.Discard, logging.LevelInfo)

	defer func() {
		if recover() == nil {
			t.Fatal("SetWriter(nil) did not panic")
		}
	}()

	lg.SetWriter(nil)
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

	for l, want := range cases {
		if got := l.String(); got != want {
			t.Errorf("Level(%d).String() = %q, want %q", int(l), got, want)
		}
	}
}

func TestConcurrentUse(t *testing.T) {
	t.Parallel()

	lg := logging.New(io.Discard, logging.LevelDebug)

	var wg sync.WaitGroup

	for range 50 {
		wg.Add(2)

		go func() {
			defer wg.Done()

			lg.Infof("concurrent")
		}()

		go func() {
			defer wg.Done()

			lg.SetLevel(logging.LevelWarn)
		}()
	}

	wg.Wait()
}
