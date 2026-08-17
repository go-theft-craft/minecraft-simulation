// AabbOracle answers geometry questions using the collision primitives of a
// real Java Edition 1.8.9 server jar, so that Go results can be compared
// against the game rather than against a second reading of it.
//
// It reads one case per line from standard input and writes one answer per
// line to standard output. Every double crosses the boundary as a hexadecimal
// float literal, because a decimal rendering would hide exactly the low-bit
// differences this harness exists to find.
//
// Cases:
//   X block[6] mover[6] motion   -> calculateXOffset
//   Y block[6] mover[6] motion   -> calculateYOffset
//   Z block[6] mover[6] motion   -> calculateZOffset
//   A box[6] dx dy dz            -> addCoord, six doubles
//   O box[6] dx dy dz            -> offset, six doubles
//   I a[6] b[6]                  -> intersectsWith, "true" or "false"
//
// This file contains no game source. It only calls the jar's public methods
// through reflection-free direct references resolved at compile time against
// the locally prepared, unredistributed jar.
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintWriter;

import net.minecraft.util.AxisAlignedBB;

public final class AabbOracle {
    public static void main(String[] args) throws Exception {
        BufferedReader in = new BufferedReader(new InputStreamReader(System.in, "UTF-8"));
        PrintWriter out = new PrintWriter(new java.io.OutputStreamWriter(System.out, "UTF-8"));

        String line;
        while ((line = in.readLine()) != null) {
            line = line.trim();
            if (line.isEmpty()) {
                continue;
            }

            String[] parts = line.split("\\s+");
            String op = parts[0];

            switch (op) {
                case "X":
                case "Y":
                case "Z": {
                    AxisAlignedBB block = box(parts, 1);
                    AxisAlignedBB mover = box(parts, 7);
                    double motion = Double.parseDouble(parts[13]);
                    double result;
                    if (op.equals("X")) {
                        result = block.calculateXOffset(mover, motion);
                    } else if (op.equals("Y")) {
                        result = block.calculateYOffset(mover, motion);
                    } else {
                        result = block.calculateZOffset(mover, motion);
                    }
                    out.println(Double.toHexString(result));
                    break;
                }
                case "A":
                case "O": {
                    AxisAlignedBB b = box(parts, 1);
                    double dx = Double.parseDouble(parts[7]);
                    double dy = Double.parseDouble(parts[8]);
                    double dz = Double.parseDouble(parts[9]);
                    AxisAlignedBB result = op.equals("A") ? b.addCoord(dx, dy, dz) : b.offset(dx, dy, dz);
                    out.println(render(result));
                    break;
                }
                case "I": {
                    AxisAlignedBB a = box(parts, 1);
                    AxisAlignedBB b = box(parts, 7);
                    out.println(a.intersectsWith(b) ? "true" : "false");
                    break;
                }
                default:
                    throw new IllegalArgumentException("unknown op: " + op);
            }
        }

        out.flush();
    }

    private static AxisAlignedBB box(String[] parts, int at) {
        return new AxisAlignedBB(
                Double.parseDouble(parts[at]),
                Double.parseDouble(parts[at + 1]),
                Double.parseDouble(parts[at + 2]),
                Double.parseDouble(parts[at + 3]),
                Double.parseDouble(parts[at + 4]),
                Double.parseDouble(parts[at + 5]));
    }

    private static String render(AxisAlignedBB b) {
        return Double.toHexString(b.minX) + " " + Double.toHexString(b.minY) + " " + Double.toHexString(b.minZ)
                + " " + Double.toHexString(b.maxX) + " " + Double.toHexString(b.maxY) + " "
                + Double.toHexString(b.maxZ);
    }
}
