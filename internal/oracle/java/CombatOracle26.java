// CombatOracle26 asks a Java Edition 26.1.2 server jar for the two combat
// rules a stub level can carry: the attack-cooldown charge curve, and the base
// knockback impulse. Both are executed by Mojang's bytecode — the curve inside
// Player.getAttackStrengthScale, the impulse inside LivingEntity.knockback —
// and this file supplies a player, a level, and a text protocol.
//
// It deliberately asks for less than the 1.8.9 oracle does. A full attack on
// this version runs through Entity.hurtOrSimulate, which takes the server
// branch only over a real ServerLevel — a running server, not a stub — so the
// composition in Player.attack (the scale factor, the critical, the
// enchantment boost) is transcribed into combat.Damage and pinned by these
// curves rather than executed end to end. That is a smaller claim than the
// 1.8.9 lane's, and the corpus says so rather than presenting the two as
// equivalent.
//
// Protocol, one command per line on standard input:
//   C ticks
// answers "A " and the charge at that many ticks since the last swing, as a
// hexadecimal float, at the fist's attack speed.
//   K power dx dz mx my mz ground
// applies LivingEntity.knockback(power, dx, dz) to a player whose motion is
// (mx, my, mz) and whose ground state is ground, and answers "A " and the
// three components of the motion it is left with, as hexadecimal doubles.
// The direction convention is hurtServer's: dx and dz point from the target
// toward the attacker, and the impulse lands away from them.
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintStream;
import java.util.UUID;

import com.mojang.authlib.GameProfile;

import net.minecraft.SharedConstants;
import net.minecraft.server.Bootstrap;
import net.minecraft.world.entity.player.Player;
import net.minecraft.world.item.component.ResolvableProfile;
import net.minecraft.world.level.GameType;
import net.minecraft.world.level.Level;
import net.minecraft.world.phys.Vec3;

public final class CombatOracle26 {
    /** StubPlayer is the smallest concrete Player whose cooldown can be set. */
    static final class StubPlayer extends Player {
        private final ResolvableProfile profile;

        StubPlayer(Level level, GameProfile gameProfile) {
            super(level, gameProfile);
            this.profile = ResolvableProfile.createResolved(gameProfile);
        }

        /**
         * ticksSinceSwing writes the counter the game increments once per tick
         * and zeroes on an attack. The field is LivingEntity's own, protected,
         * which is why this is a subclass method rather than reflection.
         */
        void ticksSinceSwing(int ticks) {
            this.attackStrengthTicker = ticks;
        }

        @Override
        public GameType gameMode() {
            return GameType.SURVIVAL;
        }

        @Override
        public ResolvableProfile getProfile() {
            return this.profile;
        }
    }

    public static void main(String[] arguments) throws Exception {
        // Held before the game starts. Bootstrapping installs a logging
        // framework that takes System.out over, and an answer written through
        // that arrives wrapped in log decoration and parses as nothing.
        PrintStream out = System.out;

        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();

        // The server's own data load. A Player carries an inventory, and this
        // version cannot construct an item stack until its components are
        // bound, which happens when a server loads its pack.
        Loaded26.load();

        MoveOracle26.StubLevel level = MoveOracle26.allocate(MoveOracle26.StubLevel.class);
        level.prepare();

        BufferedReader in = new BufferedReader(new InputStreamReader(System.in, "UTF-8"));

        String line;
        while ((line = in.readLine()) != null) {
            line = line.trim();
            if (line.isEmpty()) {
                continue;
            }

            String[] parts = line.split("\\s+");
            StubPlayer player = new StubPlayer(level,
                    new GameProfile(UUID.nameUUIDFromBytes("oracle".getBytes()), "oracle"));

            switch (parts[0]) {
                case "C": {
                    player.ticksSinceSwing(Integer.parseInt(parts[1]));
                    out.println("A " + Float.toHexString(player.getAttackStrengthScale(0.5F)));
                    break;
                }
                case "K": {
                    int at = 1;
                    double power = Double.parseDouble(parts[at++]);
                    double dx = Double.parseDouble(parts[at++]);
                    double dz = Double.parseDouble(parts[at++]);
                    double mx = Double.parseDouble(parts[at++]);
                    double my = Double.parseDouble(parts[at++]);
                    double mz = Double.parseDouble(parts[at++]);
                    boolean ground = Boolean.parseBoolean(parts[at]);

                    player.setPos(0.5, 64.0, 0.5);
                    player.setDeltaMovement(mx, my, mz);
                    player.setOnGround(ground);
                    player.knockback(power, dx, dz);

                    Vec3 motion = player.getDeltaMovement();
                    out.println("A " + Double.toHexString(motion.x)
                            + " " + Double.toHexString(motion.y)
                            + " " + Double.toHexString(motion.z));
                    break;
                }
                default:
                    throw new IllegalArgumentException("unknown command: " + parts[0]);
            }
            out.flush();
        }

        out.flush();
    }
}
