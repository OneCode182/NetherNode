// Package compose runs docker compose and docker inspect on behalf of the
// nethernode CLI lifecycle commands (start, stop, restart, status).
package compose

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ExecFunc runs an external command and returns its combined stdout+stderr
// output. The default implementation wraps os/exec.
type ExecFunc func(ctx context.Context, name string, args ...string) (string, error)

// Runner drives `docker compose` for a given compose file/project directory,
// plus `docker inspect` for container state checks.
type Runner struct {
	// ComposeFile is passed as `docker compose -f <ComposeFile>` when set.
	ComposeFile string
	// ProjectDir is passed as `docker compose --project-directory <ProjectDir>` when set.
	ProjectDir string
	// DryRun, when true, makes Up/Down/PS return the command line that would
	// run instead of executing it, and makes ContainerRunning a no-op.
	DryRun bool
	// Exec runs the underlying command. Defaults to os/exec when nil.
	Exec ExecFunc
}

// defaultExec runs name/args via os/exec, capturing combined output.
func defaultExec(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func (r *Runner) exec(ctx context.Context, name string, args ...string) (string, error) {
	fn := r.Exec
	if fn == nil {
		fn = defaultExec
	}
	return fn(ctx, name, args...)
}

// composeArgs builds `compose [-f ComposeFile] [--project-directory ProjectDir] <sub...>`.
func (r *Runner) composeArgs(sub ...string) []string {
	args := []string{"compose"}
	if r.ComposeFile != "" {
		args = append(args, "-f", r.ComposeFile)
	}
	if r.ProjectDir != "" {
		args = append(args, "--project-directory", r.ProjectDir)
	}
	args = append(args, sub...)
	return args
}

// commandLine renders name+args as a shell-like string for DryRun output.
func commandLine(name string, args []string) string {
	return strings.Join(append([]string{name}, args...), " ")
}

func (r *Runner) runCompose(ctx context.Context, sub ...string) (string, error) {
	args := r.composeArgs(sub...)
	if r.DryRun {
		return commandLine("docker", args), nil
	}
	return r.exec(ctx, "docker", args...)
}

// Run executes an arbitrary command (e.g. "df") through the same
// hookable/DryRun-aware exec path Up/Down/PS use, so callers needing a
// one-off system command (disk checks, etc.) don't have to reach past the
// Runner's Exec injection point. Unlike the compose subcommands, Run does
// not honor DryRun itself: callers that must skip the call entirely under
// --dry-run should check r.DryRun before calling Run.
func (r *Runner) Run(ctx context.Context, name string, args ...string) (string, error) {
	return r.exec(ctx, name, args...)
}

// Up runs `docker compose up -d`.
func (r *Runner) Up(ctx context.Context) (string, error) {
	return r.runCompose(ctx, "up", "-d")
}

// Down runs `docker compose down`.
func (r *Runner) Down(ctx context.Context) (string, error) {
	return r.runCompose(ctx, "down")
}

// PS runs `docker compose ps`.
func (r *Runner) PS(ctx context.Context) (string, error) {
	return r.runCompose(ctx, "ps")
}

// ContainerRunning reports whether the named container is running, via
// `docker inspect -f {{.State.Running}} <name>`. In DryRun mode it returns
// false, nil without executing anything (there is no meaningful command
// line to report for a boolean check).
func (r *Runner) ContainerRunning(ctx context.Context, name string) (bool, error) {
	if r.DryRun {
		return false, nil
	}
	out, err := r.exec(ctx, "docker", "inspect", "-f", "{{.State.Running}}", name)
	if err != nil {
		return false, fmt.Errorf("docker inspect %s: %w", name, err)
	}
	running, perr := strconv.ParseBool(strings.TrimSpace(out))
	if perr != nil {
		return false, fmt.Errorf("parse docker inspect output %q: %w", strings.TrimSpace(out), perr)
	}
	return running, nil
}

// ContainerCommand executes a command inside an already-running container.
// Status checks use this for the image-provided rcon-cli, which speaks the
// exact RCON dialect bundled with the running Minecraft image.
func (r *Runner) ContainerCommand(ctx context.Context, container string, command ...string) (string, error) {

	args := append([]string{"exec", container}, command...)
	if r.DryRun {
		return commandLine("docker", args), nil
	}
	return r.exec(ctx, "docker", args...)
}
