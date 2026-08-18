package combat_test

import "testing"

func TestVersion1_8_9HasNoCooldownAndSaysWhy(t *testing.T) {
	t.Parallel()

	c := profileFor(t, "1_8_9").Cooldown()
	if c.Present() {
		t.Fatal("1.8.9 reported an attack cooldown; the mechanic arrived in 1.9")
	}
	if c.Reason() == "" {
		t.Fatal("an absent mechanic with no reason is indistinguishable from one " +
			"nobody checked")
	}
	// Full charge always, so shared damage code needs no version branch.
	for _, ticks := range []int{0, 1, 5, 20} {
		if got := c.Charge(ticks, 4.0); got != 1 {
			t.Fatalf("Charge(%d) = %v on 1.8.9, want 1", ticks, got)
		}
	}
}

func TestVersion26_1_2ChargesOverTime(t *testing.T) {
	t.Parallel()

	c := profileFor(t, "26_1_2").Cooldown()
	if !c.Present() {
		t.Fatal("26.1.2 reported no attack cooldown")
	}
	if c.Reason() != "" {
		t.Fatalf("a present mechanic carried an absence reason: %q", c.Reason())
	}

	immediate := c.Charge(0, 4.0)
	partial := c.Charge(3, 4.0)
	full := c.Charge(100, 4.0)

	if !(immediate < partial && partial < full) {
		t.Fatalf("charge did not increase over time: %v, %v, %v",
			immediate, partial, full)
	}
	if full != 1 {
		t.Fatalf("charge saturated at %v, want 1", full)
	}
}

func TestTheChargeCurveIsTheGames(t *testing.T) {
	t.Parallel()

	// The middle of the curve, not only its ends: a curve right at 0 and 1
	// and wrong in between passes every boundary test and fails the corpus in
	// the middle. With the fist's attack speed of 4 the delay is 5 ticks, and
	// the game computes (ticks + 0.5) / 5 in float32.
	c := profileFor(t, "26_1_2").Cooldown()
	for ticks, want := range map[int]float64{
		0: float64(float32(0.5) / 5),
		1: float64(float32(1.5) / 5),
		2: float64(float32(2.5) / 5),
		3: float64(float32(3.5) / 5),
		4: float64(float32(4.5) / 5),
		5: 1,
	} {
		if got := c.Charge(ticks, 4.0); got != want {
			t.Errorf("Charge(%d) = %v, want %v", ticks, got, want)
		}
	}
}

func TestChargeIsClampedAtBothEnds(t *testing.T) {
	t.Parallel()

	// A charge above 1 multiplies damage above vanilla's maximum, which is
	// the kind of defect that passes a happy-path test and fails an
	// anti-cheat.
	c := profileFor(t, "26_1_2").Cooldown()
	for _, ticks := range []int{-5, 0, 1000} {
		got := c.Charge(ticks, 4.0)
		if got < 0 || got > 1 {
			t.Fatalf("Charge(%d) = %v, want it clamped to [0,1]", ticks, got)
		}
	}
}
