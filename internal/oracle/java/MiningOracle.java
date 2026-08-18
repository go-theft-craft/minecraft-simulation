// MiningOracle asks a Java Edition 1.8.9 server jar how fast a real player
// breaks a real block, so that break times can be compared against the game's
// own arithmetic rather than against a reading of it.
//
// Nothing here reimplements a mining rule. The tool speed, the harvest
// legality, the efficiency bonus, the haste and mining-fatigue scaling, the
// submerged penalty, and the airborne penalty are all executed by Mojang's
// bytecode inside Block.getPlayerRelativeBlockHardness,
// EntityPlayer.getToolDigEfficiency, and InventoryPlayer.getStrVsBlock. This
// file supplies a world, a player, an inventory, and a text protocol.
//
// One rule is transcribed, and it is named rather than hidden: the loop that
// adds the per-tick fraction to a running float32 until it reaches one lives in
// PlayerControllerMP.onPlayerDamageBlock, which is a client class this server
// jar does not carry. It is four lines and it is written here in the game's own
// language, at the game's own width, so that the tick count crossing the
// boundary is a Java float32 accumulation rather than a Go one.
//
// The world stub is MoveOracle's, extended with a spawn point: an EntityPlayer
// reads one in its constructor, and the collision stub carries no world info to
// read it from.
//
// The player is rebuilt for every case. Potion effects and inventory contents
// persist on a player, and a case that inherited the previous case's haste
// would be a case that measured the wrong thing and passed.
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
import java.io.PrintWriter;
import java.util.UUID;

import com.mojang.authlib.GameProfile;

import net.minecraft.block.Block;
import net.minecraft.enchantment.Enchantment;
import net.minecraft.entity.player.EntityPlayer;
import net.minecraft.init.Blocks;
import net.minecraft.init.Bootstrap;
import net.minecraft.item.Item;
import net.minecraft.item.ItemStack;
import net.minecraft.potion.Potion;
import net.minecraft.potion.PotionEffect;
import net.minecraft.util.BlockPos;
import net.minecraft.world.World;

public final class MiningOracle {
    /** StubWorld is the collision oracle's world with a spawn point added. */
    static final class StubWorld extends MoveOracle.StubWorld {
        @Override
        public BlockPos getSpawnPoint() {
            return new BlockPos(0, 64, 0);
        }
    }

    /** StubPlayer is the smallest concrete EntityPlayer that can hold a tool. */
    static final class StubPlayer extends EntityPlayer {
        StubPlayer(World world) {
            super(world, new GameProfile(UUID.nameUUIDFromBytes("oracle".getBytes()), "oracle"));
        }

        @Override
        public boolean isSpectator() {
            return false;
        }
    }

    /** The block under test, and the eye position the water case fills. */
    private static final BlockPos TARGET = new BlockPos(0, 64, 0);
    private static final double STAND_X = 0.5;
    private static final double STAND_Y = 65.0;
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
            String blockName = parts[at++];
            String heldName = parts[at++];
            int efficiency = Integer.parseInt(parts[at++]);
            int haste = Integer.parseInt(parts[at++]);
            int fatigue = Integer.parseInt(parts[at++]);
            boolean underwater = Boolean.parseBoolean(parts[at++]);
            boolean airborne = Boolean.parseBoolean(parts[at++]);

            Block block = Block.getBlockFromName(blockName);
            if (block == null) {
                throw new IllegalArgumentException("unknown block: " + blockName);
            }

            world.blocks.clear();
            world.blocks.put(MoveOracle.StubWorld.key(TARGET.getX(), TARGET.getY(), TARGET.getZ()),
                    block.getDefaultState());

            StubPlayer player = new StubPlayer(world);
            player.setPosition(STAND_X, STAND_Y, STAND_Z);
            player.onGround = !airborne;

            // The submerged case is the game's own question — the eye's block
            // being water — rather than a flag on the player, so the water is
            // placed where the eye is and the jar answers for itself.
            if (underwater) {
                BlockPos eye = new BlockPos(STAND_X, STAND_Y + player.getEyeHeight(), STAND_Z);
                world.blocks.put(MoveOracle.StubWorld.key(eye.getX(), eye.getY(), eye.getZ()),
                        Blocks.water.getDefaultState());
            }

            if (!heldName.equals("-")) {
                Item item = Item.getByNameOrId(heldName);
                if (item == null) {
                    throw new IllegalArgumentException("unknown item: " + heldName);
                }
                ItemStack stack = new ItemStack(item);
                if (efficiency > 0) {
                    stack.addEnchantment(Enchantment.efficiency, efficiency);
                }
                player.inventory.currentItem = 0;
                player.inventory.setInventorySlotContents(0, stack);
            }

            if (haste >= 0) {
                player.addPotionEffect(new PotionEffect(Potion.digSpeed.getId(), 20 * 60, haste));
            }
            if (fatigue >= 0) {
                player.addPotionEffect(new PotionEffect(Potion.digSlowdown.getId(), 20 * 60, fatigue));
            }

            float hardness = block.getBlockHardness(world, TARGET);
            float speed = player.inventory.getStrVsBlock(block);
            float damage = block.getPlayerRelativeBlockHardness(player, world, TARGET);

            // "A " marks an answer. Loading the game writes progress lines to
            // standard output, and a reader counting lines would take one of
            // those for a break time.
            out.println("A " + Float.toHexString(hardness)
                    + " " + Float.toHexString(speed)
                    + " " + Float.toHexString(damage)
                    + " " + ticks(damage)
                    + " " + player.canHarvestBlock(block));
            out.flush();
        }

        out.flush();
    }

    /**
     * ticks counts the additions the game makes before its progress reaches one.
     *
     * This is PlayerControllerMP.onPlayerDamageBlock's loop, transcribed: the
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
