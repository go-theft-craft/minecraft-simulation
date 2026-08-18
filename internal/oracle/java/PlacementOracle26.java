// PlacementOracle26 asks a Java Edition 26.1.2 server jar what block state a
// click produces, so that a placement rule can be compared against the game's
// own rather than against a reading of it.
//
// Nothing here reimplements a placement rule. Which way stairs face, which half
// a slab takes, which axis a log lies along, and what a block with no
// orientation becomes are all decided by Mojang's bytecode inside
// Block.getStateForPlacement, reading a BlockPlaceContext this file assembles
// out of a level, a player, a held stack, and a hit result.
//
// The state that comes back crosses the boundary as the flat state id
// Block.getId produces, which is the same number this version's profile mints a
// handle from. That is the point: a comparison of names would pass while the
// numbering was wrong, and the numbering is what a client is sent.
//
// The level is MoveOracle26's, and the data pack is loaded by Loaded26 — an
// item cannot be constructed at all until it is.
//
// Protocol, one command per line on standard input:
//   P item clicked[3] face cursor[3] yaw pitch player[3]
// where item is a registry name, face is the wire's own numbering (0 bottom,
// 1 top, 2 north, 3 south, 4 west, 5 east), and cursor is the hit point inside
// the clicked cell, in block-local coordinates.
// P writes one line: "A " and then the state id and the state's block name, or
// "A none -" for an item this version refuses to place at all.
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintStream;
import java.util.UUID;

import com.mojang.authlib.GameProfile;

import net.minecraft.SharedConstants;
import net.minecraft.core.BlockPos;
import net.minecraft.core.Direction;
import net.minecraft.core.registries.BuiltInRegistries;
import net.minecraft.resources.Identifier;
import net.minecraft.server.Bootstrap;
import net.minecraft.world.InteractionHand;
import net.minecraft.world.entity.player.Player;
import net.minecraft.world.item.BlockItem;
import net.minecraft.world.item.Item;
import net.minecraft.world.item.ItemStack;
import net.minecraft.world.item.component.ResolvableProfile;
import net.minecraft.world.item.context.BlockPlaceContext;
import net.minecraft.world.level.GameType;
import net.minecraft.world.level.Level;
import net.minecraft.world.level.block.Block;
import net.minecraft.world.level.block.Blocks;
import net.minecraft.world.level.block.state.BlockState;
import net.minecraft.world.phys.BlockHitResult;
import net.minecraft.world.phys.Vec3;

public final class PlacementOracle26 {
    /** StubPlayer is the smallest concrete Player a placement context accepts. */
    static final class StubPlayer extends Player {
        private final ResolvableProfile profile;

        StubPlayer(Level level, GameProfile gameProfile) {
            super(level, gameProfile);
            this.profile = ResolvableProfile.createResolved(gameProfile);
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
            if (!parts[0].equals("P")) {
                throw new IllegalArgumentException("unknown command: " + parts[0]);
            }

            int at = 1;
            String itemName = parts[at++];
            BlockPos clicked = new BlockPos(
                    Integer.parseInt(parts[at++]), Integer.parseInt(parts[at++]), Integer.parseInt(parts[at++]));
            Direction face = faceOf(Integer.parseInt(parts[at++]));
            double cursorX = Double.parseDouble(parts[at++]);
            double cursorY = Double.parseDouble(parts[at++]);
            double cursorZ = Double.parseDouble(parts[at++]);
            float yaw = Float.parseFloat(parts[at++]);
            float pitch = Float.parseFloat(parts[at++]);
            double playerX = Double.parseDouble(parts[at++]);
            double playerY = Double.parseDouble(parts[at++]);
            double playerZ = Double.parseDouble(parts[at++]);

            Item item = BuiltInRegistries.ITEM.getValue(Identifier.parse(itemName));
            if (item == null) {
                throw new IllegalArgumentException("unknown item: " + itemName);
            }

            // The clicked cell holds stone and the cell the placement lands in
            // holds air, which is the situation a player is in when they build.
            // A rule that reads its neighbours — a stair's shape, a fence's
            // connections — reads this world, so what it sees is stated here
            // rather than left to whatever the last case left behind.
            level.blocks.clear();
            level.blocks.put(MoveOracle26.StubLevel.key(clicked.getX(), clicked.getY(), clicked.getZ()),
                    Blocks.STONE.defaultBlockState());

            StubPlayer player = new StubPlayer(level,
                    new GameProfile(UUID.nameUUIDFromBytes("oracle".getBytes()), "oracle"));
            player.setPos(playerX, playerY, playerZ);
            player.setYRot(yaw);
            player.setXRot(pitch);
            // The yaw a placement rule reads is the one the body has moved to,
            // not the one it is turning towards.
            player.yRotO = yaw;
            player.xRotO = pitch;

            ItemStack stack = new ItemStack(item);
            BlockHitResult hit = new BlockHitResult(
                    new Vec3(clicked.getX() + cursorX, clicked.getY() + cursorY, clicked.getZ() + cursorZ),
                    face, clicked, false);
            BlockPlaceContext context =
                    new BlockPlaceContext(player, InteractionHand.MAIN_HAND, stack, hit);

            if (!(item instanceof BlockItem block)) {
                out.println("A none -");
                out.flush();

                continue;
            }

            BlockState placed = block.getBlock().getStateForPlacement(context);
            if (placed == null) {
                out.println("A none -");
                out.flush();

                continue;
            }

            out.println("A " + Block.getId(placed) + " "
                    + BuiltInRegistries.BLOCK.getKey(placed.getBlock()));
            out.flush();
        }

        out.flush();
    }

    /** faceOf turns the wire's face number into the game's direction. */
    private static Direction faceOf(int face) {
        switch (face) {
            case 0:
                return Direction.DOWN;
            case 1:
                return Direction.UP;
            case 2:
                return Direction.NORTH;
            case 3:
                return Direction.SOUTH;
            case 4:
                return Direction.WEST;
            case 5:
                return Direction.EAST;
            default:
                throw new IllegalArgumentException("no such face: " + face);
        }
    }
}
