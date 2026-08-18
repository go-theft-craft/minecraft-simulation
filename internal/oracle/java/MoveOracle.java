// MoveOracle runs the real Entity.moveEntity of a Java Edition 1.8.9 server
// against a world made of blocks this harness is told about, so that the whole
// of this module's collision path can be compared against the game's own
// algorithm rather than against a reading of it.
//
// Nothing here reimplements game logic. The candidate gathering, the axis
// order, the two step-up attempts, the settle, and the collision flags are all
// executed by Mojang's bytecode. This file supplies a block lookup, a minimal
// entity, and a text protocol.
//
// The overrides are chosen to remove everything that is not movement and
// nothing that is. getBlockState is the world; every column reports loaded so
// the game's broad phase actually runs; area loading reports false so block
// contact callbacks are skipped; flammability and wetness report false so fire
// and sound are skipped; and the entity list is empty because this milestone
// has no entities.
//
// Protocol, one command per line on standard input:
//   C                     forget every block
//   B x y z kind          place a block, kind 0 stone and 1 bottom slab
//   M body[6] dx dy dz onGround stepHeight
// M writes one line: body[6] collidedHorizontally collidedVertically onGround.
// Every double crosses the boundary as a hexadecimal float literal.
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintWriter;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

import net.minecraft.block.Block;
import net.minecraft.block.state.IBlockState;
import net.minecraft.entity.Entity;
import net.minecraft.init.Blocks;
import net.minecraft.init.Bootstrap;
import net.minecraft.nbt.NBTTagCompound;
import net.minecraft.profiler.Profiler;
import net.minecraft.util.AxisAlignedBB;
import net.minecraft.util.BlockPos;
import net.minecraft.world.World;
import net.minecraft.world.WorldProviderSurface;
import net.minecraft.world.chunk.IChunkProvider;

public final class MoveOracle {
    /**
     * StubWorld is a block lookup with every other world duty removed.
     *
     * Not final: MiningOracle extends it to add the spawn point an EntityPlayer
     * reads in its constructor. Extending is what keeps the collision, movement,
     * and mining oracles from being able to disagree about what the world is.
     */
    static class StubWorld extends World {
        final Map<Long, IBlockState> blocks = new HashMap<>();
        IBlockState air;

        StubWorld() {
            super(null, null, new WorldProviderSurface(), new Profiler(), false);
        }

        static long key(int x, int y, int z) {
            return ((long) x & 0x3FFFFF) << 42 | ((long) y & 0xFFFFF) << 22 | ((long) z & 0x3FFFFF);
        }

        @Override
        protected IChunkProvider createChunkProvider() {
            return null;
        }

        @Override
        protected int getRenderDistanceChunks() {
            return 0;
        }

        @Override
        public IBlockState getBlockState(BlockPos pos) {
            IBlockState state = this.blocks.get(key(pos.getX(), pos.getY(), pos.getZ()));
            return state == null ? this.air : state;
        }

        @Override
        public boolean isBlockLoaded(BlockPos pos) {
            return true;
        }

        @Override
        public boolean isBlockLoaded(BlockPos pos, boolean allowEmpty) {
            return true;
        }

        @Override
        public boolean isAreaLoaded(BlockPos from, BlockPos to) {
            return false;
        }

        // A world with no chunk provider has no loaded chunk, and saying so is
        // what lets an entity lookup return nothing instead of dereferencing
        // the provider that is not there. Nothing that only moves a body asks
        // this; a dropped item does, when it looks for another to merge with.
        @Override
        protected boolean isChunkLoaded(int chunkX, int chunkZ, boolean allowEmpty) {
            return false;
        }

        @Override
        public boolean isFlammableWithin(AxisAlignedBB region) {
            return false;
        }

        @Override
        public List<Entity> getEntitiesWithinAABBExcludingEntity(Entity excluded, AxisAlignedBB region) {
            return new ArrayList<>();
        }
    }

    /** StubEntity is the smallest concrete Entity that can be moved. */
    static final class StubEntity extends Entity {
        StubEntity(World world) {
            super(world);
        }

        @Override
        protected void entityInit() {
        }

        @Override
        protected void readEntityFromNBT(NBTTagCompound tag) {
        }

        @Override
        protected void writeEntityToNBT(NBTTagCompound tag) {
        }

        @Override
        protected boolean canTriggerWalking() {
            return false;
        }

        @Override
        protected void updateFallState(double distance, boolean onGroundIn, Block block, BlockPos pos) {
        }

        @Override
        public boolean isWet() {
            return false;
        }

        @Override
        public void playSound(String name, float volume, float pitch) {
        }
    }

    public static void main(String[] args) throws Exception {
        Bootstrap.register();

        BufferedReader in = new BufferedReader(new InputStreamReader(System.in, "UTF-8"));
        PrintWriter out = new PrintWriter(new java.io.OutputStreamWriter(System.out, "UTF-8"));

        StubWorld world = new StubWorld();
        world.air = Blocks.air.getDefaultState();
        IBlockState stone = Blocks.stone.getDefaultState();
        IBlockState slab = Blocks.stone_slab.getDefaultState();

        String line;
        while ((line = in.readLine()) != null) {
            line = line.trim();
            if (line.isEmpty()) {
                continue;
            }

            String[] parts = line.split("\\s+");
            switch (parts[0]) {
                case "C":
                    world.blocks.clear();
                    break;
                case "B": {
                    int x = Integer.parseInt(parts[1]);
                    int y = Integer.parseInt(parts[2]);
                    int z = Integer.parseInt(parts[3]);
                    world.blocks.put(StubWorld.key(x, y, z), parts[4].equals("1") ? slab : stone);
                    break;
                }
                case "M": {
                    int at = 1;
                    AxisAlignedBB body = new AxisAlignedBB(
                            Double.parseDouble(parts[at++]),
                            Double.parseDouble(parts[at++]),
                            Double.parseDouble(parts[at++]),
                            Double.parseDouble(parts[at++]),
                            Double.parseDouble(parts[at++]),
                            Double.parseDouble(parts[at++]));
                    double dx = Double.parseDouble(parts[at++]);
                    double dy = Double.parseDouble(parts[at++]);
                    double dz = Double.parseDouble(parts[at++]);
                    boolean onGround = Boolean.parseBoolean(parts[at++]);
                    float stepHeight = Float.parseFloat(parts[at++]);

                    StubEntity entity = new StubEntity(world);
                    entity.setEntityBoundingBox(body);
                    entity.onGround = onGround;
                    entity.stepHeight = stepHeight;
                    entity.moveEntity(dx, dy, dz);

                    AxisAlignedBB after = entity.getEntityBoundingBox();
                    out.println(hex(after.minX) + " " + hex(after.minY) + " " + hex(after.minZ)
                            + " " + hex(after.maxX) + " " + hex(after.maxY) + " " + hex(after.maxZ)
                            + " " + entity.isCollidedHorizontally
                            + " " + entity.isCollidedVertically
                            + " " + entity.onGround);
                    break;
                }
                default:
                    throw new IllegalArgumentException("unknown command: " + parts[0]);
            }
        }

        out.flush();
    }

    private static String hex(double value) {
        return Double.toHexString(value);
    }
}
