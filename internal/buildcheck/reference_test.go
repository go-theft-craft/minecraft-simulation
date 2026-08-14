package buildcheck

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestrictedReferenceArtifactsAreNotTracked(t *testing.T) {
	command := exec.Command("git", "ls-files", "-z")
	command.Dir = filepath.Join("..", "..")
	data, err := command.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	for _, raw := range bytes.Split(data, []byte{0}) {
		path := string(raw)
		if path == "" {
			continue
		}
		if strings.HasPrefix(filepath.ToSlash(path), "reference/work/") {
			t.Errorf("restricted workspace file is tracked: %s", path)
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jar", ".class", ".java", ".tiny", ".tsrg", ".srg", ".csrg":
			t.Errorf("restricted artifact is tracked: %s", path)
		}
	}
}
