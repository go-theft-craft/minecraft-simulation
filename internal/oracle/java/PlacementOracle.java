// PlacementOracle asks a Java Edition 1.8.9 server jar what block state a click
// produces, so that a placement rule can be compared against the game's own
// rather than against a reading of it.
//
// Nothing here reimplements a placement rule. Which way stairs face, which half
// a slab takes, which axis a log lies along, and what a block with no
// orientation becomes are all decided by Mojang's bytecode inside
// Block.onBlockPlaced, which is the call ItemBlock.onItemUse makes and the only
// place this version decides a placed state.
//
// **onBlockPlacedBy is not called, and that is a limit worth stating.** Vanilla
// calls it after the block is in the world, and a handful of blocks — a bed, a
// piston, a chest — do their orientation there instead. Those blocks are outside
// what this harness answers for, and the corpus says so rather than reporting a
// state the game would have replaced a tick later.
//
// The state crosses the boundary as the metadata Block.getMetaFromState
// produces, because that is how this version addresses a state: a block id and
// four bits. Its counterpart on 26.1.2 reports a flat state id, and the two
// numbers are not comparable — which is the whole reason the placement rule is
// version-owned.
//
// The world stub is MoveOracle's, extended with a spawn point as MiningOracle's
// is: an EntityPlayer reads one in its constructor.
//
// Protocol, one command per line on standard input:
//   P item clicked[3] face cursor[3] yaw pitch player[3]
// where item is a registry name, face is the wire's own numbering (0 bottom,
// 1 top, 2 north, 3 south, 4 west, 5 east), and cursor is the hit point inside
// the clicked cell, in block-local coordinates.
// P writes one line: "A " and then the metadata and the block's name, or
// "A none -" for an item that is not a block.
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintWriter;
import java.util.UUID;

import com.mojang.authlib.GameProfile;

import net.minecraft.block.Block;
import net.minecraft.block.state.IBlockState;
import net.minecraft.entity.player.EntityPlayer;
import net.minecraft.init.Blocks;
import net.minecraft.init.Bootstrap;
import net.minecraft.item.Item;
import net.minecraft.item.ItemBlock;
import net.minecraft.item.ItemStack;
import net.minecraft.util.BlockPos;
import net.minecraft.util.EnumFacing;
import net.minecraft.world.World;

public final class PlacementOracle {
    /** StubPlayer is the smallest concrete EntityPlayer a placement rule reads. */
    static final class StubPlayer extends EntityPlayer {
        StubPlayer(World world) {
            super(world, new GameProfile(UUID.nameUUIDFromBytes("oracle".getBytes()), "oracle"));
        }

        @Override
        public boolean isSpectator() {
            return false;
        }
    }

    public static void main(String[] args) throws Exception {
        Bootstrap.register();

        BufferedReader in = new BufferedReader(new InputStreamReader(System.in, "UTF-8"));
        PrintWriter out = new PrintWriter(new java.io.OutputStreamWriter(System.out, "UTF-8"));

        MiningOracle.StubWorld world = new MiningOracle.StubWorld();
        world.air = Blocks.air.getDefaultState();

        String line;
        while ((line = in.readLine()) != null) {
            line = line.trim();
            if (line.isEmpty()) {
                continue;
            }

            String[] parts = line.split("\\s+");
            if (!parts[0].equals("P")) {
                throw new IllegalArgumentException("unknown command: " + parts[0]);
            }

            int at = 1;
            String itemName = parts[at++];
            BlockPos clicked = new BlockPos(
                    Integer.parseInt(parts[at++]), Integer.parseInt(parts[at++]), Integer.parseInt(parts[at++]));
            EnumFacing face = faceOf(Integer.parseInt(parts[at++]));
            float cursorX = Float.parseFloat(parts[at++]);
            float cursorY = Float.parseFloat(parts[at++]);
            float cursorZ = Float.parseFloat(parts[at++]);
            float yaw = Float.parseFloat(parts[at++]);
            float pitch = Float.parseFloat(parts[at++]);
            double playerX = Double.parseDouble(parts[at++]);
            double playerY = Double.parseDouble(parts[at++]);
            double playerZ = Double.parseDouble(parts[at++]);

            Item item = Item.getByNameOrId(itemName);
            if (item == null) {
                throw new IllegalArgumentException("unknown item: " + itemName);
            }
            if (!(item instanceof ItemBlock)) {
                out.println("A none -");
                out.flush();

                continue;
            }

            Block block = ((ItemBlock) item).getBlock();

            // The clicked cell holds stone and the cell the placement lands in
            // holds air, which is the situation a player is in when they build.
            // A rule that reads its neighbours reads this world, so what it
            // sees is stated here rather than left to the last case.
            world.blocks.clear();
            world.blocks.put(MoveOracle.StubWorld.key(clicked.getX(), clicked.getY(), clicked.getZ()),
                    Blocks.stone.getDefaultState());

            StubPlayer player = new StubPlayer(world);
            player.setPositionAndRotation(playerX, playerY, playerZ, yaw, pitch);
            // The yaw a placement rule reads is the one the body has moved to,
            // not the one it is turning towards.
            player.prevRotationYaw = yaw;
            player.rotationYawHead = yaw;
            player.prevRotationPitch = pitch;

            ItemStack stack = new ItemStack(item);
            int metadata = item.getMetadata(stack.getMetadata());
            BlockPos placed = clicked.offset(face);

            IBlockState state = block.onBlockPlaced(
                    world, placed, face, cursorX, cursorY, cursorZ, metadata, player);

            out.println("A " + block.getMetaFromState(state) + " " + Block.blockRegistry.getNameForObject(block));
            out.flush();
        }

        out.flush();
    }

    /** faceOf turns the wire's face number into the game's direction. */
    private static EnumFacing faceOf(int face) {
        switch (face) {
            case 0:
                return EnumFacing.DOWN;
            case 1:
                return EnumFacing.UP;
            case 2:
                return EnumFacing.NORTH;
            case 3:
                return EnumFacing.SOUTH;
            case 4:
                return EnumFacing.WEST;
            case 5:
                return EnumFacing.EAST;
            default:
                throw new IllegalArgumentException("no such face: " + face);
        }
    }
}
