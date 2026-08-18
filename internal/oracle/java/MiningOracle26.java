// MiningOracle26 asks a Java Edition 26.1.2 server jar how fast a real player
// breaks a real block, so that break times can be compared against the game's
// own arithmetic rather than against a reading of it.
//
// Nothing here reimplements a mining rule. The tool speed, the harvest
// legality, the efficiency bonus, the haste and mining-fatigue scaling, the
// submerged penalty, and the airborne penalty are all executed by Mojang's
// bytecode inside BlockState.getDestroyProgress, Player.getDestroySpeed,
// ItemStack.getDestroySpeed, and ItemStack.isCorrectToolForDrops. This file
// supplies a level, a player, an inventory, and a text protocol.
//
// The level is MoveOracle26's, allocated without running its constructor for
// the reason that file states. The collision, movement, and mining oracles must
// not be able to disagree about what a world is.
//
// Two things this harness does that the game does inside a tick it cannot run
// here, both named rather than hidden:
//
//   * **Equipment attribute modifiers are applied by hand.** A held item's
//     modifiers — which is how Efficiency reaches MINING_EFFICIENCY, since 26.1
//     states the enchantment as data rather than as a branch — are installed by
//     LivingEntity.handleEquipmentChanges during a tick. A player cannot be
//     ticked on a level with no chunk source, so the harness walks
//     ItemStack.forEachModifier and adds each modifier as a transient one,
//     which is what that method does with them. The values are the jar's; only
//     the moment they are applied is this file's.
//   * **The vanilla data pack is loaded**, by Loaded26. 26.1 keeps a tool's
//     speeds in an item component and its enchantments in a data-driven
//     registry, and a server binds both when it loads its pack, so both come
//     from the pack rather than from a constant typed here.
//
//   * **The submerged flag is stated rather than observed.** The game decides
//     it by tracking fluids across a tick, and its tracker reads a loaded chunk
//     through a chunk source this level does not have. So StubPlayer answers
//     isEyeInFluid from the case instead, which is the same claim
//     mining.Conditions.Underwater is on the Go side. What the jar still
//     executes is the rule under test: the SUBMERGED_MINING_SPEED attribute
//     applied to the speed. The 1.8.9 harness does not need this — that
//     version reads the eye's block directly, so it is given real water.
//
// The player is rebuilt for every case. Effects and inventory contents persist
// on a player, and a case that inherited the previous case's haste would be a
// case that measured the wrong thing and passed.
//
// This version ships unobfuscated, so the harness compiles against the real
// names and javac checks it against the jar it will run on. A renamed method
// fails to compile instead of throwing halfway through a run.
//
// Protocol, one command per line on standard input:
//   Q block held efficiency haste fatigue underwater airborne
// where block and held are registry names, held is "-" for a bare hand, and
// haste and fatigue are amplifiers with -1 meaning the effect is absent.
// Q writes one line: "A " and then hardness speed damage ticks harvestable. The
// marker is there because loading the game writes its own lines to standard
// output, and a reader counting lines would take one of those for an answer.
// The three floats cross the boundary as hexadecimal float literals, and ticks
// is -1 when the game never finishes the block.
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintStream;
import java.util.UUID;

import com.mojang.authlib.GameProfile;

import net.minecraft.SharedConstants;
import net.minecraft.core.BlockPos;
import net.minecraft.core.Holder;
import net.minecraft.core.registries.BuiltInRegistries;
import net.minecraft.core.registries.Registries;
import net.minecraft.resources.Identifier;
import net.minecraft.server.Bootstrap;
import net.minecraft.server.ReloadableServerResources;
import net.minecraft.tags.FluidTags;
import net.minecraft.tags.TagKey;
import net.minecraft.world.effect.MobEffectInstance;
import net.minecraft.world.effect.MobEffects;
import net.minecraft.world.entity.EquipmentSlot;
import net.minecraft.world.entity.ai.attributes.AttributeInstance;
import net.minecraft.world.entity.player.Player;
import net.minecraft.world.item.Item;
import net.minecraft.world.item.ItemStack;
import net.minecraft.world.item.component.ResolvableProfile;
import net.minecraft.world.item.enchantment.Enchantment;
import net.minecraft.world.item.enchantment.Enchantments;
import net.minecraft.world.level.GameType;
import net.minecraft.world.level.Level;
import net.minecraft.world.level.block.Block;
import net.minecraft.world.level.block.Blocks;
import net.minecraft.world.level.block.state.BlockState;
import net.minecraft.world.level.material.Fluid;

public final class MiningOracle26 {
    /** StubPlayer is the smallest concrete Player that can hold a tool. */
    static final class StubPlayer extends Player {
        private final ResolvableProfile profile;

        /** submerged is the case's claim about the eye, not the level's. */
        boolean submerged;

        StubPlayer(Level level, GameProfile gameProfile) {
            super(level, gameProfile);
            this.profile = ResolvableProfile.createResolved(gameProfile);
        }

        @Override
        public boolean isEyeInFluid(TagKey<Fluid> fluid) {
            return this.submerged && fluid == FluidTags.WATER;
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

    /** The block under test, and the block the player stands in. */
    private static final BlockPos TARGET = new BlockPos(0, 64, 0);
    private static final double STAND_X = 0.5;
    private static final double STAND_Y = 65.0;
    private static final double STAND_Z = 0.5;

    public static void main(String[] arguments) throws Exception {
        // Held before the game starts. Bootstrapping installs a logging
        // framework that takes System.out over, and an answer written through
        // that arrives wrapped in log decoration and parses as nothing.
        PrintStream out = System.out;

        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();

        // The server's own data load, which Loaded26 runs in WorldLoader's
        // order. This version cannot answer a question about a tool without it:
        // a tool's speeds live in an item component, and components are bound
        // when a server loads its pack rather than when the game bootstraps.
        ReloadableServerResources loaded = Loaded26.load();

        Holder<Enchantment> efficiencyEnchantment = loaded.fullRegistries()
                .lookup()
                .lookupOrThrow(Registries.ENCHANTMENT)
                .getOrThrow(Enchantments.EFFICIENCY);

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
            if (!parts[0].equals("Q")) {
                throw new IllegalArgumentException("unknown command: " + parts[0]);
            }

            int at = 1;
            String blockName = parts[at++];
            String heldName = parts[at++];
            int efficiency = Integer.parseInt(parts[at++]);
            int haste = Integer.parseInt(parts[at++]);
            int fatigue = Integer.parseInt(parts[at++]);
            boolean underwater = Boolean.parseBoolean(parts[at++]);
            boolean airborne = Boolean.parseBoolean(parts[at++]);

            BlockState state = state(blockName);

            level.blocks.clear();
            level.blocks.put(MoveOracle26.StubLevel.key(TARGET.getX(), TARGET.getY(), TARGET.getZ()), state);

            StubPlayer player = new StubPlayer(level,
                    new GameProfile(UUID.nameUUIDFromBytes("oracle".getBytes()), "oracle"));
            player.setPos(STAND_X, STAND_Y, STAND_Z);
            player.setOnGround(!airborne);

            player.submerged = underwater;

            ItemStack held = ItemStack.EMPTY;
            if (!heldName.equals("-")) {
                Item item = BuiltInRegistries.ITEM.getValue(Identifier.parse(heldName));
                if (item == null) {
                    throw new IllegalArgumentException("unknown item: " + heldName);
                }
                held = new ItemStack(item);
                if (efficiency > 0) {
                    held.enchant(efficiencyEnchantment, efficiency);
                }
            }
            player.getInventory().setSelectedSlot(0);
            player.getInventory().setItem(0, held);
            applyModifiers(player, held);

            if (haste >= 0) {
                player.addEffect(new MobEffectInstance(MobEffects.HASTE, 20 * 60, haste));
            }
            if (fatigue >= 0) {
                player.addEffect(new MobEffectInstance(MobEffects.MINING_FATIGUE, 20 * 60, fatigue));
            }

            float hardness = state.getDestroySpeed(level, TARGET);
            float speed = held.getDestroySpeed(state);
            float damage = state.getDestroyProgress(player, level, TARGET);

            // "A " marks an answer. Loading the game writes progress lines to
            // standard output, and a reader counting lines would take one of
            // those for a break time.
            out.println("A " + Float.toHexString(hardness)
                    + " " + Float.toHexString(speed)
                    + " " + Float.toHexString(damage)
                    + " " + ticks(damage)
                    + " " + player.hasCorrectToolForDrops(state));
            out.flush();
        }

        out.flush();
    }

    /**
     * applyModifiers installs a held item's attribute modifiers on the player.
     *
     * This is the application step of LivingEntity.handleEquipmentChanges, which
     * runs inside a tick this harness cannot run: a level with no chunk source
     * cannot tick a player. The modifiers themselves — including the ones an
     * enchantment contributes — come from the jar's own forEachModifier.
     */
    private static void applyModifiers(Player player, ItemStack held) {
        held.forEachModifier(EquipmentSlot.MAINHAND, (attribute, modifier) -> {
            AttributeInstance instance = player.getAttributes().getInstance(attribute);
            if (instance == null) {
                return;
            }
            instance.removeModifier(modifier.id());
            instance.addTransientModifier(modifier);
        });
    }

    /** state returns a block's default state, or fails on an unknown name. */
    private static BlockState state(String name) {
        Identifier id = Identifier.parse(name);
        Block block = BuiltInRegistries.BLOCK.getValue(id);
        if (block == Blocks.AIR && !name.equals("air") && !name.equals("minecraft:air")) {
            throw new IllegalArgumentException("unknown block: " + name);
        }

        return block.defaultBlockState();
    }

    /**
     * ticks counts the additions the game makes before its progress reaches one.
     *
     * This is MultiPlayerGameMode.continueDestroyBlock's loop, transcribed: the
     * class is a client class and this jar is the server. It is the only game
     * logic in this file, and it is here rather than on the Go side so that the
     * accumulation the corpus records is the game's width and the game's
     * addition.
     *
     * A fraction that no longer moves the running total is a block this player
     * never breaks, which is a different answer from a long time and is reported
     * as -1 rather than as a number.
     */
    private static int ticks(float damage) {
        if (damage <= 0.0F) {
            return -1;
        }

        float progress = 0.0F;
        for (int count = 1; ; count++) {
            float next = progress + damage;
            if (next == progress) {
                return -1;
            }
            progress = next;
            if (progress >= 1.0F) {
                return count;
            }
        }
    }
}
