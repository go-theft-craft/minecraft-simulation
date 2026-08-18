// MovementOracle26 runs the real movement tick of a Java Edition 26.1.2 server
// against a world made of blocks this harness is told about, so that whole
// trajectories can be compared against the game's own algorithm rather than
// against a reading of it.
//
// Nothing here reimplements a movement rule. The jump counter, the motion
// threshold, the jump impulse, the friction lookup, the acceleration, the
// heading, the shape-based collision with its step-up, the block callback that
// stops a landing body, gravity, and the two drags are all executed by Mojang's
// bytecode inside LivingEntity.aiStep. This file supplies a block lookup, a
// minimal living entity, and a text protocol.
//
// The world stub is MoveOracle26's, deliberately: the collision oracle and this
// one must not be able to disagree about what the world is.
//
// **The body is a living entity of the player type, not a Player.** The type is
// what the tick branches on — the motion threshold is a player rule, and the
// attribute supplier a player type carries is where the jump strength, the
// movement speed, and the step height come from — while the Player class adds an
// inventory, an entity pickup that needs a populated level, and a re-read of the
// speed attribute at the end of every tick. A server's Player is also
// client-authoritative and does not travel at all: the tick under test is the one
// a client runs for its own body, and a living entity of the player type runs
// exactly that code.
//
// Four overrides remove what is not movement: fall damage, movement sounds,
// entity pushing, and the input decay. The last one is not a removal of a rule
// but a move of one — the client's own player replaces the shared decay with a
// decay, a sneak factor, and a stretch onto the unit square, all in a class this
// jar does not carry — so the Go side sends axes it has already shaped, and the I
// command below checks that shaping against this jar's own arithmetic.
//
// One override restores a rule instead: a player's airborne speed is 0.02, or
// 0.026 while sprinting, and that pair is declared in the Player class. A living
// entity answers 0.02 either way, which would make every sprint-jump comparison
// meaningless. It is transcribed, and it is the only game logic in this file.
//
// The entity persists between ticks, which is the point: the jump counter, the
// fall distance, and the supporting block it carries are private to the game's
// classes, so the only honest way to check them is to let the game keep them.
//
// This version ships unobfuscated, so the harness compiles against the real
// names and javac checks it against the jar it will run on. A renamed method
// fails to compile instead of throwing halfway through a run.
//
// Protocol, one command per line on standard input:
//   C                                          forget every block
//   B cell[3] name                             place a block by its registry name
//   S x y z yaw pitch onGround moveSpeed       spawn a body, reporting its state
//   T strafe forward yaw pitch jump sprint     run one whole tick
//   I strafe forward sneaking                  the client's input shaping
// S and T each write body[6] motion[3] onGround horizontalCollision
// verticalCollision. I writes the two shaped axes. Every double crosses the
// boundary as a hexadecimal float literal, and every float is parsed from its
// decimal text at single width.
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintStream;

import net.minecraft.SharedConstants;
import net.minecraft.core.BlockPos;
import net.minecraft.core.registries.BuiltInRegistries;
import net.minecraft.resources.Identifier;
import net.minecraft.server.Bootstrap;
import net.minecraft.util.Mth;
import net.minecraft.world.damagesource.DamageSource;
import net.minecraft.world.entity.Entity;
import net.minecraft.world.entity.EntityType;
import net.minecraft.world.entity.LivingEntity;
import net.minecraft.world.level.Level;
import net.minecraft.world.level.block.Block;
import net.minecraft.world.level.block.Blocks;
import net.minecraft.world.level.block.state.BlockState;
import net.minecraft.world.level.storage.ValueInput;
import net.minecraft.world.level.storage.ValueOutput;
import net.minecraft.world.phys.AABB;
import net.minecraft.world.phys.Vec2;

public final class MovementOracle26 {
    /** StubLiving is the smallest living entity of the player type that can be ticked. */
    static final class StubLiving extends LivingEntity {
        StubLiving(Level level) {
            super(EntityType.PLAYER, level);
        }

        /**
         * The client's airborne speed, which is a Player declaration rather than a
         * living entity's. Transcribed, and named as such: it is the one rule in
         * this file the jar is not asked for.
         */
        @Override
        protected float getFlyingSpeed() {
            return this.isSprinting() ? 0.025999999F : 0.02F;
        }

        /**
         * The input decay, removed. A client's own player replaces it with a
         * shaping this jar does not carry, so the axes arrive already shaped and
         * the tick must not decay them a second time.
         */
        @Override
        protected void applyInput() {
        }

        // Not movement: fall damage, the sounds and game events a step emits, and
        // pushing other entities. The stub world holds no entities and no damage
        // sources, and none of the three feeds back into a trajectory.

        @Override
        protected void checkFallDamage(double y, boolean onGround, BlockState state, BlockPos pos) {
        }

        @Override
        protected Entity.MovementEmission getMovementEmission() {
            return Entity.MovementEmission.NONE;
        }

        @Override
        protected void pushEntities() {
        }

        /**
         * The effects the blocks a body passed through apply to it — cobweb,
         * powder snow, fire, the rain check inside them. None is land movement,
         * and the pass reads weather out of a level record this stub has none of.
         */
        @Override
        public void applyEffectsFromBlocks() {
        }

        @Override
        public net.minecraft.world.entity.HumanoidArm getMainArm() {
            return net.minecraft.world.entity.HumanoidArm.RIGHT;
        }

        @Override
        protected void defineSynchedData(net.minecraft.network.syncher.SynchedEntityData.Builder builder) {
            super.defineSynchedData(builder);
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

        MoveOracle26.StubLevel level = MoveOracle26.allocate(MoveOracle26.StubLevel.class);
        level.prepare();

        StubLiving body = null;

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
                    level.blocks.put(MoveOracle26.StubLevel.key(cellX, cellY, cellZ), state(parts[4]));
                    break;
                }
                case "S": {
                    int at = 1;
                    double x = Double.parseDouble(parts[at++]);
                    double y = Double.parseDouble(parts[at++]);
                    double z = Double.parseDouble(parts[at++]);
                    float yaw = Float.parseFloat(parts[at++]);
                    float pitch = Float.parseFloat(parts[at++]);
                    boolean onGround = Boolean.parseBoolean(parts[at++]);
                    float moveSpeed = Float.parseFloat(parts[at++]);

                    body = new StubLiving(level);
                    body.snapTo(x, y, z, yaw, pitch);
                    body.setOnGround(onGround);
                    body.setSpeed(moveSpeed);
                    report(out, body);
                    break;
                }
                case "T": {
                    if (body == null) {
                        throw new IllegalStateException("a tick was asked for before any body was spawned");
                    }
                    int at = 1;
                    body.xxa = Float.parseFloat(parts[at++]);
                    body.zza = Float.parseFloat(parts[at++]);
                    body.setYRot(Float.parseFloat(parts[at++]));
                    body.setXRot(Float.parseFloat(parts[at++]));
                    body.setJumping(Boolean.parseBoolean(parts[at++]));
                    body.setSprinting(Boolean.parseBoolean(parts[at++]));

                    body.aiStep();
                    report(out, body);
                    break;
                }
                case "I": {
                    // The client's input shaping, composed here out of the jar's
                    // own Vec2 and Mth so that the widths are the game's. The
                    // composition is transcribed from the client class; the
                    // arithmetic is not.
                    Vec2 input = new Vec2(Float.parseFloat(parts[1]), Float.parseFloat(parts[2]));
                    boolean sneaking = Boolean.parseBoolean(parts[3]);
                    Vec2 shaped = shape(input, sneaking);
                    out.println(hexFloat(shaped.x) + " " + hexFloat(shaped.y));
                    break;
                }
                default:
                    throw new IllegalArgumentException("unknown command: " + parts[0]);
            }
        }

        out.flush();
    }

    /** shape is the client's modifyInput, over this jar's arithmetic. */
    private static Vec2 shape(Vec2 input, boolean sneaking) {
        if (input.lengthSquared() == 0.0F) {
            return input;
        }

        Vec2 shaped = input.scale(0.98F);
        if (sneaking) {
            shaped = shaped.scale(0.3F);
        }

        float length = shaped.length();
        if (length <= 0.0F) {
            return shaped;
        }

        Vec2 direction = shaped.scale(1.0F / length);
        float directionX = Math.abs(direction.x);
        float directionY = Math.abs(direction.y);
        float tan = directionY > directionX ? directionX / directionY : directionY / directionX;
        float toUnitSquare = Mth.sqrt(1.0F + Mth.square(tan));

        return direction.scale(Math.min(length * toUnitSquare, 1.0F));
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

    private static void report(PrintStream out, StubLiving body) {
        AABB box = body.getBoundingBox();
        out.println(hex(box.minX) + " " + hex(box.minY) + " " + hex(box.minZ)
                + " " + hex(box.maxX) + " " + hex(box.maxY) + " " + hex(box.maxZ)
                + " " + hex(body.getDeltaMovement().x)
                + " " + hex(body.getDeltaMovement().y)
                + " " + hex(body.getDeltaMovement().z)
                + " " + body.onGround()
                + " " + body.horizontalCollision
                + " " + body.verticalCollision);
    }

    private static String hex(double value) {
        return Double.toHexString(value);
    }

    private static String hexFloat(float value) {
        return Float.toHexString(value);
    }
}
