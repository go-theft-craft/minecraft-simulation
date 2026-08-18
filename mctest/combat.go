package mctest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// CombatCorpus is what one version's game says about a swing.
//
// Every number in it is the jar's, asked through internal/oracle and
// committed, so the gate that reads it runs without a workspace while the
// numbers stay traceable to the version that stated them.
//
// The two versions cannot be asked the same questions, and the shape says so
// rather than papering over it. 1.8.9's whole attack runs over a stub world,
// so its corpus holds full strikes — damage and knockback from one call into
// EntityPlayer.attackTargetEntityWithCurrentItem. 26.1.2's hurt path demands a
// running server, so its corpus holds the two rules its jar can answer from a
// stub — the cooldown charge curve and the base knockback impulse — and the
// composition between them stays a transcription. A smaller claim honestly
// stated, which the gate restates in its own comments.
type CombatCorpus struct {
	// Version is the game the answers came from: "1.8.9" or "26.1.2".
	Version string `json:"version"`
	// Source says where the numbers came from — the harness, the jar, and the
	// date.
	Source string `json:"source"`
	// Dropped says what this corpus deliberately does not cover, and why, so
	// a reader of a green run is told what was left out.
	Dropped []string `json:"dropped"`
	// CooldownAbsent records, for a version without the attack cooldown, why
	// it has none. It is the difference between "verified absent" and "never
	// checked": a version with the mechanic leaves it empty and carries
	// Charges instead, and the gate refuses a corpus that does neither.
	CooldownAbsent string `json:"cooldownAbsent,omitempty"`
	// Strikes are full attack rounds: 1.8.9's lane.
	Strikes []StrikeCase `json:"strikes,omitempty"`
	// Charges are the cooldown curve: 26.1.2's lane.
	Charges []ChargeCase `json:"charges,omitempty"`
	// Impulses are base knockback calls: 26.1.2's lane.
	Impulses []ImpulseCase `json:"impulses,omitempty"`
}

// StrikeCase is one bare-handed swing and everything the jar said it did.
type StrikeCase struct {
	// Name labels the case for a failure message.
	Name string `json:"name"`
	// Sprint and Critical are the attacker's state; Knockback is the
	// enchantment level on an otherwise inert held stick.
	Sprint    bool `json:"sprint,omitempty"`
	Critical  bool `json:"critical,omitempty"`
	Knockback int  `json:"knockback,omitempty"`
	// DX and DZ place the target relative to the attacker; Motion is the
	// target's motion before the hit. Hexadecimal doubles.
	DX     string    `json:"dx"`
	DZ     string    `json:"dz"`
	Motion [3]string `json:"motion"`
	// Damage is the health the hit removed, a hexadecimal float.
	Damage string `json:"damage"`
	// Result is the motion the target was left with. Hexadecimal doubles.
	Result [3]string `json:"result"`
	// SprintAfter is whether the attacker was still sprinting afterwards —
	// vanilla cancels a sprint that spent itself as bonus knockback.
	SprintAfter bool `json:"sprintAfter"`
}

// DamageValue reads the damage the jar reported.
func (c StrikeCase) DamageValue() (float64, error) { return parseFloat32(c.Damage) }

// ChargeCase is one point on the cooldown curve, at the fist's attack speed.
type ChargeCase struct {
	// Ticks is how many ticks have passed since the last swing.
	Ticks int `json:"ticks"`
	// Charge is Player.getAttackStrengthScale(0.5F), a hexadecimal float.
	Charge string `json:"charge"`
}

// ChargeValue reads the charge the jar reported.
func (c ChargeCase) ChargeValue() (float64, error) { return parseFloat32(c.Charge) }

// ImpulseCase is one LivingEntity.knockback call and what it left behind.
type ImpulseCase struct {
	// Name labels the case for a failure message.
	Name string `json:"name"`
	// Power is the impulse strength; DX and DZ point from the target toward
	// the attacker, as hurtServer passes them. Hexadecimal doubles.
	Power string `json:"power"`
	DX    string `json:"dx"`
	DZ    string `json:"dz"`
	// Motion is the target's motion before; Ground its standing state, which
	// this version's lift depends on and 1.8.9's does not.
	Motion [3]string `json:"motion"`
	Ground bool      `json:"ground"`
	// Result is the motion the target was left with. Hexadecimal doubles.
	Result [3]string `json:"result"`
}

// ParseDoubles reads a triple of the harness's hexadecimal doubles.
func ParseDoubles(triple [3]string) ([3]float64, error) {
	var values [3]float64
	for at, text := range triple {
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return values, fmt.Errorf("mctest: read %q: %w", text, err)
		}
		values[at] = value
	}

	return values, nil
}

// ParseDouble reads one of the harness's hexadecimal doubles.
func ParseDouble(text string) (float64, error) {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("mctest: read %q: %w", text, err)
	}

	return value, nil
}

// Save writes a corpus where a gate will look for it.
func (c CombatCorpus) Save(path string) error {
	content, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("mctest: encode corpus: %w", err)
	}

	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("mctest: write corpus: %w", err)
	}

	return nil
}

// LoadCombatCorpus reads one version's corpus.
//
// A corpus that neither holds cases nor records the cooldown's absence is
// refused rather than returned: a gate over an empty corpus passes and proves
// nothing.
func LoadCombatCorpus(path string) (CombatCorpus, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return CombatCorpus{}, fmt.Errorf("mctest: read corpus: %w", err)
	}

	var corpus CombatCorpus
	decoder := json.NewDecoder(bytes.NewReader(content))
	// An unknown field is a corpus written against a different format, and
	// reading it would silently ignore whatever it was trying to say.
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		return CombatCorpus{}, fmt.Errorf("%w %s: %w", ErrFixture, path, err)
	}

	switch {
	case corpus.Version == "":
		return CombatCorpus{}, fmt.Errorf("%w %s: no version", ErrFixture, path)
	case len(corpus.Strikes) == 0 && len(corpus.Charges) == 0 && len(corpus.Impulses) == 0:
		return CombatCorpus{}, fmt.Errorf("%w %s: no cases", ErrFixture, path)
	case corpus.CooldownAbsent == "" && len(corpus.Charges) == 0:
		return CombatCorpus{}, fmt.Errorf(
			"%w %s: neither a cooldown curve nor a recorded reason for having none",
			ErrFixture, path)
	}

	return corpus, nil
}
