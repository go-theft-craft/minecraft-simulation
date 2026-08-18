package mctest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// PlacementCorpus is what one version's game does with a click.
//
// Every answer in it is the jar's, asked through internal/oracle: 1.8.9's
// Block.onBlockPlaced and 26.1.2's Block.getStateForPlacement, which are the
// calls their versions decide a placed state in. The corpus is committed, so
// the gate that reads it runs without a workspace while the numbers stay
// traceable to the version that stated them.
type PlacementCorpus struct {
	// Version is the game the answers came from: "1.8.9" or "26.1.2".
	Version string `json:"version"`
	// Source says where the numbers came from — the harness and the jar.
	Source string `json:"source"`
	// Covers names the block families this corpus asks about. A placement rule
	// answers something for every block; only these are checked, and a reader
	// of a green run should not conclude more than that.
	Covers []string `json:"covers"`
	// Dropped says what is deliberately not asked, and why.
	Dropped []string `json:"dropped"`
	// Cases are the clicks and the states they produced.
	Cases []PlacementCase `json:"cases"`
}

// PlacementCase is one click and the state the game placed.
type PlacementCase struct {
	// Name labels the case for a failure message.
	Name string `json:"name"`
	// Item is the registry name of the item held.
	Item string `json:"item"`
	// Clicked is the cell the cursor was on.
	Clicked [3]int32 `json:"clicked"`
	// Face is the side clicked, in the wire's own numbering.
	Face uint8 `json:"face"`
	// Cursor is where in the clicked face the click landed, block-local.
	Cursor [3]float64 `json:"cursor"`
	// Yaw and Pitch are where the player was looking.
	Yaw   float32 `json:"yaw"`
	Pitch float32 `json:"pitch"`
	// Player is where the player stood.
	Player [3]float64 `json:"player"`

	// Block is the block the game placed, by registry name.
	Block string `json:"block"`
	// State is how the version addresses what it placed: four bits of metadata
	// on 1.8.9, a flat state id on 26.1.2. The two numbers are not comparable,
	// which is the whole reason a placement rule is version-owned.
	State int `json:"state"`
}

// Save writes a corpus where a gate will look for it.
func (c PlacementCorpus) Save(path string) error {
	content, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("mctest: encode corpus: %w", err)
	}

	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("mctest: write corpus: %w", err)
	}

	return nil
}

// LoadPlacementCorpus reads one version's corpus.
//
// A corpus with no cases is refused rather than returned, for the reason
// LoadMiningCorpus refuses one: a matrix gate with no cases passes and proves
// nothing, and the emptiness would show up as a green run.
func LoadPlacementCorpus(path string) (PlacementCorpus, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return PlacementCorpus{}, fmt.Errorf("mctest: read corpus: %w", err)
	}

	var corpus PlacementCorpus
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		return PlacementCorpus{}, fmt.Errorf("%w %s: %w", ErrFixture, path, err)
	}

	switch {
	case len(corpus.Cases) == 0:
		return PlacementCorpus{}, fmt.Errorf(
			"%w %s: it holds no cases, and a gate with none passes and proves nothing",
			ErrFixture, path,
		)
	case corpus.Source == "":
		return PlacementCorpus{}, fmt.Errorf(
			"%w %s: its numbers name no run to trace them to", ErrFixture, path,
		)
	case len(corpus.Covers) == 0:
		return PlacementCorpus{}, fmt.Errorf(
			"%w %s: it names no families, so a green run would read as covering every block",
			ErrFixture, path,
		)
	}

	return corpus, nil
}
