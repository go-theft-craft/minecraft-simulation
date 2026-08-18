package oracle_test

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/mctest"
)

// combatCorpusDirectory is where the combat gate reads its corpora from,
// beside the arithmetic it gates, for the same reason the mining corpus sits
// beside mining.
const combatCorpusDirectory = "../../combat/testdata/vanilla"

// noCooldownReason is the 1.8.9 corpus's recorded absence. The profile carries
// the same sentence; the corpus repeats it so a reader of the committed file
// is told rather than sent looking.
const noCooldownReason = "the attack cooldown arrived in 1.9; " +
	"every 1.8.9 swing deals full damage regardless of timing"

// strikeQuestion is one bare-handed swing to ask the 1.8.9 jar about.
type strikeQuestion struct {
	name             string
	sprint, critical bool
	knockback        int
	dx, dz           float64
	motion           [3]float64
}

// strikeMatrix samples the axes one swing has: direction, sprint, critical,
// the knockback enchantment, and the target's own motion.
func strikeMatrix() []strikeQuestion {
	return []strikeQuestion{
		{name: "plain", dx: 2},
		{name: "diagonal", dx: 1.5, dz: 1.5},
		{name: "behind", dx: -2, dz: -0.5},
		{name: "sprinting", sprint: true, dx: 2},
		{name: "critical", critical: true, dx: 2},
		{name: "sprinting critical", sprint: true, critical: true, dx: 2},
		{name: "knockback I", knockback: 1, dx: 2},
		{name: "knockback II", knockback: 2, dx: 2},
		{name: "sprinting knockback I", sprint: true, knockback: 1, dx: 2},
		{name: "fleeing target", dx: 2, motion: [3]float64{0.5, 0, 0}},
		{name: "approaching target", dx: 2, motion: [3]float64{-0.5, 0.2, 0.3}},
		{name: "rising target", dx: 2, motion: [3]float64{0, 3, 0}},
	}
}

// combatDropped is what the corpora deliberately do not cover. Silent
// truncation reads as "covered everything" when it did not.
var combatDropped = []string{
	"weapons and armour: the strike is bare-handed against an unarmoured player, " +
		"so the damage is the base attribute and nothing folds an unverified " +
		"reduction into it",
	"sharpness, strength, and weakness: combat.Strike takes their damage " +
		"already resolved, and which level gives what is version data no " +
		"profile carries yet",
	"zero distance: both games nudge the direction by Math.random until it is " +
		"not zero, and a random answer cannot be a corpus case",
}

// combat26Dropped is what the 26.1.2 lane additionally leaves out, because a
// stub level cannot answer it.
var combat26Dropped = []string{
	"the full attack round: Entity.hurtOrSimulate takes the server branch only " +
		"over a real ServerLevel, so Player.attack's composition is transcribed " +
		"into combat.Damage and pinned by the charge curve rather than executed",
	"the sprint and enchantment bonus: this version routes it through a second " +
		"LivingEntity.knockback call that halves the motion again, where 1.8.9 " +
		"adds velocity; the shared impulse carries 1.8.9's composition and the " +
		"divergence is recorded on combat.Knockback",
	"airborne targets: this version keeps their vertical motion where 1.8.9 " +
		"lifts them; recorded on combat.Knockback with the bonus divergence",
}

// TestGenerateCombatCorpus records what the two jars say about a swing.
//
// The flag that makes it write is the movement generator's, deliberately: one
// run rewrites every committed expectation or none of them. Without the flag
// it checks that the committed corpora still say what the jars say.
func TestGenerateCombatCorpus(t *testing.T) {
	t.Run("1.8.9", func(t *testing.T) {
		jar := newOracle(t)

		matrix := strikeMatrix()
		questions := make([]string, 0, len(matrix))
		for _, question := range matrix {
			questions = append(questions, fmt.Sprintf("Q %v %v %d %s %s %s %s %s",
				question.sprint, question.critical, question.knockback,
				plain(question.dx), plain(question.dz),
				plain(question.motion[0]), plain(question.motion[1]), plain(question.motion[2])))
		}

		answers := jar.runTagged(t, "CombatOracle", questions, len(questions))

		corpus := mctest.CombatCorpus{
			Version: "1.8.9",
			Source: "asked of a Java Edition 1.8.9 server jar through " +
				"internal/oracle/java/CombatOracle.java",
			Dropped:        combatDropped,
			CooldownAbsent: noCooldownReason,
		}
		for at, question := range matrix {
			corpus.Strikes = append(corpus.Strikes, readStrikeAnswer(t, question, answers[at]))
		}

		settleCombatCorpus(t, corpus, "1_8_9.json")
	})

	t.Run("26.1.2", func(t *testing.T) {
		jar := newOracle26(t)

		// The curve at the fist's attack speed, sampled through the middle: a
		// curve right at 0 and 1 and wrong in between passes every boundary
		// test, and the middle is where the composition reads it.
		ticks := []int{0, 1, 2, 3, 4, 5, 6, 8, 10, 20, 100}
		impulses := []struct {
			name   string
			dx, dz float64
			motion [3]float64
		}{
			{name: "still", dx: -2},
			{name: "diagonal", dx: -1.5, dz: -1.5},
			{name: "fleeing", dx: -2, motion: [3]float64{0.5, 0, 0}},
			{name: "rising", dx: -2, motion: [3]float64{0, 3, 0}},
			{name: "approaching", dx: -2, motion: [3]float64{-0.5, 0.2, 0.3}},
		}

		questions := make([]string, 0, len(ticks)+len(impulses))
		for _, tick := range ticks {
			questions = append(questions, fmt.Sprintf("C %d", tick))
		}
		for _, impulse := range impulses {
			// The base impulse hurtServer applies, at its strength and with
			// dx dz pointing from the target toward the attacker, on a
			// standing target.
			questions = append(questions, fmt.Sprintf("K 0.4 %s %s %s %s %s true",
				plain(impulse.dx), plain(impulse.dz),
				plain(impulse.motion[0]), plain(impulse.motion[1]), plain(impulse.motion[2])))
		}

		answers := jar.runTagged(t, "CombatOracle26", questions, len(questions))

		corpus := mctest.CombatCorpus{
			Version: "26.1.2",
			Source: "asked of a Java Edition 26.1.2 server jar through " +
				"internal/oracle/java/CombatOracle26.java, at the fist's attack speed",
			Dropped: append(append([]string{}, combatDropped...), combat26Dropped...),
		}
		at := 0
		for _, tick := range ticks {
			fields := answerFields(t, answers[at], 1)
			corpus.Charges = append(corpus.Charges, mctest.ChargeCase{
				Ticks: tick, Charge: fields[0],
			})
			at++
		}
		for _, impulse := range impulses {
			fields := answerFields(t, answers[at], 3)
			corpus.Impulses = append(corpus.Impulses, mctest.ImpulseCase{
				Name:   impulse.name,
				Power:  hexDouble(float64(float32(0.4))),
				DX:     hexDouble(impulse.dx),
				DZ:     hexDouble(impulse.dz),
				Motion: hexTriple(impulse.motion),
				Ground: true,
				Result: [3]string{fields[0], fields[1], fields[2]},
			})
			at++
		}

		settleCombatCorpus(t, corpus, "26_1_2.json")
	})
}

// settleCombatCorpus writes the corpus under -write-fixtures and otherwise
// checks the committed one still says what the jar says.
func settleCombatCorpus(t *testing.T, fresh mctest.CombatCorpus, file string) {
	t.Helper()

	path := filepath.Join(combatCorpusDirectory, file)
	if !*writeFixtures {
		committed, err := mctest.LoadCombatCorpus(path)
		if err != nil {
			t.Fatalf("%s: %v (pass -write-fixtures to record it)", fresh.Version, err)
		}
		compareCombatCorpora(t, committed, fresh)

		return
	}

	if err := fresh.Save(path); err != nil {
		t.Fatalf("%s: save: %v", fresh.Version, err)
	}
	t.Logf("wrote %s: %d strikes, %d charges, %d impulses",
		path, len(fresh.Strikes), len(fresh.Charges), len(fresh.Impulses))
}

// compareCombatCorpora reports every answer that moved.
func compareCombatCorpora(t *testing.T, committed, fresh mctest.CombatCorpus) {
	t.Helper()

	if len(committed.Strikes) != len(fresh.Strikes) ||
		len(committed.Charges) != len(fresh.Charges) ||
		len(committed.Impulses) != len(fresh.Impulses) {
		t.Fatalf("the committed corpus holds %d/%d/%d cases and the jar answered %d/%d/%d",
			len(committed.Strikes), len(committed.Charges), len(committed.Impulses),
			len(fresh.Strikes), len(fresh.Charges), len(fresh.Impulses))
	}
	for at, want := range fresh.Strikes {
		if got := committed.Strikes[at]; got != want {
			t.Errorf("strike %q: committed %+v, jar says %+v", want.Name, got, want)
		}
	}
	for at, want := range fresh.Charges {
		if got := committed.Charges[at]; got != want {
			t.Errorf("charge at %d ticks: committed %+v, jar says %+v", want.Ticks, got, want)
		}
	}
	for at, want := range fresh.Impulses {
		if got := committed.Impulses[at]; got != want {
			t.Errorf("impulse %q: committed %+v, jar says %+v", want.Name, got, want)
		}
	}
}

// readStrikeAnswer turns one harness line into a corpus case.
func readStrikeAnswer(t *testing.T, question strikeQuestion, answer string) mctest.StrikeCase {
	t.Helper()

	fields := answerFields(t, answer, 5)
	sprintAfter, err := strconv.ParseBool(fields[4])
	if err != nil {
		t.Fatalf("read %q: %v", answer, err)
	}

	return mctest.StrikeCase{
		Name:        question.name,
		Sprint:      question.sprint,
		Critical:    question.critical,
		Knockback:   question.knockback,
		DX:          hexDouble(question.dx),
		DZ:          hexDouble(question.dz),
		Motion:      hexTriple(question.motion),
		Damage:      fields[0],
		Result:      [3]string{fields[1], fields[2], fields[3]},
		SprintAfter: sprintAfter,
	}
}

// answerFields splits one answer and checks its arity.
func answerFields(t *testing.T, answer string, want int) []string {
	t.Helper()

	fields := strings.Fields(answer)
	if len(fields) != want {
		t.Fatalf("answer %q has %d fields, want %d", answer, len(fields), want)
	}

	return fields
}

// plain renders a question double the way Java's Double.parseDouble reads it
// back to the same bits.
func plain(value float64) string { return strconv.FormatFloat(value, 'g', -1, 64) }

// hexDouble renders a corpus double the way the harness renders its answers.
func hexDouble(value float64) string { return strconv.FormatFloat(value, 'x', -1, 64) }

// hexTriple renders three corpus doubles.
func hexTriple(values [3]float64) [3]string {
	return [3]string{hexDouble(values[0]), hexDouble(values[1]), hexDouble(values[2])}
}
