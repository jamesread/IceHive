package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const gitCommitInterval = 30 * time.Minute

// gitPeriodicEnabled reports whether the 30-minute git sync loop runs.
func gitPeriodicEnabled() bool {
	return gitCommitEnabled() || gitPushEnabled()
}

// gitCommitEnabled reports whether periodic git commits are enabled (GIT_COMMIT set).
func gitCommitEnabled() bool {
	return strings.TrimSpace(os.Getenv("GIT_COMMIT")) != ""
}

// gitPushEnabled reports whether git push runs after each sync (GIT_PUSH set).
func gitPushEnabled() bool {
	return strings.TrimSpace(os.Getenv("GIT_PUSH")) != ""
}

func gitCommitMessage() string {
	msg := strings.TrimSpace(os.Getenv("GIT_COMMIT"))
	if msg == "" {
		return "persister-yaml: entity snapshot"
	}
	return "persister-yaml: " + msg
}

type gitSyncResult struct {
	committedFiles int
	pushed         bool
}

func (s *yamlStore) runGitSync(ctx context.Context, log *logrus.Logger, message, phase string) {
	fields := logrus.Fields{
		"phase":      phase,
		"data_dir":   s.root,
		"git_commit": gitCommitEnabled(),
		"git_push":   gitPushEnabled(),
	}
	log.WithFields(fields).Info("git sync starting")

	res, err := s.syncGitYAML(ctx, log, message)
	fields["committed_files"] = res.committedFiles
	fields["pushed"] = res.pushed
	if err != nil {
		log.WithError(err).WithFields(fields).Warn("git sync failed")
		return
	}
	log.WithFields(fields).Info("git sync completed")
}

func (s *yamlStore) runPeriodicGitCommit(ctx context.Context, log *logrus.Logger, message string) {
	ticker := time.NewTicker(gitCommitInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runGitSync(ctx, log, message, "periodic")
		}
	}
}

// syncGitYAML pauses entity writes, optionally commits untracked YAML, then optionally pushes.
func (s *yamlStore) syncGitYAML(ctx context.Context, log *logrus.Logger, message string) (gitSyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	log.Debug("pausing entity writes for git sync")
	var res gitSyncResult
	if gitCommitEnabled() {
		n, err := commitUntrackedYAMLLocked(ctx, s.root, message)
		if err != nil {
			return res, err
		}
		res.committedFiles = n
	}
	if gitPushEnabled() {
		if err := gitPushLocked(ctx, s.root); err != nil {
			return res, err
		}
		res.pushed = true
	}
	return res, nil
}

func gitPushArgs() []string {
	v := strings.TrimSpace(os.Getenv("GIT_PUSH"))
	if v == "" || v == "1" || strings.EqualFold(v, "true") {
		return []string{"push"}
	}
	return append([]string{"push"}, strings.Fields(v)...)
}

func gitPushLocked(ctx context.Context, dir string) error {
	args := append([]string{"-C", dir}, gitPushArgs()...)
	out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w (%s)", strings.Join(gitPushArgs(), " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func commitUntrackedYAMLLocked(ctx context.Context, dir, message string) (int, error) {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return 0, fmt.Errorf("data_dir is not a git repository: %w", err)
	}
	files, err := listUntrackedYAML(ctx, dir)
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		return 0, nil
	}
	addArgs := append([]string{"-C", dir, "add", "--"}, files...)
	if out, err := exec.CommandContext(ctx, "git", addArgs...).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("git add: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	commitCmd := exec.CommandContext(ctx, "git", "-C", dir, "commit", "-m", message)
	out, err := commitCmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "nothing to commit") || strings.Contains(msg, "nothing added to commit") {
			return 0, nil
		}
		return 0, fmt.Errorf("git commit: %w (%s)", err, msg)
	}
	return len(files), nil
}

func listUntrackedYAML(ctx context.Context, dir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "ls-files", "--others", "--exclude-standard")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("git ls-files: %w (%s)", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var files []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, ".yaml") || strings.HasSuffix(line, ".yml") {
			files = append(files, line)
		}
	}
	return files, nil
}
