//go:build windows

package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phyatt/win-r-protector/internal/appmeta"
)

func TestStartAndWrite(t *testing.T) {
	if err := Close(); err != nil {
		t.Fatalf("reset logger: %v", err)
	}
	localAppData := t.TempDir()
	t.Setenv("LocalAppData", localAppData)

	path, err := Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	Errorf("test failure %d", 42)
	if err := Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	wantPath := filepath.Join(localAppData, appmeta.Name, filename)
	if path != wantPath {
		t.Fatalf("log path = %q, want %q", path, wantPath)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(contents), "ERROR test failure 42") {
		t.Fatalf("log did not contain failure: %q", contents)
	}
}
