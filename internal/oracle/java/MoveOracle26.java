// MoveOracle26 runs the real Entity.collide of a Java Edition 26.1.2 server
// against a world made of blocks this harness is told about, so that the whole
// of this module's shape-based collision path — the step-up assembly included —
// can be compared against the game's own algorithm rather than against a
// reading of it.
//
// ShapeOracle26 checks the pieces: the axis clamp, the multi-axis resolve, and
// the list of candidate step-up heights. What it cannot reach is the assembly
// around them, because that lives in a private instance method and needs a
// Level. This harness supplies one, so the grounded box, the probe the
// candidates are collected from, the choice of the first improving candidate,
// and the drop back to the original feet are all executed by Mojang's bytecode
// too.
//
// Nothing here reimplements game logic. This file supplies a block lookup, a
// minimal entity, a text protocol, and one reflective handle.
//
// **The level is allocated without running its constructor.** That constructor
// wants a writable level record, a layered registry access, and a dimension
// type, and it eagerly builds a damage-source table out of them — so
// constructing one honestly means loading the vanilla data pack, which is a
// server start. None of it is reachable from collision: the whole of what
// Entity.collide asks a level for is entity colliders, the world border, and
// the blocks in a chunk, and those three are the three overrides below. Every
// other level duty throws, so a future version that starts asking a level for
// something else during a move fails loudly here instead of being answered with
// a plausible default.
//
// The block lookup is a BlockGetter behind getChunkForCollisions, which is the
// one seam the game's own broad phase reaches a chunk through. Blocks are
// placed by registry name and answered as their default state, so the shapes
// the comparison runs against are the game's own rather than boxes this harness
// invented — and Q reports them, so the Go side builds its world from the same
// answer instead of a transcription.
//
// This version ships unobfuscated, so the harness compiles against the real
// names and javac checks it against the jar it will run on. A renamed method
// fails to compile instead of throwing halfway through a run.
//
// Protocol, one command per line on standard input:
//   C                                     forget every block
//   B cell[3] name                        place a block by its registry name
//   Q name                                the block's collision boxes, block-local
//   Y name                                the Y coordinates the block's shape offers
//   M body[6] dx dy dz onGround maxStep   the whole collide
// Q and Y write a count and then their values; M writes the resulting motion.
// Every double crosses the boundary as a hexadecimal float literal, and every
// float is parsed from its decimal text at single width.
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintStream;
import java.lang.reflect.Constructor;
import java.lang.reflect.Method;
import java.util.Collection;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

import net.minecraft.SharedConstants;
import net.minecraft.core.BlockPos;
import net.minecraft.core.Holder;
import net.minecraft.core.particles.ExplosionParticleInfo;
import net.minecraft.core.particles.ParticleOptions;
import net.minecraft.core.registries.BuiltInRegistries;
import net.minecraft.resources.Identifier;
import net.minecraft.server.Bootstrap;
import net.minecraft.sounds.SoundEvent;
import net.minecraft.sounds.SoundSource;
import net.minecraft.util.random.WeightedList;
import net.minecraft.world.TickRateManager;
import net.minecraft.world.attribute.EnvironmentAttributeSystem;
import net.minecraft.world.clock.ClockManager;
import net.minecraft.world.damagesource.DamageSource;
import net.minecraft.world.entity.Entity;
import net.minecraft.world.entity.EntityType;
import net.minecraft.world.entity.boss.enderdragon.EnderDragonPart;
import net.minecraft.world.entity.player.Player;
import net.minecraft.world.flag.FeatureFlagSet;
import net.minecraft.world.item.alchemy.PotionBrewing;
import net.minecraft.world.item.crafting.RecipeAccess;
import net.minecraft.world.level.BlockGetter;
import net.minecraft.world.level.EmptyBlockGetter;
import net.minecraft.world.level.ExplosionDamageCalculator;
import net.minecraft.world.level.Level;
import net.minecraft.world.level.biome.Biome;
import net.minecraft.world.level.block.Block;
import net.minecraft.world.level.block.Blocks;
import net.minecraft.world.level.block.entity.BlockEntity;
import net.minecraft.world.level.block.entity.FuelValues;
import net.minecraft.world.level.block.state.BlockState;
import net.minecraft.world.level.border.WorldBorder;
import net.minecraft.world.level.chunk.ChunkSource;
import net.minecraft.world.level.entity.LevelEntityGetter;
import net.minecraft.world.level.gameevent.GameEvent;
import net.minecraft.world.level.material.Fluid;
import net.minecraft.world.level.material.FluidState;
import net.minecraft.world.level.saveddata.maps.MapId;
import net.minecraft.world.level.saveddata.maps.MapItemSavedData;
import net.minecraft.world.level.storage.LevelData;
import net.minecraft.world.level.storage.ValueInput;
import net.minecraft.world.level.storage.ValueOutput;
import net.minecraft.world.phys.AABB;
import net.minecraft.world.phys.Vec3;
import net.minecraft.world.phys.shapes.VoxelShape;
import net.minecraft.world.scores.Scoreboard;
import net.minecraft.world.ticks.LevelTickAccess;

public final class MoveOracle26 {
    /** StubLevel is a block lookup with every other level duty removed. */
    static class StubLevel extends Level {
        // Assigned by prepare rather than by an initializer, because this class
        // is allocated without running a constructor and field initializers run
        // with one.
        Map<Long, BlockState> blocks;
        WorldBorder border;
        BlockGetter getter;

        // Never called. It exists so that the class compiles: a subclass needs
        // a constructor chaining to its superclass's, and allocate below makes
        // the instance without either.
        private StubLevel() {
            super(null, null, null, null, false, false, 0L, 0);
        }

        void prepare() {
            this.blocks = new HashMap<>();
            this.border = new WorldBorder();
            this.getter = new BlockGetter() {
                @Override
                public BlockState getBlockState(BlockPos pos) {
                    return StubLevel.this.stateAt(pos);
                }

                @Override
                public FluidState getFluidState(BlockPos pos) {
                    return this.getBlockState(pos).getFluidState();
                }

                @Override
                public BlockEntity getBlockEntity(BlockPos pos) {
                    return null;
                }

                @Override
                public int getHeight() {
                    return 384;
                }

                @Override
                public int getMinY() {
                    return -64;
                }
            };
        }

        static long key(int x, int y, int z) {
            return ((long) x & 0x3FFFFF) << 42 | ((long) y & 0xFFFFF) << 22 | ((long) z & 0x3FFFFF);
        }

        BlockState stateAt(BlockPos pos) {
            BlockState state = this.blocks.get(key(pos.getX(), pos.getY(), pos.getZ()));
            return state == null ? Blocks.AIR.defaultBlockState() : state;
        }

        // The three duties a move actually has. The stub world holds no
        // entities, its border is the default one every world starts with, and
        // its blocks are what this harness was told about.

        // The world's extent and its fluids, answered without a dimension type.
        // An unconstructed level has no dimension registration, and the three
        // defaults that would read one are the height, the floor, and the fluid
        // lookup a travel consults before it decides it is on land. The stub
        // world holds no fluid, so every cell answers with its block's own —
        // which for every block this harness places is none.

        @Override
        public int getMinY() {
            return -64;
        }

        @Override
        public int getHeight() {
            return 384;
        }

        @Override
        public FluidState getFluidState(BlockPos pos) {
            return this.stateAt(pos).getFluidState();
        }

        // A block read that does not go through a chunk source. The move's own
        // broad phase reaches blocks through getChunkForCollisions below; the
        // tick around it reads single cells directly, and both must answer from
        // the same table.
        @Override
        public BlockState getBlockState(BlockPos pos) {
            return this.stateAt(pos);
        }

        @Override
        public List<VoxelShape> getEntityCollisions(Entity source, AABB box) {
            return List.of();
        }

        @Override
        public WorldBorder getWorldBorder() {
            return this.border;
        }

        @Override
        public BlockGetter getChunkForCollisions(int chunkX, int chunkZ) {
            return this.getter;
        }

        // Everything else. A move that reaches any of these is a move this
        // harness is not modelling, and saying so is the point.

        @Override
        public void sendBlockUpdated(BlockPos pos, BlockState old, BlockState current, int updateFlags) {
            throw new UnsupportedOperationException("sendBlockUpdated");
        }

        @Override
        public void playSeededSound(Entity source, double x, double y, double z, Holder<SoundEvent> sound,
                SoundSource category, float volume, float pitch, long seed) {
            throw new UnsupportedOperationException("playSeededSound");
        }

        @Override
        public void playSeededSound(Entity source, Entity target, Holder<SoundEvent> sound, SoundSource category,
                float volume, float pitch, long seed) {
            throw new UnsupportedOperationException("playSeededSound");
        }

        @Override
        public void explode(Entity source, DamageSource damage, ExplosionDamageCalculator calculator, double x,
                double y, double z, float radius, boolean fire, Level.ExplosionInteraction interaction,
                ParticleOptions small, ParticleOptions large, WeightedList<ExplosionParticleInfo> particles,
                Holder<SoundEvent> sound) {
            throw new UnsupportedOperationException("explode");
        }

        @Override
        public String gatherChunkSourceStats() {
            throw new UnsupportedOperationException("gatherChunkSourceStats");
        }

        @Override
        public void setRespawnData(LevelData.RespawnData respawnData) {
            throw new UnsupportedOperationException("setRespawnData");
        }

        @Override
        public LevelData.RespawnData getRespawnData() {
            throw new UnsupportedOperationException("getRespawnData");
        }

        @Override
        public Entity getEntity(int id) {
            throw new UnsupportedOperationException("getEntity");
        }

        @Override
        public Collection<EnderDragonPart> dragonParts() {
            throw new UnsupportedOperationException("dragonParts");
        }

        @Override
        public TickRateManager tickRateManager() {
            throw new UnsupportedOperationException("tickRateManager");
        }

        @Override
        public MapItemSavedData getMapData(MapId id) {
            throw new UnsupportedOperationException("getMapData");
        }

        @Override
        public void destroyBlockProgress(int id, BlockPos pos, int progress) {
            throw new UnsupportedOperationException("destroyBlockProgress");
        }

        @Override
        public Scoreboard getScoreboard() {
            throw new UnsupportedOperationException("getScoreboard");
        }

        @Override
        public RecipeAccess recipeAccess() {
            throw new UnsupportedOperationException("recipeAccess");
        }

        @Override
        protected LevelEntityGetter<Entity> getEntities() {
            throw new UnsupportedOperationException("getEntities");
        }

        @Override
        public ClockManager clockManager() {
            throw new UnsupportedOperationException("clockManager");
        }

        @Override
        public EnvironmentAttributeSystem environmentAttributes() {
            throw new UnsupportedOperationException("environmentAttributes");
        }

        @Override
        public PotionBrewing potionBrewing() {
            throw new UnsupportedOperationException("potionBrewing");
        }

        @Override
        public FuelValues fuelValues() {
            throw new UnsupportedOperationException("fuelValues");
        }

        @Override
        public int getSeaLevel() {
            throw new UnsupportedOperationException("getSeaLevel");
        }

        @Override
        public List<? extends Player> players() {
            throw new UnsupportedOperationException("players");
        }

        @Override
        public Holder<Biome> getUncachedNoiseBiome(int x, int y, int z) {
            throw new UnsupportedOperationException("getUncachedNoiseBiome");
        }

        @Override
        public LevelTickAccess<Block> getBlockTicks() {
            throw new UnsupportedOperationException("getBlockTicks");
        }

        @Override
        public LevelTickAccess<Fluid> getFluidTicks() {
            throw new UnsupportedOperationException("getFluidTicks");
        }

        @Override
        public FeatureFlagSet enabledFeatures() {
            throw new UnsupportedOperationException("enabledFeatures");
        }

        @Override
        public ChunkSource getChunkSource() {
            throw new UnsupportedOperationException("getChunkSource");
        }

        @Override
        public void levelEvent(Entity source, int type, BlockPos pos, int data) {
            throw new UnsupportedOperationException("levelEvent");
        }

        @Override
        public void gameEvent(Holder<GameEvent> event, Vec3 at, GameEvent.Context context) {
            throw new UnsupportedOperationException("gameEvent");
        }
    }

    /**
     * StubEntity is the smallest concrete Entity that can be collided.
     *
     * It is built as a player, because the step height under test is a player's
     * and the entity's own type decides its collision context. The step height
     * is a field rather than the attribute the game reads, so that a case can
     * ask for a body with no step at all.
     */
    static final class StubEntity extends Entity {
        float step;

        StubEntity(Level level) {
            super(EntityType.PLAYER, level);
        }

        @Override
        public float maxUpStep() {
            return this.step;
        }

        @Override
        protected void defineSynchedData(net.minecraft.network.syncher.SynchedEntityData.Builder builder) {
        }

        @Override
        protected void readAdditionalSaveData(ValueInput input) {
        }

        @Override
        protected void addAdditionalSaveData(ValueOutput output) {
        }

        @Override
        public boolean hurtServer(net.minecraft.server.level.ServerLevel level, DamageSource source, float damage) {
            return false;
        }
    }

    public static void main(String[] arguments) throws Exception {
        // Held before the game starts. Bootstrapping installs a logging
        // framework that takes System.out over, and an answer written through
        // that arrives wrapped in log decoration and parses as nothing.
        PrintStream out = System.out;

        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();

        Method collide = Entity.class.getDeclaredMethod("collide", Vec3.class);
        collide.setAccessible(true);

        StubLevel level = allocate(StubLevel.class);
        level.prepare();
        StubEntity entity = new StubEntity(level);

        BufferedReader in = new BufferedReader(new InputStreamReader(System.in, "UTF-8"));

        String line;
        while ((line = in.readLine()) != null) {
            line = line.trim();
            if (line.isEmpty()) {
                continue;
            }

            String[] parts = line.split("\\s+");
            switch (parts[0]) {
                case "C":
                    level.blocks.clear();
                    break;
                case "B": {
                    int cellX = Integer.parseInt(parts[1]);
                    int cellY = Integer.parseInt(parts[2]);
                    int cellZ = Integer.parseInt(parts[3]);
                    level.blocks.put(StubLevel.key(cellX, cellY, cellZ), state(parts[4]));
                    break;
                }
                case "Q": {
                    // The shape a block contributes, in its own cell's
                    // coordinates. Reported as the boxes the game stores rather
                    // than the ones the block was written as, which for a
                    // gridded shape are not the same list.
                    StringBuilder answer = new StringBuilder();
                    List<AABB> boxes = shape(parts[1]).toAabbs();
                    answer.append(boxes.size());
                    for (AABB box : boxes) {
                        answer.append(' ').append(hex(box.minX)).append(' ').append(hex(box.minY))
                                .append(' ').append(hex(box.minZ)).append(' ').append(hex(box.maxX))
                                .append(' ').append(hex(box.maxY)).append(' ').append(hex(box.maxZ));
                    }
                    out.println(answer);
                    break;
                }
                case "Y": {
                    // The Y coordinates the shape offers, which is the list a
                    // step-up collects its candidates from.
                    it.unimi.dsi.fastutil.doubles.DoubleList coords =
                            shape(parts[1]).getCoords(net.minecraft.core.Direction.Axis.Y);
                    StringBuilder answer = new StringBuilder();
                    answer.append(coords.size());
                    for (double coord : coords) {
                        answer.append(' ').append(hex(coord));
                    }
                    out.println(answer);
                    break;
                }
                case "M": {
                    AABB body = new AABB(
                            Double.parseDouble(parts[1]),
                            Double.parseDouble(parts[2]),
                            Double.parseDouble(parts[3]),
                            Double.parseDouble(parts[4]),
                            Double.parseDouble(parts[5]),
                            Double.parseDouble(parts[6]));
                    Vec3 motion = new Vec3(
                            Double.parseDouble(parts[7]),
                            Double.parseDouble(parts[8]),
                            Double.parseDouble(parts[9]));
                    // The position is set as well as the box, and before it,
                    // because setPos derives a box from the entity's dimensions
                    // and would overwrite the one under test. Only the world
                    // border reads the position during a move, and it reads it
                    // to ask how close the body is to the edge of the world.
                    entity.setPos(
                            (body.minX + body.maxX) / 2.0,
                            body.minY,
                            (body.minZ + body.maxZ) / 2.0);
                    entity.setBoundingBox(body);
                    entity.setOnGround(Boolean.parseBoolean(parts[10]));
                    entity.step = Float.parseFloat(parts[11]);

                    Vec3 applied = (Vec3) collide.invoke(entity, motion);
                    out.println(hex(applied.x) + " " + hex(applied.y) + " " + hex(applied.z));
                    break;
                }
                default:
                    throw new IllegalArgumentException("unknown command: " + parts[0]);
            }
        }

        out.flush();
    }

    /** state returns a block's default state, or fails on an unknown name. */
    private static BlockState state(String name) {
        Identifier id = Identifier.withDefaultNamespace(name);
        Block block = BuiltInRegistries.BLOCK.getValue(id);
        if (block == Blocks.AIR && !name.equals("air")) {
            throw new IllegalArgumentException("unknown block: " + name);
        }

        return block.defaultBlockState();
    }

    /**
     * shape returns a block's collision shape in its own cell.
     *
     * It is read with no surroundings and no collision context, so a block whose
     * shape depends on either would be reported wrongly here. The Go side asks
     * only about blocks whose shape is a fact about the block.
     */
    private static VoxelShape shape(String name) {
        return state(name).getCollisionShape(EmptyBlockGetter.INSTANCE, BlockPos.ZERO);
    }

    /**
     * allocate makes an instance without running a constructor.
     *
     * MovementOracle26 uses it for the same level, so that the two harnesses
     * cannot disagree about what a world is.
     *
     * The level's constructor is unreachable from here, as the file comment
     * explains, and this is the standard way of skipping one: the same
     * mechanism serialization uses to rebuild an object without its
     * constructor's side effects.
     */
    @SuppressWarnings("unchecked")
    static <T> T allocate(Class<T> type) throws Exception {
        Constructor<Object> base = Object.class.getDeclaredConstructor();
        Constructor<?> bypass = sun.reflect.ReflectionFactory.getReflectionFactory()
                .newConstructorForSerialization(type, base);

        return (T) bypass.newInstance();
    }

    private static String hex(double value) {
        return Double.toHexString(value);
    }
}
