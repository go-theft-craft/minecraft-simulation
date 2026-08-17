// MovementOracle runs the real movement tick of a Java Edition 1.8.9 server
// against a world made of blocks this harness is told about, so that whole
// trajectories can be compared against the game's own algorithm rather than
// against a reading of it.
//
// Nothing here reimplements game logic. The jump counter, the jump impulse, the
// motion threshold, the input decay, the friction lookup, the heading, the
// collision resolve, gravity, and the two drags are all executed by Mojang's
// bytecode inside EntityLivingBase.onLivingUpdate. This file supplies a block
// lookup, a minimal living entity, and a text protocol.
//
// The world stub is MoveOracle's, deliberately: the collision oracle and this
// one must not be able to disagree about what the world is.
//
// The overrides on the entity remove everything that is not movement and
// nothing that is. Nearby-entity pushing is removed because this milestone has
// no second entity and the stub world has no chunk provider to find one with;
// the fall-state callback is removed because fall damage, water, and step
// sounds are not movement; and the four abstract equipment accessors are
// answered emptily because a body with no inventory still moves.
//
// The entity persists between ticks, which is the point: the jump counter it
// carries is private to EntityLivingBase, so the only honest way to check a
// counter is to let the game keep it.
//
// Protocol, one command per line on standard input:
//   C                     forget every block
//   B x y z name          place a block by its registry name
//   S x y z yaw pitch onGround moveSpeed jumpFactor    spawn a body
//   T strafe forward yaw pitch jump sprint             run one whole tick
// S and T each write one line: body[6] motion[3] onGround collidedHorizontally
// collidedVertically. Every double crosses the boundary as a hexadecimal float
// literal, and every float is parsed from its decimal text at single width.
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintWriter;

import net.minecraft.block.Block;
import net.minecraft.block.state.IBlockState;
import net.minecraft.entity.EntityLivingBase;
import net.minecraft.init.Blocks;
import net.minecraft.init.Bootstrap;
import net.minecraft.item.ItemStack;
import net.minecraft.util.AxisAlignedBB;
import net.minecraft.util.BlockPos;
import net.minecraft.world.World;

public final class MovementOracle {
    /** StubLiving is the smallest concrete EntityLivingBase that can be ticked. */
    static final class StubLiving extends EntityLivingBase {
        StubLiving(World world) {
            super(world);
        }

        @Override
        public ItemStack getHeldItem() {
            return null;
        }

        @Override
        public ItemStack getEquipmentInSlot(int slot) {
            return null;
        }

        @Override
        public void setCurrentItemOrArmor(int slot, ItemStack stack) {
        }

        @Override
        public ItemStack[] getInventory() {
            return new ItemStack[0];
        }

        @Override
        protected void collideWithNearbyEntities() {
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

        @Override
        protected boolean canTriggerWalking() {
            return false;
        }
    }

    public static void main(String[] args) throws Exception {
        Bootstrap.register();

        BufferedReader in = new BufferedReader(new InputStreamReader(System.in, "UTF-8"));
        PrintWriter out = new PrintWriter(new java.io.OutputStreamWriter(System.out, "UTF-8"));

        MoveOracle.StubWorld world = new MoveOracle.StubWorld();
        world.air = Blocks.air.getDefaultState();

        StubLiving body = null;

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
                    Block block = Block.getBlockFromName(parts[4]);
                    if (block == null) {
                        throw new IllegalArgumentException("unknown block: " + parts[4]);
                    }
                    IBlockState state = block.getDefaultState();
                    if (block == Blocks.air) {
                        world.blocks.remove(MoveOracle.StubWorld.key(x, y, z));
                    } else {
                        world.blocks.put(MoveOracle.StubWorld.key(x, y, z), state);
                    }
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
                    float jumpFactor = Float.parseFloat(parts[at++]);

                    body = new StubLiving(world);
                    body.setPositionAndRotation(x, y, z, yaw, pitch);
                    body.onGround = onGround;
                    body.setAIMoveSpeed(moveSpeed);
                    body.jumpMovementFactor = jumpFactor;
                    report(out, body);
                    break;
                }
                case "T": {
                    if (body == null) {
                        throw new IllegalStateException("a tick was asked for before any body was spawned");
                    }
                    int at = 1;
                    body.moveStrafing = Float.parseFloat(parts[at++]);
                    body.moveForward = Float.parseFloat(parts[at++]);
                    body.rotationYaw = Float.parseFloat(parts[at++]);
                    body.rotationPitch = Float.parseFloat(parts[at++]);
                    body.setJumping(Boolean.parseBoolean(parts[at++]));
                    body.setSprinting(Boolean.parseBoolean(parts[at++]));

                    body.onLivingUpdate();
                    report(out, body);
                    break;
                }
                default:
                    throw new IllegalArgumentException("unknown command: " + parts[0]);
            }
        }

        out.flush();
    }

    private static void report(PrintWriter out, StubLiving body) {
        AxisAlignedBB box = body.getEntityBoundingBox();
        out.println(hex(box.minX) + " " + hex(box.minY) + " " + hex(box.minZ)
                + " " + hex(box.maxX) + " " + hex(box.maxY) + " " + hex(box.maxZ)
                + " " + hex(body.motionX) + " " + hex(body.motionY) + " " + hex(body.motionZ)
                + " " + body.onGround
                + " " + body.isCollidedHorizontally
                + " " + body.isCollidedVertically);
    }

    private static String hex(double value) {
        return Double.toHexString(value);
    }
}
