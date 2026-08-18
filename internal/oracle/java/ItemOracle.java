// ItemOracle runs the real tick of a Java Edition 1.8.9 dropped item against a
// world made of blocks this harness is told about, so that whole trajectories
// can be compared against the game's own algorithm rather than against a
// reading of it.
//
// Nothing here reimplements a movement rule. The gravity subtracted before the
// move, the collision sweep, the friction taken from the block the item ended
// the tick on, the two drags, and the bounce are all executed by Mojang's
// bytecode inside EntityItem.onUpdate. This file supplies a block lookup, an
// item entity with the parts that are not movement removed, and a text
// protocol.
//
// The world stub is MoveOracle's, deliberately: the collision oracle and this
// one must not be able to disagree about what the world is.
//
// The overrides remove what is not movement, and each one is a thing the stub
// world cannot answer rather than a rule being dodged. Water handling needs
// fluid states this milestone does not model and would branch the tick away
// from the land path under test; wetness and sound are not movement at all.
//
// Merging with a nearby item is not overridden, because it cannot be: the
// method is private in EntityItem. It runs against the stub world's own entity
// lookup, which finds nothing, so the item under test merges with nobody.
//
// The entity persists between ticks, which is the point: the item's own
// onGround flag and its motion are what the next tick reads.
//
// Protocol, one command per line on standard input:
//   C                          forget every block
//   B x y z name               place a block by its registry name
//   S x y z mx my mz           spawn an item with a position and a motion
//   T                          run one whole tick
//
// Every reply is the item's box, its motion, and its flags, each double as a
// hexadecimal float so that no digit is lost on the way out.
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintWriter;

import net.minecraft.block.Block;
import net.minecraft.block.state.IBlockState;
import net.minecraft.entity.item.EntityItem;
import net.minecraft.init.Blocks;
import net.minecraft.init.Bootstrap;
import net.minecraft.init.Items;
import net.minecraft.item.ItemStack;
import net.minecraft.util.AxisAlignedBB;
import net.minecraft.world.World;

public final class ItemOracle {
    static final class StubItem extends EntityItem {
        StubItem(World world, double x, double y, double z) {
            super(world, x, y, z, new ItemStack(Items.stick));
        }

        @Override
        public boolean handleWaterMovement() {
            return false;
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

        StubItem body = null;

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
                    double mx = Double.parseDouble(parts[at++]);
                    double my = Double.parseDouble(parts[at++]);
                    double mz = Double.parseDouble(parts[at++]);

                    body = new StubItem(world, x, y, z);
                    body.motionX = mx;
                    body.motionY = my;
                    body.motionZ = mz;
                    // The pickup delay counts down inside the tick and has no
                    // effect on motion; it is zeroed so a reader is not left
                    // wondering whether it did.
                    body.setDefaultPickupDelay();
                    report(out, body);
                    break;
                }
                case "T": {
                    if (body == null) {
                        throw new IllegalStateException("a tick was asked for before any item was spawned");
                    }
                    body.onUpdate();
                    report(out, body);
                    break;
                }
                default:
                    throw new IllegalArgumentException("unknown command: " + parts[0]);
            }
        }

        out.flush();
    }

    private static void report(PrintWriter out, StubItem body) {
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
