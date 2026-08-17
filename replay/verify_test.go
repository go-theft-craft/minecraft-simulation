package replay_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/replay"
)

// theRecordings are the runs the determinism gate rests on, by name.
//
// Asserted as a list rather than counted. A recording that stopped being
// committed would otherwise make this suite greener rather than redder, and each
// of these exists to reach a different piece of arithmetic.
var theRecordings = []string{
	"ice",
	"jump-and-fall",
	"slime-and-soul-sand",
	"sprint-diagonal",
	"step-up",
}

// minimumTicks is how long a recording has to be to be evidence.
//
// The float32 paths this matrix tests are ones where a single differing bit
// compounds over ticks into a visible position. Twenty ticks of walking would
// agree everywhere and say nothing about that, so shrinking these files to speed
// up CI has to fail rather than pass quietly.
const minimumTicks = 200

// TestRecordingsAreReproducible is the one test the determinism matrix runs.
//
// It needs no JDK, no game jar, and no network beyond the module download: every
// expectation is in the committed files. What it proves is agreement, not
// correctness — these digests are this module's own output, and whether they
// match the game is what internal/oracle answers.
func TestRecordingsAreReproducible(t *testing.T) {
	built := profile(t)

	for _, name := range theRecordings {
		t.Run(name, func(t *testing.T) {
			recording, err := replay.Load(filepath.Join("testdata", name+".json"))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if err := replay.Verify(built, recording); err != nil {
				t.Fatalf("Verify: %v", err)
			}
		})
	}
}

func TestTheCommittedRecordingsAreNotTrivial(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil {
		t.Fatalf("list the recordings: %v", err)
	}
	if len(paths) != len(theRecordings) {
		t.Fatalf("found %d recordings, want %d: %v", len(paths), len(theRecordings), paths)
	}

	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		if !slices.Contains(theRecordings, name) {
			t.Errorf("%s is committed but not in the list this gate checks", path)
		}

		recording, err := replay.Load(path)
		if err != nil {
			t.Fatalf("Load %s: %v", path, err)
		}

		if recording.Covers() < minimumTicks {
			t.Errorf("%s covers %d ticks, want at least %d",
				path, recording.Covers(), minimumTicks)
		}
		if recording.DataDigest == "" {
			t.Errorf("%s pins no game data, so a dataset correction would surface here "+
				"as an unexplained mismatch", path)
		}
		if recording.Note == "" {
			t.Errorf("%s says nothing about which arithmetic it reaches, which is the "+
				"only reason it exists", path)
		}

		// A run that stopped moving produces the same result every tick. Six
		// platforms would agree about that too.
		distinct := make(map[string]struct{}, recording.Covers())
		for _, tick := range recording.Ticks {
			distinct[tick.Digest] = struct{}{}
		}
		if len(distinct) < recording.Covers()/2 {
			t.Errorf("%s produced %d distinct digests over %d ticks; the body is barely moving",
				path, len(distinct), recording.Covers())
		}
	}
}
