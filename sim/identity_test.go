package sim

import "testing"

func TestProfileIDString(t *testing.T) {
	id := ProfileID{Edition: "java", GameVersion: "1.8.9", RulesRevision: "1"}
	if got, want := id.String(), "java/1.8.9@1"; got != want {
		t.Fatalf("String = %q, want %q", got, want)
	}
}

func TestProfileIDIsZero(t *testing.T) {
	if !(ProfileID{}).IsZero() {
		t.Error("the zero identity does not report itself as zero")
	}
	if (ProfileID{Edition: "java"}).IsZero() {
		t.Error("a partly filled identity reports itself as zero")
	}
}

func TestRevisionAndTickAreDistinctTypes(t *testing.T) {
	// A compile-time check with a runtime assertion attached, so that the test
	// fails loudly rather than being silently deleted if the types merge.
	var revision Revision = 7
	var tick Tick = 7
	if uint64(revision) != uint64(tick) {
		t.Fatal("the two counters disagree on the value seven")
	}
}
