// Loaded26 starts a Java Edition 26.1.2 server's data far enough to ask it
// questions, and no further.
//
// Bootstrapping the game is not enough for anything that touches an item or an
// enchantment. 26.1 keeps a tool's speeds in an item component and its
// enchantments in a data-driven registry, and a server binds both when it loads
// its data pack: until it does, constructing any ItemStack throws "Components
// not bound yet", and binding them against the built-in registries alone fails
// because an item component reads damage-type tags.
//
// The sequence below is WorldLoader.load's, in its order. It is shared rather
// than copied because two harnesses need it, and two copies of a startup
// sequence are two things that can drift into loading different games.
import java.util.List;

import net.minecraft.commands.Commands;
import net.minecraft.core.LayeredRegistryAccess;
import net.minecraft.core.Registry;
import net.minecraft.core.RegistryAccess;
import net.minecraft.resources.RegistryDataLoader;
import net.minecraft.server.RegistryLayer;
import net.minecraft.server.ReloadableServerResources;
import net.minecraft.server.packs.PackType;
import net.minecraft.server.packs.repository.PackRepository;
import net.minecraft.server.packs.repository.ServerPacksSource;
import net.minecraft.server.packs.resources.CloseableResourceManager;
import net.minecraft.server.packs.resources.MultiPackResourceManager;
import net.minecraft.server.permissions.PermissionSet;
import net.minecraft.tags.TagLoader;
import net.minecraft.util.Util;
import net.minecraft.world.flag.FeatureFlags;

public final class Loaded26 {
    private Loaded26() {
    }

    /**
     * load runs the server's own data load and returns what it produced.
     *
     * The returned resources are what a caller reads a data-driven registry
     * from — an enchantment, say — and the components every item carries are
     * bound as a side effect of the last call.
     */
    public static ReloadableServerResources load() {
        PackRepository packs = ServerPacksSource.createVanillaTrustedRepository();
        packs.reload();
        // The vanilla pack alone. reload discovers and selects nothing, and the
        // three experimental packs it discovers beside it — minecart
        // improvements, redstone experiments, trade rebalance — are not the game
        // a player is playing.
        packs.setSelected(List.of("vanilla"));
        CloseableResourceManager resources =
                new MultiPackResourceManager(PackType.SERVER_DATA, packs.openAllSelected());

        LayeredRegistryAccess<RegistryLayer> layers = RegistryLayer.createRegistryAccess();
        List<Registry.PendingTags<?>> pendingTags = TagLoader.loadTagsForExistingRegistries(
                resources, layers.getLayer(RegistryLayer.STATIC));
        RegistryAccess.Frozen worldgen = RegistryDataLoader.load(
                resources,
                TagLoader.buildUpdatedLookups(layers.getAccessForLoading(RegistryLayer.WORLDGEN), pendingTags),
                RegistryDataLoader.WORLDGEN_REGISTRIES,
                Util.backgroundExecutor()).join();
        layers = layers.replaceFrom(RegistryLayer.WORLDGEN, worldgen);

        ReloadableServerResources loaded = ReloadableServerResources.loadResources(
                resources, layers, pendingTags, FeatureFlags.DEFAULT_FLAGS,
                Commands.CommandSelection.DEDICATED, PermissionSet.ALL_PERMISSIONS,
                Util.backgroundExecutor(), Runnable::run).join();
        loaded.updateComponentsAndStaticRegistryTags();

        return loaded;
    }
}
