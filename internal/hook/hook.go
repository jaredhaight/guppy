// Package hook runs the shell commands an app config attaches to an install.
package hook

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
)

// Runner runs hook commands.
type Runner struct {
	// Dir is the working directory commands run in.
	Dir string
	// Env holds extra environment variables layered over the parent process's.
	Env map[string]string
	// Stdout and Stderr receive the commands' output. Defaults to the
	// process's own when nil.
	Stdout io.Writer
	Stderr io.Writer
}

// Run executes each command in order, stopping at the first failure.
//
// Commands go through the system shell so that pipes, redirection and && work
// the way they do when typed at a prompt — the point of the feature is to
// replace the wrapper scripts users would otherwise write.
//
// Running a shell is the feature, not an oversight: the command text comes
// from the user's own config file, which already names a URL guppy will
// download and execute. Untrusted values from a release — versions, filenames,
// paths — are handed to the command as environment variables and never
// interpolated into the command text, so they cannot become shell syntax.
// Callers must preserve that: build Env, never build the command string.
//
// ponytail: "sh" rather than $SHELL. Predictable beats configurable here, and
// hooks that depend on fish or zsh syntax can call those explicitly.
func (r *Runner) Run(commands []string) error {
	for _, command := range commands {
		if err := r.runOne(command); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) runOne(command string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}

	cmd.Dir = r.Dir
	cmd.Env = r.environ()

	cmd.Stdout = r.Stdout
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = r.Stderr
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %s: %w", command, err)
	}
	return nil
}

func (r *Runner) environ() []string {
	if len(r.Env) == 0 {
		return nil // inherit the parent environment unchanged
	}

	keys := make([]string, 0, len(r.Env))
	for key := range r.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys) // stable ordering keeps test output predictable

	env := os.Environ()
	for _, key := range keys {
		env = append(env, key+"="+r.Env[key])
	}
	return env
}
