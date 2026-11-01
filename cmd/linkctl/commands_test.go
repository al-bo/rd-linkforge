package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/al-bo/rd-linkforge/internal/link"
	"github.com/al-bo/rd-linkforge/internal/linkio"
)

func runCommand(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, loggingEnv(t, nil), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func readCommandFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCommandUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no command", nil},
		{"unknown command", []string{"unknown"}},
		{"import missing both flags", []string{"import"}},
		{"import missing db", []string{"import", "-in", "input.txt"}},
		{"import missing input", []string{"import", "-db", "db.jsonl"}},
		{"import missing flag value", []string{"import", "-in"}},
		{"import unknown flag", []string{"import", "-unknown"}},
		{"import positional argument", []string{"import", "-in", "input.txt", "-db", "db.jsonl", "extra"}},
		{"export missing db", []string{"export"}},
		{"export missing db value", []string{"export", "-db"}},
		{"export missing out value", []string{"export", "-db", "db.jsonl", "-out"}},
		{"export unknown flag", []string{"export", "-unknown"}},
		{"export positional argument", []string{"export", "-db", "db.jsonl", "extra"}},
		{"config unknown flag", []string{"config", "-unknown"}},
		{"config positional argument", []string{"config", "extra"}},
		{"version extra argument", []string{"version", "extra"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := runCommand(t, tc.args...)
			if code != 2 || stdout != "" || stderr == "" {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestVersionCommand(t *testing.T) {
	code, stdout, stderr := runCommand(t, "version")
	if code != 0 || stdout != fmt.Sprintf("linkctl %s\n", version) || stderr != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestImportThenExportAndContinueIDs(t *testing.T) {
	dir := t.TempDir()
	input, db := filepath.Join(dir, "urls.txt"), filepath.Join(dir, "links.jsonl")
	wantTargets := []string{"https://example.com/a,b", "https://go.dev", "https://example.org"}
	writeLoggingFile(t, input, strings.Join(wantTargets[:2], "\n")+"\n")
	code, stdout, stderr := runCommand(t, "import", "-in", input, "-db", db)
	if code != 0 || stdout != "imported: 2\nskipped: 0\n" {
		t.Fatalf("first import: code=%d stdout=%q stderr=%s", code, stdout, stderr)
	}
	before, err := linkio.LoadFile(db)
	if err != nil || len(before) != 2 {
		t.Fatalf("first import did not persist two links: %v", err)
	}
	writeLoggingFile(t, input, wantTargets[2]+"\n")
	code, stdout, stderr = runCommand(t, "import", "-in", input, "-db", db)
	if code != 0 || stdout != "imported: 1\nskipped: 0\n" {
		t.Fatalf("second import: code=%d stdout=%q stderr=%s", code, stdout, stderr)
	}
	links, err := linkio.LoadFile(db)
	if err != nil || len(links) != 3 {
		t.Fatalf("second import did not persist three links: %v", err)
	}
	if !reflect.DeepEqual(before, links[:2]) {
		t.Error("second import changed existing links")
	}
	for i, l := range links {
		if l.ID != uint64(i+1) || l.Target != wantTargets[i] || l.OwnerID != "cli" {
			t.Errorf("unexpected persisted link: %+v", l)
		}
	}
	for _, destination := range []string{"stdout", "file"} {
		t.Run(destination, func(t *testing.T) {
			args := []string{"export", "-db", db}
			path := filepath.Join(dir, "links.csv")
			if destination == "file" {
				args = append(args, "-out", path)
			}
			code, stdout, stderr := runCommand(t, args...)
			if code != 0 {
				t.Fatalf("export failed: %s", stderr)
			}
			data := stdout
			if destination == "file" {
				if stdout != "" {
					t.Errorf("file export wrote to stdout: %q", stdout)
				}
				data = string(readCommandFile(t, path))
			}
			rows, err := csv.NewReader(strings.NewReader(data)).ReadAll()
			if err != nil || len(rows) != 4 {
				t.Fatalf("CSV rows=%v error=%v", rows, err)
			}
			if !reflect.DeepEqual(rows[0], []string{"id", "code", "target", "owner_id", "created_at", "expires_at"}) {
				t.Fatalf("unexpected CSV header: %v", rows[0])
			}
			for i, row := range rows[1:] {
				if row[0] != fmt.Sprint(i+1) || row[1] != links[i].Code || row[2] != wantTargets[i] || row[3] != "cli" || row[5] != "" {
					t.Errorf("unexpected CSV row: %v", row)
				}
				if _, err := time.Parse(time.RFC3339, row[4]); err != nil || !strings.HasSuffix(row[4], "Z") {
					t.Errorf("creation time must be RFC3339 UTC: %q", row[4])
				}
			}
		})
	}
}

func TestImportPersistsValidTargetsWhenOthersAreSkipped(t *testing.T) {
	dir := t.TempDir()
	input, db := filepath.Join(dir, "urls.txt"), filepath.Join(dir, "links.jsonl")
	writeLoggingFile(t, input, "https://example.com\nnot-a-url\n")
	code, stdout, stderr := runCommand(t, "import", "-in", input, "-db", db)
	if code != 1 || stdout != "imported: 1\nskipped: 1\n" || !strings.Contains(stderr, "line=2 error=") {
		t.Fatalf("code=%d stdout=%q stderr=%s", code, stdout, stderr)
	}
	links, err := linkio.LoadFile(db)
	if err != nil || len(links) != 1 || links[0].Target != "https://example.com" || links[0].ID != 1 {
		t.Fatalf("valid target was not saved: links=%+v error=%v", links, err)
	}
}

func TestEmptyDatabaseExport(t *testing.T) {
	dir := t.TempDir()
	db, out := filepath.Join(dir, "empty.jsonl"), filepath.Join(dir, "out.csv")
	writeLoggingFile(t, db, "")
	code, stdout, stderr := runCommand(t, "export", "-db", db)
	if code != 0 || stdout != "id,code,target,owner_id,created_at,expires_at\n" {
		t.Fatalf("code=%d stdout=%q stderr=%s", code, stdout, stderr)
	}
	code, fileStdout, stderr := runCommand(t, "export", "-db", db, "-out", out)
	if code != 0 || fileStdout != "" || string(readCommandFile(t, out)) != stdout {
		t.Fatalf("file export: code=%d stdout=%q stderr=%s", code, fileStdout, stderr)
	}
}

func TestCommandFileFailures(t *testing.T) {
	dir := t.TempDir()
	input, db := filepath.Join(dir, "urls.txt"), filepath.Join(dir, "valid.jsonl")
	writeLoggingFile(t, input, "https://example.com\n")
	if err := linkio.SaveFile(db, []link.Link{{ID: 1, Code: "000001", Target: "https://example.com", OwnerID: "cli"}}); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "absent")
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"import missing input", []string{"import", "-in", missing, "-db", db}},
		{"import database is directory", []string{"import", "-in", input, "-db", dir}},
		{"export missing database", []string{"export", "-db", missing}},
		{"export cannot create file", []string{"export", "-db", db, "-out", filepath.Join(missing, "out.csv")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := readCommandFile(t, db)
			code, stdout, stderr := runCommand(t, tc.args...)
			if code != 1 || stdout != "" || stderr == "" {
				t.Fatalf("code=%d stdout=%q stderr=%s", code, stdout, stderr)
			}
			if !bytes.Equal(before, readCommandFile(t, db)) {
				t.Error("failed command changed the database")
			}
		})
	}
}

func TestImportRejectsBrokenDatabaseWithoutOverwriting(t *testing.T) {
	dir := t.TempDir()
	input, db := filepath.Join(dir, "urls.txt"), filepath.Join(dir, "db.jsonl")
	writeLoggingFile(t, input, "https://example.com\n")
	for _, tc := range []struct{ name, data string }{
		{"invalid JSON", "{broken}\n"},
		{"duplicate IDs", "{\"ID\":1,\"Code\":\"first\"}\n{\"ID\":1,\"Code\":\"second\"}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeLoggingFile(t, db, tc.data)
			code, stdout, stderr := runCommand(t, "import", "-in", input, "-db", db)
			if code != 1 || stdout != "" || string(readCommandFile(t, db)) != tc.data {
				t.Fatalf("code=%d stdout=%q stderr=%s", code, stdout, stderr)
			}
		})
	}
}

type cancelOnLog struct {
	bytes.Buffer
	cancel context.CancelFunc
}

func (w *cancelOnLog) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if bytes.Contains(p, []byte(`msg="target imported"`)) {
		w.cancel()
	}
	return n, err
}

func TestCancellationDuringImportPreservesDatabase(t *testing.T) {
	dir := t.TempDir()
	input, db := filepath.Join(dir, "urls.txt"), filepath.Join(dir, "db.jsonl")
	writeLoggingFile(t, input, "https://example.com\nhttps://go.dev\n")
	if err := linkio.SaveFile(db, []link.Link{{ID: 7, Code: "000007", OwnerID: "cli", Target: "https://existing.example"}}); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, db)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stderr := &cancelOnLog{cancel: cancel}
	var stdout bytes.Buffer
	code := run(ctx, []string{"import", "-in", input, "-db", db}, loggingEnv(t, map[string]string{"LF_LOG_LEVEL": "debug"}), &stdout, stderr)
	if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "target imported") || !strings.Contains(stderr.String(), "import canceled") {
		t.Fatalf("code=%d stdout=%q stderr=%s", code, stdout.String(), stderr.String())
	}
	if !bytes.Equal(before, readCommandFile(t, db)) {
		t.Error("cancellation after the first target changed the database")
	}
}

func TestCommandOutputFailures(t *testing.T) {
	db := filepath.Join(t.TempDir(), "db.jsonl")
	writeLoggingFile(t, db, "")
	for _, args := range [][]string{{"version"}, {"config"}, {"export", "-db", db}} {
		t.Run(args[0], func(t *testing.T) {
			values := map[string]string{"LF_PG_DSN": "test-dsn", "LF_MONGO_URI": "test-uri", "LF_JWT_SECRET": strings.Repeat("x", 32)}
			var stderr bytes.Buffer
			code := run(context.Background(), args, loggingEnv(t, values), failingOutput{err: os.ErrClosed}, &stderr)
			if code != 1 || !strings.Contains(stderr.String(), "level=ERROR") {
				t.Fatalf("code=%d stderr=%s", code, stderr.String())
			}
		})
	}
}
