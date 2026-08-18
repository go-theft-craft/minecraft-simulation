// CombatOracle asks a Java Edition 1.8.9 server jar what one bare-handed swing
// does to another player, so that damage and knockback can be compared against
// the game's own arithmetic rather than against a reading of it.
//
// Nothing here reimplements a combat rule. The attribute read, the critical
// multiplier, the enchantment modifier, the base knockback, the sprint and
// enchantment bonus, and the sprint cancellation are all executed by Mojang's
// bytecode inside EntityPlayer.attackTargetEntityWithCurrentItem and
// EntityLivingBase.knockBack. This file supplies two players, a world, and a
// text protocol.
//
// The knockback bonus is enchanted onto a stick rather than a sword, because a
// stick carries no attack-damage modifier: the case then measures the bonus
// and only the bonus, at the fist's base damage.
//
// Both players are rebuilt for every case. Sprint state, fall distance, and
// hurt-resistance ticks all persist on a player, and a case that inherited the
// previous case's would measure the wrong thing and pass.
//
// Protocol, one command per line on standard input:
//   Q sprint crit kb dx dz mx my mz
// where sprint and crit are booleans, kb is the knockback enchantment level,
// dx dz place the target relative to the attacker, and mx my mz are the
// target's motion before the hit. Doubles cross as hexadecimal float literals.
// Q writes one line: "A " and then damage (a float), the target's motion after
// the hit (three doubles), and whether the attacker is still sprinting. The
// marker is there because loading the game writes its own lines to standard
// output, and a reader counting lines would take one of those for an answer.
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintWriter;
import java.util.UUID;

import com.mojang.authlib.GameProfile;

import net.minecraft.enchantment.Enchantment;
import net.minecraft.entity.player.EntityPlayer;
import net.minecraft.init.Blocks;
import net.minecraft.init.Bootstrap;
import net.minecraft.init.Items;
import net.minecraft.item.ItemStack;
import net.minecraft.util.BlockPos;
import net.minecraft.world.EnumDifficulty;
import net.minecraft.world.World;

public final class CombatOracle {
    /**
     * StubWorld is the mining oracle's world with a difficulty added: a hurt
     * player asks for one on the way to deciding whether the source scales,
     * and the base world reads it from the world info this harness has none
     * of.
     */
    static final class StubWorld extends MoveOracle.StubWorld {
        @Override
        public BlockPos getSpawnPoint() {
            return new BlockPos(0, 64, 0);
        }

        @Override
        public EnumDifficulty getDifficulty() {
            return EnumDifficulty.NORMAL;
        }
    }

    /** StubPlayer is the smallest concrete EntityPlayer that can fight. */
    static final class StubPlayer extends EntityPlayer {
        StubPlayer(World world, String name) {
            super(world, new GameProfile(UUID.nameUUIDFromBytes(name.getBytes()), name));
        }

        @Override
        public boolean isSpectator() {
            return false;
        }
    }

    private static final double STAND_X = 0.5;
    private static final double STAND_Y = 64.0;
    private static final double STAND_Z = 0.5;

    public static void main(String[] args) throws Exception {
        Bootstrap.register();

        BufferedReader in = new BufferedReader(new InputStreamReader(System.in, "UTF-8"));
        PrintWriter out = new PrintWriter(new java.io.OutputStreamWriter(System.out, "UTF-8"));

        StubWorld world = new StubWorld();
        world.air = Blocks.air.getDefaultState();

        String line;
        while ((line = in.readLine()) != null) {
            line = line.trim();
            if (line.isEmpty()) {
                continue;
            }

            String[] parts = line.split("\\s+");
            if (!parts[0].equals("Q")) {
                throw new IllegalArgumentException("unknown command: " + parts[0]);
            }

            int at = 1;
            boolean sprint = Boolean.parseBoolean(parts[at++]);
            boolean crit = Boolean.parseBoolean(parts[at++]);
            int kb = Integer.parseInt(parts[at++]);
            double dx = Double.parseDouble(parts[at++]);
            double dz = Double.parseDouble(parts[at++]);
            double mx = Double.parseDouble(parts[at++]);
            double my = Double.parseDouble(parts[at++]);
            double mz = Double.parseDouble(parts[at++]);

            StubPlayer attacker = new StubPlayer(world, "attacker");
            attacker.setPosition(STAND_X, STAND_Y, STAND_Z);
            attacker.onGround = true;

            StubPlayer target = new StubPlayer(world, "target");
            target.setPosition(STAND_X + dx, STAND_Y, STAND_Z + dz);
            target.onGround = true;
            target.motionX = mx;
            target.motionY = my;
            target.motionZ = mz;

            // The bonus knockback travels along the attacker's yaw rather than
            // along the line between the two, so the attacker faces the target
            // the way a client aiming at it would.
            attacker.rotationYaw = (float) (Math.atan2(-dx, dz) * 180.0 / Math.PI);

            attacker.setSprinting(sprint);
            if (crit) {
                // The game's own conditions for a critical: falling, and not
                // on the ground. Ladders, water, blindness, and riding are
                // absent by construction.
                attacker.fallDistance = 1.5F;
                attacker.onGround = false;
            }
            if (kb > 0) {
                ItemStack stick = new ItemStack(Items.stick);
                stick.addEnchantment(Enchantment.knockback, kb);
                attacker.inventory.currentItem = 0;
                attacker.inventory.setInventorySlotContents(0, stick);
            }

            float before = target.getHealth();
            attacker.attackTargetEntityWithCurrentItem(target);

            out.println("A " + Float.toHexString(before - target.getHealth())
                    + " " + Double.toHexString(target.motionX)
                    + " " + Double.toHexString(target.motionY)
                    + " " + Double.toHexString(target.motionZ)
                    + " " + attacker.isSprinting());
            out.flush();
        }

        out.flush();
    }
}
