package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@icehive.local"},
		{"config", "user.name", "icehive-test"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGitPushArgs(t *testing.T) {
	t.Setenv("GIT_PUSH", "origin main")
	args := gitPushArgs()
	if len(args) != 3 || args[0] != "push" || args[1] != "origin" || args[2] != "main" {
		t.Fatalf("got %v", args)
	}
	t.Setenv("GIT_PUSH", "true")
	if got := gitPushArgs(); len(got) != 1 || got[0] != "push" {
		t.Fatalf("got %v", got)
	}
}

func TestGitPeriodicEnabled(t *testing.T) {
	t.Setenv("GIT_COMMIT", "")
	t.Setenv("GIT_PUSH", "")
	if gitPeriodicEnabled() {
		t.Fatal("expected off")
	}
	t.Setenv("GIT_PUSH", "1")
	if !gitPeriodicEnabled() {
		t.Fatal("expected on with GIT_PUSH only")
	}
}

func TestGitCommitEnabled(t *testing.T) {
	t.Setenv("GIT_COMMIT", "")
	if gitCommitEnabled() {
		t.Fatal("expected disabled when unset")
	}
	t.Setenv("GIT_COMMIT", "abc123")
	if !gitCommitEnabled() {
		t.Fatal("expected enabled when set")
	}
	if gitCommitMessage() != "persister-yaml: abc123" {
		t.Fatalf("message: %q", gitCommitMessage())
	}
}

func TestListUntrackedYAMLFiltersExtension(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, dir, "GitRepo/a.yaml", "x: 1\n")
	writeFile(t, dir, "notes.txt", "hi")
	writeFile(t, dir, "Animal/b.yml", "y: 2\n")

	files, err := listUntrackedYAML(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %v", files)
	}
	for _, f := range files {
		if !strings.HasSuffix(f, ".yaml") && !strings.HasSuffix(f, ".yml") {
			t.Fatalf("unexpected file %q", f)
		}
	}
}

func TestSyncGitYAMLCommitsUntrackedOnStartup(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, dir, "Animal/a.yaml", "x: 1\n")

	t.Setenv("GIT_COMMIT", "startup snapshot")
	t.Setenv("GIT_PUSH", "")

	store := &yamlStore{root: dir}
	res, err := store.syncGitYAML(t.Context(), logrus.New(), gitCommitMessage())
	if err != nil {
		t.Fatal(err)
	}
	if res.committedFiles != 1 {
		t.Fatalf("committed %d files, want 1", res.committedFiles)
	}
	out, err := exec.Command("git", "-C", dir, "log", "-1", "--oneline").CombinedOutput()
	if err != nil {
		t.Fatalf("git log: %v (%s)", err, out)
	}
	if !strings.Contains(string(out), "startup snapshot") {
		t.Fatalf("unexpected log: %s", out)
	}
}

func TestRunGitSyncLogsStatus(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, dir, "Animal/a.yaml", "x: 1\n")

	t.Setenv("GIT_COMMIT", "log test")
	t.Setenv("GIT_PUSH", "")

	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	var buf bytes.Buffer
	log.SetOutput(&buf)

	store := &yamlStore{root: dir}
	store.runGitSync(t.Context(), log, gitCommitMessage(), "startup")

	out := buf.String()
	for _, want := range []string{"git sync starting", "git sync completed", "committed_files=1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log missing %q:\n%s", want, out)
		}
	}
}

func TestWriteBlocksDuringCommitLock(t *testing.T) {
	store := &yamlStore{root: t.TempDir()}
	store.mu.Lock()
	done := make(chan struct{})
	go func() {
		_, _ = store.writeEntity(t.Context(), &entityMessage{
			Type:          "Entity",
			SchemaVersion: "v1",
			Metadata: collectorMetadata{
				EntityType:     "Animal",
				SourceUniqueID: "x",
				SourceHash:     sourceHash{HashValue: "h"},
			},
			Values: map[string]any{},
		})
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("write should block while commit holds lock")
	case <-time.After(50 * time.Millisecond):
	}
	store.mu.Unlock()
	<-done
}
