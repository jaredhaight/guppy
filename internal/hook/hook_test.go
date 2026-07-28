package hook

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("hook commands here use POSIX shell syntax")
	}
}

func TestRunSequential(t *testing.T) {
	skipOnWindows(t)

	var out bytes.Buffer
	r := &Runner{Stdout: &out, Stderr: &out}

	if err := r.Run([]string{"echo one", "echo two", "echo three"}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if got := out.String(); got != "one\ntwo\nthree\n" {
		t.Errorf("Run() output = %q, want commands run in order", got)
	}
}

func TestRunStopsAtFirstFailure(t *testing.T) {
	skipOnWindows(t)

	var out bytes.Buffer
	r := &Runner{Stdout: &out, Stderr: &out}

	err := r.Run([]string{"echo first", "exit 3", "echo never"})
	if err == nil {
		t.Fatal("Run() succeeded, want an error from the failing command")
	}
	if !strings.Contains(err.Error(), "exit 3") {
		t.Errorf("Run() error = %v, want it to name the failing command", err)
	}
	if strings.Contains(out.String(), "never") {
		t.Error("Run() kept going after a failure")
	}
}

func TestRunEmpty(t *testing.T) {
	r := &Runner{}
	if err := r.Run(nil); err != nil {
		t.Errorf("Run(nil) error: %v", err)
	}
}

func TestRunEnv(t *testing.T) {
	skipOnWindows(t)

	var out bytes.Buffer
	r := &Runner{
		Stdout: &out,
		Stderr: &out,
		Env:    map[string]string{"GUPPY_APP": "ripgrep", "GUPPY_VERSION": "14.1.1"},
	}

	if err := r.Run([]string{`echo "$GUPPY_APP $GUPPY_VERSION"`}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "ripgrep 14.1.1" {
		t.Errorf("Run() output = %q, want the hook env visible to the command", got)
	}
}

func TestRunEnvInheritsParent(t *testing.T) {
	skipOnWindows(t)

	t.Setenv("GUPPY_TEST_PARENT", "inherited")

	var out bytes.Buffer
	r := &Runner{Stdout: &out, Stderr: &out, Env: map[string]string{"GUPPY_APP": "x"}}

	if err := r.Run([]string{`echo "$GUPPY_TEST_PARENT"`}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "inherited" {
		t.Errorf("Run() output = %q, want the parent environment preserved", got)
	}
}

func TestRunDir(t *testing.T) {
	skipOnWindows(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker"), []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write marker: %v", err)
	}

	var out bytes.Buffer
	r := &Runner{Dir: dir, Stdout: &out, Stderr: &out}

	if err := r.Run([]string{"cat marker"}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if got := out.String(); got != "x" {
		t.Errorf("Run() output = %q, want the command to run in Dir", got)
	}
}

// Shell metacharacters in a hook are the user's own config and must work.
func TestRunSupportsShellSyntax(t *testing.T) {
	skipOnWindows(t)

	var out bytes.Buffer
	r := &Runner{Stdout: &out, Stderr: &out}

	if err := r.Run([]string{"echo a && echo b | tr 'b' 'c'"}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if got := out.String(); got != "a\nc\n" {
		t.Errorf("Run() output = %q, want pipes and && to work", got)
	}
}

// Values guppy passes in are environment variables, never interpolated into
// the command text, so shell syntax inside them stays inert.
func TestRunEnvValuesAreNotInterpreted(t *testing.T) {
	skipOnWindows(t)

	canary := filepath.Join(t.TempDir(), "pwned")

	var out bytes.Buffer
	r := &Runner{
		Stdout: &out,
		Stderr: &out,
		Env:    map[string]string{"GUPPY_VERSION": "1.0.0; touch " + canary},
	}

	if err := r.Run([]string{`echo "version is $GUPPY_VERSION"`}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if _, err := os.Stat(canary); err == nil {
		t.Error("a command substitution in an env value was executed")
	}
	if got := strings.TrimSpace(out.String()); got != "version is 1.0.0; touch "+canary {
		t.Errorf("Run() output = %q, want the value passed through literally", got)
	}
}
