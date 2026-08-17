// ShapeOracle26 runs the real collision arithmetic of a Java Edition 26.1.2
// server against boxes this harness is told about, so that the version-selected
// collision variant can be compared against the game's own algorithm rather
// than against a reading of it.
//
// Nothing here reimplements game logic. The per-axis clamp, the tolerances, the
// motion-dependent axis order, and the list of candidate step-up heights are all
// executed by Mojang's bytecode. This file supplies shapes, a text protocol, and
// two reflective handles.
//
// Two of the three entry points are private statics on Entity. They are reached
// by reflection rather than by driving an entity, because driving an entity in
// this version needs a Level, and a Level here is twenty abstract methods and a
// registry-backed constructor — a task of its own, and one this check does not
// need. What it costs is that the step-up *assembly* around these calls is not
// covered here; the movement oracle covers that once the world stub exists.
//
// This version ships unobfuscated, so the harness compiles against the real
// names and javac checks it against the jar it will run on. A renamed method
// fails to compile instead of throwing halfway through a run.
//
// Protocol, one command per line on standard input:
//   C                                    forget every shape
//   S cell[3] minX minY minZ maxX maxY maxZ   add a collider, block-local then moved
//   A axis body[6] distance              one axis clamp
//   R body[6] dx dy dz                   the whole multi-axis resolve
//   H body[6] maxStep skip               the candidate step-up heights
//   G                                    the Y coordinates of every shape
// A and R write the resulting motion, H writes a count and the heights. Every
// double crosses the boundary as a hexadecimal float literal, and every float
// is parsed from its decimal text at single width.
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintStream;
import java.lang.reflect.Method;
import java.util.ArrayList;
import java.util.List;

import net.minecraft.SharedConstants;
import net.minecraft.core.Direction;
import net.minecraft.server.Bootstrap;
import net.minecraft.world.entity.Entity;
import net.minecraft.world.phys.AABB;
import net.minecraft.world.phys.Vec3;
import net.minecraft.world.phys.shapes.Shapes;
import net.minecraft.world.phys.shapes.VoxelShape;

public final class ShapeOracle26 {
    public static void main(String[] arguments) throws Exception {
        // Held before the game starts. Bootstrapping installs a logging
        // framework that takes System.out over, and an answer written through
        // that arrives wrapped in log decoration and parses as nothing.
        PrintStream out = System.out;

        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();

        Method collideWithShapes = Entity.class.getDeclaredMethod(
                "collideWithShapes", Vec3.class, AABB.class, List.class);
        collideWithShapes.setAccessible(true);
        Method collectCandidateStepUpHeights = Entity.class.getDeclaredMethod(
                "collectCandidateStepUpHeights", AABB.class, List.class, float.class, float.class);
        collectCandidateStepUpHeights.setAccessible(true);

        BufferedReader in = new BufferedReader(new InputStreamReader(System.in, "UTF-8"));
        List<VoxelShape> shapes = new ArrayList<>();

        String line;
        while ((line = in.readLine()) != null) {
            line = line.trim();
            if (line.isEmpty()) {
                continue;
            }

            String[] parts = line.split("\\s+");
            switch (parts[0]) {
                case "C":
                    shapes.clear();
                    break;
                case "S": {
                    // Built the way the game builds a block's collider: from a
                    // block-local box, then moved into its cell. Creating it
                    // directly in world coordinates would skip the
                    // discretization, and the discretization is the thing the
                    // candidate heights are collected from.
                    int cellX = Integer.parseInt(parts[1]);
                    int cellY = Integer.parseInt(parts[2]);
                    int cellZ = Integer.parseInt(parts[3]);
                    shapes.add(Shapes.create(box(parts, 4)).move(cellX, cellY, cellZ));
                    break;
                }
                case "A": {
                    Direction.Axis axis = axis(parts[1]);
                    AABB body = box(parts, 2);
                    double distance = Double.parseDouble(parts[8]);
                    out.println(hex(Shapes.collide(axis, body, shapes, distance)));
                    break;
                }
                case "R": {
                    AABB body = box(parts, 1);
                    Vec3 motion = new Vec3(
                            Double.parseDouble(parts[7]),
                            Double.parseDouble(parts[8]),
                            Double.parseDouble(parts[9]));
                    Vec3 resolved = (Vec3)collideWithShapes.invoke(null, motion, body, shapes);
                    out.println(hex(resolved.x) + " " + hex(resolved.y) + " " + hex(resolved.z));
                    break;
                }
                case "G": {
                    // The Y coordinates of each shape, which is what the
                    // step-up candidates are collected from.
                    StringBuilder answer = new StringBuilder();
                    for (VoxelShape shape : shapes) {
                        answer.append('[');
                        boolean first = true;
                        for (double coord : shape.getCoords(Direction.Axis.Y)) {
                            if (!first) {
                                answer.append(' ');
                            }
                            answer.append(Double.toString(coord));
                            first = false;
                        }
                        answer.append(']');
                    }
                    out.println(answer);
                    break;
                }
                case "H": {
                    AABB body = box(parts, 1);
                    float maxStep = Float.parseFloat(parts[7]);
                    float skip = Float.parseFloat(parts[8]);
                    float[] heights = (float[])collectCandidateStepUpHeights.invoke(
                            null, body, shapes, maxStep, skip);
                    StringBuilder answer = new StringBuilder();
                    answer.append(heights.length);
                    for (float height : heights) {
                        answer.append(' ').append(Float.toString(height));
                    }
                    out.println(answer);
                    break;
                }
                default:
                    throw new IllegalArgumentException("unknown command: " + parts[0]);
            }
        }

        out.flush();
    }

    private static AABB box(String[] parts, int at) {
        return new AABB(
                Double.parseDouble(parts[at]),
                Double.parseDouble(parts[at + 1]),
                Double.parseDouble(parts[at + 2]),
                Double.parseDouble(parts[at + 3]),
                Double.parseDouble(parts[at + 4]),
                Double.parseDouble(parts[at + 5]));
    }

    private static Direction.Axis axis(String name) {
        switch (name) {
            case "X":
                return Direction.Axis.X;
            case "Y":
                return Direction.Axis.Y;
            case "Z":
                return Direction.Axis.Z;
            default:
                throw new IllegalArgumentException("unknown axis: " + name);
        }
    }

    private static String hex(double value) {
        return Double.toHexString(value);
    }
}
