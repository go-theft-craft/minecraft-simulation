package buildcheck

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// This repository provisions its toolchain through Devbox, and the determinism
// matrix cannot: Devbox provisions through Nix, and Nix does not run natively on
// Windows, which is one of the six targets that gate has to cover. So the
// verification job uses Devbox on one platform and the determinism job uses
// actions/setup-go on six.
//
// Two toolchains are affordable. Two *versions* are not: the matrix would then
// be checking a compiler that nothing else in the project uses, and it would go
// on passing while saying nothing about the build everyone actually ships. This
// is the check that keeps the two in step, and it fails loudly rather than
// warning, because a matrix that has quietly stopped testing the real compiler
// looks exactly like one that is working.
var (
	devboxGo    = regexp.MustCompile(`go-flake#go_(\d+)_(\d+)_(\d+)`)
	workflowGo  = regexp.MustCompile(`go-version:\s*"([0-9.]+)"`)
	moduleGoRow = regexp.MustCompile(`(?m)^go\s+([0-9.]+)$`)
)

func TestOneGoVersionIsNamedEverywhere(t *testing.T) {
	root := filepath.Join("..", "..")

	sources := []struct {
		name    string
		path    string
		pattern *regexp.Regexp
		// join says how to turn the capture groups into a version. devbox.json
		// names the package as go_1_26_6 and the others name 1.26.6.
		join func([]string) string
	}{
		{
			name:    "devbox.json",
			path:    "devbox.json",
			pattern: devboxGo,
			join:    func(groups []string) string { return groups[1] + "." + groups[2] + "." + groups[3] },
		},
		{
			name:    ".github/workflows/determinism.yml",
			path:    filepath.Join(".github", "workflows", "determinism.yml"),
			pattern: workflowGo,
			join:    func(groups []string) string { return groups[1] },
		},
		{
			name:    "go.mod",
			path:    "go.mod",
			pattern: moduleGoRow,
			join:    func(groups []string) string { return groups[1] },
		},
	}

	want := ""
	for index, source := range sources {
		content, err := os.ReadFile(filepath.Join(root, source.path))
		if err != nil {
			t.Fatalf("read %s: %v", source.name, err)
		}

		groups := source.pattern.FindStringSubmatch(string(content))
		if groups == nil {
			t.Fatalf("%s names no Go version that %s matches", source.name, source.pattern)
		}

		got := source.join(groups)
		if index == 0 {
			want = got

			continue
		}
		if got != want {
			t.Errorf("%s names Go %s, and %s names Go %s",
				source.name, got, sources[0].name, want)
		}
	}
}
