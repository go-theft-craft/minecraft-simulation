package buildcheck

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// harnessSources is the one place Java may be tracked.
//
// The files there are ours: they supply a block lookup, a minimal entity, and a
// text protocol, and they reimplement no game logic. They are committed so that
// anyone with a prepared workspace can re-run the differential gates instead of
// taking their results on trust, which is the difference between a check and a
// claim.
//
// Nothing else is exempt. Decompiled sources, jars, mappings, and everything
// under the reference workspace stay untracked, and this is the narrowest hole
// that lets the harnesses through: a directory, not an extension.
const harnessSources = "internal/oracle/java/"

func TestRestrictedReferenceArtifactsAreNotTracked(t *testing.T) {
	command := exec.Command("git", "ls-files", "-z")
	command.Dir = filepath.Join("..", "..")
	data, err := command.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	for _, raw := range bytes.Split(data, []byte{0}) {
		path := filepath.ToSlash(string(raw))
		if path == "" {
			continue
		}
		if !allowed(path) {
			t.Errorf("restricted artifact is tracked: %s", path)
		}
	}
}

func TestTheHarnessExemptionDoesNotReachFurther(t *testing.T) {
	// The exemption is a prefix, and a prefix is easy to widen by accident. A
	// decompiled source dropped one directory up, or a jar dropped inside the
	// harness directory, must still fail.
	for _, path := range []string{
		"internal/oracle/EntityLivingBase.java",
		"internal/oracle/java/server.jar",
		"reference/work/sources/1.8.9/server/net/minecraft/entity/Entity.java",
	} {
		if allowed(path) {
			t.Errorf("%s would be allowed to be tracked", path)
		}
	}
	if !allowed("internal/oracle/java/MovementOracle.java") {
		t.Error("the harness itself would be refused")
	}
}

// allowed reports whether a path may be tracked. It is the rule both tests ask,
// so that the edges cannot drift away from what is enforced.
func allowed(path string) bool {
	if strings.HasPrefix(path, "reference/work/") {
		return false
	}

	extension := strings.ToLower(filepath.Ext(path))
	if extension == ".java" && strings.HasPrefix(path, harnessSources) {
		return true
	}
	switch extension {
	case ".jar", ".class", ".java", ".tiny", ".tsrg", ".srg", ".csrg":
		return false
	}

	return true
}
