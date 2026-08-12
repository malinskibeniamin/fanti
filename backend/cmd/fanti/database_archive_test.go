package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDatabaseArchiveBackupWritesValidatedArchive(t *testing.T) {
	t.Parallel()

	backupDir := t.TempDir()
	result := runDatabaseArchive(t, "success", "backup", backupDir)
	if result.err != nil {
		t.Fatalf("backup: %v\nstderr: %s", result.err, result.stderr)
	}

	archivePath := strings.TrimSpace(result.stdout)
	if filepath.Dir(archivePath) != backupDir {
		t.Fatalf("archive path = %q, want directory %q", archivePath, backupDir)
	}

	archive, err := os.ReadFile(archivePath) //nolint:gosec // path was emitted by the test-owned script invocation
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if string(archive) != "FAKE_CUSTOM_ARCHIVE\n" {
		t.Fatalf("archive = %q", archive)
	}

	log, err := os.ReadFile(result.dockerLog)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	if !strings.Contains(string(log), "pg_dump") || !strings.Contains(string(log), "pg_restore --list") {
		t.Fatalf("docker calls did not dump and validate archive:\n%s", log)
	}
}

func TestDatabaseArchiveBackupFailureLeavesNoArchive(t *testing.T) {
	t.Parallel()

	backupDir := t.TempDir()
	result := runDatabaseArchive(t, "dump-fails", "backup", backupDir)
	if result.err == nil {
		t.Fatal("backup succeeded, want pg_dump failure")
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("read backup directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("backup directory contains incomplete artifacts: %v", entries)
	}
	if !strings.Contains(result.stderr, "incomplete archive removed") {
		t.Fatalf("stderr does not explain cleanup:\n%s", result.stderr)
	}
}

func TestDatabaseArchiveRestoreRejectsInvalidArchiveBeforeMutation(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "invalid.dump")
	if err := os.WriteFile(archivePath, []byte("INVALID_ARCHIVE\n"), 0o600); err != nil {
		t.Fatalf("write invalid archive: %v", err)
	}

	result := runDatabaseArchive(t, "invalid-archive", "restore", archivePath)
	if result.err == nil {
		t.Fatal("restore succeeded, want validation failure")
	}

	log, err := os.ReadFile(result.dockerLog)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read docker log: %v", err)
	}
	if strings.Contains(string(log), "pg_dump") || strings.Contains(string(log), "pg_restore --clean") {
		t.Fatalf("invalid archive caused database mutation:\n%s", log)
	}
	if !strings.Contains(result.stderr, "archive validation failed") {
		t.Fatalf("stderr does not explain validation failure:\n%s", result.stderr)
	}
}

func TestDatabaseArchiveRestoreRequiresExactConfirmation(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "requested.dump")
	if err := os.WriteFile(archivePath, []byte("REQUESTED_ARCHIVE\n"), 0o600); err != nil {
		t.Fatalf("write requested archive: %v", err)
	}

	result := runDatabaseArchiveWithInput(t, "success", "restore\n", "restore", archivePath)
	if result.err == nil {
		t.Fatal("restore succeeded without exact confirmation")
	}

	log, err := os.ReadFile(result.dockerLog)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	if strings.Contains(string(log), "pg_dump") || strings.Contains(string(log), "pg_restore --clean") {
		t.Fatalf("cancelled restore caused database mutation:\n%s", log)
	}
	if !strings.Contains(result.stderr, "database unchanged") {
		t.Fatalf("stderr does not confirm cancellation:\n%s", result.stderr)
	}
}

func TestDatabaseArchiveRestoreCreatesRecoveryBackupAndRestoresArchive(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "requested.dump")
	if err := os.WriteFile(archivePath, []byte("REQUESTED_ARCHIVE\n"), 0o600); err != nil {
		t.Fatalf("write requested archive: %v", err)
	}

	result := runDatabaseArchive(t, "success", "restore", archivePath)
	if result.err != nil {
		t.Fatalf("restore: %v\nstderr: %s", result.err, result.stderr)
	}

	restored, err := os.ReadFile(result.databaseState)
	if err != nil {
		t.Fatalf("read restored database state: %v", err)
	}
	if string(restored) != "REQUESTED_ARCHIVE\n" {
		t.Fatalf("restored database = %q", restored)
	}

	backups, err := filepath.Glob(filepath.Join(result.backupDir, "*-pre-restore.dump"))
	if err != nil {
		t.Fatalf("glob recovery backups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("recovery backups = %v, want one", backups)
	}
	if !strings.Contains(result.stdout, archivePath) || !strings.Contains(result.stdout, backups[0]) {
		t.Fatalf("success output missing archive paths:\n%s", result.stdout)
	}

	log, err := os.ReadFile(result.dockerLog)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	for _, want := range []string{"pg_dump", "stop app", "pg_restore --clean", "start app"} {
		if !strings.Contains(string(log), want) {
			t.Errorf("docker log missing %q:\n%s", want, log)
		}
	}
	stopAt := strings.Index(string(log), "stop app")
	backupAt := strings.Index(string(log), "pg_dump")
	restoreAt := strings.Index(string(log), "pg_restore --clean")
	if stopAt > backupAt || backupAt > restoreAt {
		t.Fatalf("app must stop before the recovery snapshot and restore:\n%s", log)
	}
}

func TestDatabaseArchiveRestoreFailureRollsBackToRecoveryBackup(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "requested.dump")
	if err := os.WriteFile(archivePath, []byte("REQUESTED_ARCHIVE\n"), 0o600); err != nil {
		t.Fatalf("write requested archive: %v", err)
	}

	result := runDatabaseArchive(t, "restore-fails-primary", "restore", archivePath)
	if result.err == nil {
		t.Fatal("restore succeeded, want requested archive failure")
	}

	restored, err := os.ReadFile(result.databaseState)
	if err != nil {
		t.Fatalf("read recovered database state: %v", err)
	}
	if string(restored) != "FAKE_CUSTOM_ARCHIVE\n" {
		t.Fatalf("recovered database = %q, want recovery backup", restored)
	}

	log, err := os.ReadFile(result.dockerLog)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	if got := strings.Count(string(log), "pg_restore --clean"); got != 2 {
		t.Fatalf("restore attempts = %d, want requested restore plus rollback:\n%s", got, log)
	}
	if !strings.Contains(result.stderr, "original database restored from") {
		t.Fatalf("stderr does not report successful rollback:\n%s", result.stderr)
	}
	if !strings.Contains(string(log), "start app") {
		t.Fatalf("app was not restarted after rollback:\n%s", log)
	}
}

type archiveResult struct {
	stdout        string
	stderr        string
	dockerLog     string
	backupDir     string
	databaseState string
	err           error
}

func runDatabaseArchive(t *testing.T, mode string, args ...string) archiveResult {
	t.Helper()

	return runDatabaseArchiveWithInput(t, mode, "RESTORE\n", args...)
}

func runDatabaseArchiveWithInput(
	t *testing.T,
	mode string,
	input string,
	args ...string,
) archiveResult {
	t.Helper()

	binDir := t.TempDir()
	runtimeDir := t.TempDir()
	dockerLog := filepath.Join(runtimeDir, "docker.log")
	backupDir := filepath.Join(runtimeDir, "backups")
	databaseState := filepath.Join(runtimeDir, "database-state")
	fakeDocker := filepath.Join(binDir, "docker")
	const fakeDockerScript = `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$FAKE_DOCKER_LOG"
case "$*" in
  *" pg_dump "*)
	if [ "$FAKE_DOCKER_MODE" = "dump-fails" ]; then
	  printf 'PARTIAL_ARCHIVE\n'
	  exit 7
	fi
    printf 'FAKE_CUSTOM_ARCHIVE\n'
    ;;
  *" pg_restore --list"*)
    cat >/dev/null
	if [ "$FAKE_DOCKER_MODE" = "invalid-archive" ]; then
	  exit 8
	fi
    printf '215; 1259 1 TABLE public goose_db_version fanti\n'
    ;;
	"compose ps --status running --services")
	  printf 'app\n'
	  ;;
	"compose stop app"|"compose start app")
	  ;;
	*" pg_restore --clean "*)
	  input_file="$FAKE_DATABASE_STATE.input.$$"
	  cat > "$input_file"
	  if [ "$FAKE_DOCKER_MODE" = "restore-fails-primary" ] && grep -q REQUESTED_ARCHIVE "$input_file"; then
	    printf 'PARTIAL_RESTORE\n' > "$FAKE_DATABASE_STATE"
	    rm -f "$input_file"
	    exit 10
	  fi
	  mv "$input_file" "$FAKE_DATABASE_STATE"
	  ;;
  *)
    printf 'unexpected docker command: %s\n' "$*" >&2
    exit 90
    ;;
esac
`
	if err := os.WriteFile(fakeDocker, []byte(fakeDockerScript), 0o755); err != nil { //nolint:gosec // executable test fixture
		t.Fatalf("write fake docker: %v", err)
	}

	cmd := exec.CommandContext( //nolint:gosec // arguments are test-controlled and exercise the public script seam
		t.Context(), "sh", append([]string{"../../../deploy/database-archive.sh"}, args...)...,
	)
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"FAKE_DOCKER_LOG="+dockerLog,
		"FAKE_DOCKER_MODE="+mode,
		"FAKE_DATABASE_STATE="+databaseState,
		"FANTI_BACKUP_DIR="+backupDir,
	)
	cmd.Stdin = strings.NewReader(input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return archiveResult{
		stdout:        stdout.String(),
		stderr:        stderr.String(),
		dockerLog:     dockerLog,
		backupDir:     backupDir,
		databaseState: databaseState,
		err:           err,
	}
}
