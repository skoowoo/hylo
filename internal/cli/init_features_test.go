package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/plugins/search"
	"github.com/hardhacker/hylo/internal/storage"
)

func runInit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := newInitCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs(append([]string{root}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func readInitState(t *testing.T, root string) vaultInitState {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".hylo", "init.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state vaultInitState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func writeInitFixture(t *testing.T, root, name, content string) {
	t.Helper()
	dest := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestInitFeaturesCombineSkipAndForce(t *testing.T) {
	root := t.TempDir()
	writeInitFixture(t, root, "note.md", "# Note\n![[first.png]]\n")
	writeInitFixture(t, root, "_assets/first.png", "image")
	runInit(t, root, "--image", "--preview")
	state := readInitState(t, root)
	if !state.Notes || len(state.Features) != 3 || state.Features["image"] != 1 || state.Features["graph"] != 1 || state.Features["preview"] != 1 {
		t.Fatalf("state: %+v", state)
	}
	if _, err := os.Stat(filepath.Join(root, ".hylo", "data.idx")); !os.IsNotExist(err) {
		t.Fatalf("unselected index was built: %v", err)
	}
	vault, err := storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.SetNotePinned("/note.md", true); err != nil {
		t.Fatal(err)
	}
	original, err := vault.StatNote("/note.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.Close(); err != nil {
		t.Fatal(err)
	}
	writeInitFixture(t, root, "note.md", "# Changed\n![[second.png]]\n")
	writeInitFixture(t, root, "_assets/second.png", "image")
	if output := runInit(t, root, "--image"); !strings.Contains(output, "skipping") {
		t.Fatal(output)
	}
	vault, err = storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	cover, err := vault.GetNoteAsset("/note.md", storage.AssetKindCover)
	if err != nil || cover.Filename != "first.png" {
		t.Fatalf("normal init must skip: %+v %v", cover, err)
	}
	vault.Close()
	runInit(t, root, "--image", "--force")
	vault, err = storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	cover, err = vault.GetNoteAsset("/note.md", storage.AssetKindCover)
	if err != nil || cover.Filename != "second.png" {
		t.Fatalf("forced cover: %+v %v", cover, err)
	}
	links, err := vault.ListNoteAssets("/note.md", storage.AssetKindImage)
	if err != nil || len(links) != 1 || links[0].Filename != "second.png" {
		t.Fatalf("forced image links: %+v %v", links, err)
	}
	note, err := vault.StatNote("/note.md")
	if err != nil || !note.Pinned || !note.CreatedAt.Equal(original.CreatedAt) || note.Preview != original.Preview {
		t.Fatalf("unrelated note data changed: %+v %v", note, err)
	}
	if state := readInitState(t, root); len(state.Features) != 3 {
		t.Fatalf("unselected features changed: %+v", state)
	}
}

func TestInitRecoversMissingOrInvalidState(t *testing.T) {
	for _, stateFile := range []string{"missing", "invalid"} {
		t.Run(stateFile, func(t *testing.T) {
			root := t.TempDir()
			writeInitFixture(t, root, "note.md", "# Note\n")
			vault, err := storage.New(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := vault.Close(); err != nil {
				t.Fatal(err)
			}
			if stateFile == "invalid" {
				if err := os.WriteFile(filepath.Join(root, ".hylo", "init.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			runInit(t, root, "--preview")
			vault, err = storage.New(root)
			if err != nil {
				t.Fatal(err)
			}
			defer vault.Close()
			if _, err := vault.StatNote("/note.md"); err != nil {
				t.Fatalf("note was not registered: %v", err)
			}
			if state := readInitState(t, root); !state.Notes || state.Features["preview"] != 1 {
				t.Fatalf("recovered state: %+v", state)
			}
		})
	}
}

func TestInitDefaultCompletesRemainingFeaturesAndForceAll(t *testing.T) {
	root := t.TempDir()
	writeInitFixture(t, root, "note.md", "# Note\n")
	runInit(t, root, "--preview")
	output := runInit(t, root)
	if !strings.Contains(output, "preview: already initialized") || !strings.Contains(output, "index: complete") {
		t.Fatal(output)
	}
	if state := readInitState(t, root); len(state.Features) != len(vaultInitFeatures) {
		t.Fatalf("incomplete: %+v", state)
	}
	output = runInit(t, root, "--force")
	if strings.Contains(output, "skipping") {
		t.Fatal(output)
	}
	for _, feature := range vaultInitFeatures {
		if !strings.Contains(output, feature.name+": complete") {
			t.Fatal(output)
		}
	}
}

func TestInitFailureDoesNotMarkFeatureCompleteAndRetryResumes(t *testing.T) {
	root := t.TempDir()
	writeInitFixture(t, root, "note.md", "# Note\n")
	vault, err := storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	cfg := config.MustLoadFor(root)
	cfg.Server.Proxy = "invalid"
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := initializeVault(context.Background(), vault, cfg, log, false, map[string]bool{"image": true}, false, io.Discard); err == nil {
		t.Fatal("expected invalid proxy error")
	}
	state := readInitState(t, root)
	if !state.Notes || len(state.Features) != 2 || state.Features["graph"] != 1 || state.Features["preview"] != 1 {
		t.Fatalf("failed feature marked done: %+v", state)
	}
	cfg.Server.Proxy = ""
	if err := initializeVault(context.Background(), vault, cfg, log, true, map[string]bool{"image": true}, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	if state := readInitState(t, root); state.Features["image"] != 1 {
		t.Fatalf("not resumed: %+v", state)
	}
}

func TestInitLegacyVaultPreservesMetadata(t *testing.T) {
	root := t.TempDir()
	vault, err := storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteNote("/note.md", []byte("# Note\n"), ""); err != nil {
		t.Fatal(err)
	}
	if err := vault.SetNotePinned("/note.md", true); err != nil {
		t.Fatal(err)
	}
	vault.Close()
	runInit(t, root, "--preview")
	vault, err = storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	note, err := vault.StatNote("/note.md")
	if err != nil || !note.Pinned {
		t.Fatalf("legacy metadata: %+v %v", note, err)
	}
}

func TestInitForceIndexDropsStaleDocuments(t *testing.T) {
	root := t.TempDir()
	writeInitFixture(t, root, "note.md", "# Note\n")
	runInit(t, root, "--index")
	indexer, err := search.NewBleveIndexer(root, false)
	if err != nil {
		t.Fatal(err)
	}
	indexer.Upsert("/ghost.md", "ghost.md", "stale document", nil, time.Now(), "")
	if err := indexer.Close(); err != nil {
		t.Fatal(err)
	}
	runInit(t, root, "--index")
	indexer, err = search.NewBleveIndexer(root, false)
	if err != nil {
		t.Fatal(err)
	}
	count, err := indexer.DocCount()
	if err != nil || count != 2 {
		t.Fatalf("normal init should skip: %d %v", count, err)
	}
	indexer.Close()
	runInit(t, root, "--index", "--force")
	indexer, err = search.NewBleveIndexer(root, false)
	if err != nil {
		t.Fatal(err)
	}
	defer indexer.Close()
	count, err = indexer.DocCount()
	if err != nil || count != 1 {
		t.Fatalf("forced index should discard stale documents: %d %v", count, err)
	}
}

func TestInitForceGraphReplacesEdges(t *testing.T) {
	root := t.TempDir()
	writeInitFixture(t, root, "_knowledge/first.md", "---\nkind: knowledge\nentity_type: concept\n---\n[[second]]\n")
	writeInitFixture(t, root, "_knowledge/second.md", "---\nkind: knowledge\nentity_type: concept\n---\nSecond\n")
	runInit(t, root, "--graph")
	vault, err := storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	edges, err := vault.GetAllKnowledgeLinks()
	if err != nil {
		t.Fatal(err)
	}
	linked := false
	for _, edge := range edges {
		if edge.Source == "/_knowledge/first.md" && edge.Target == "/_knowledge/second.md" {
			linked = true
		}
	}
	if !linked {
		t.Fatalf("initial link missing: %+v", edges)
	}
	vault.Close()
	writeInitFixture(t, root, "_knowledge/first.md", "---\nkind: knowledge\nentity_type: concept\n---\nNo links\n")
	runInit(t, root, "--graph", "--force")
	vault, err = storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	edges, err = vault.GetAllKnowledgeLinks()
	if err != nil {
		t.Fatal(err)
	}
	for _, edge := range edges {
		if edge.Source != edge.Target {
			t.Fatalf("stale edge: %+v", edge)
		}
	}
	types, err := vault.GetKnowledgeEntityTypes()
	if err != nil || len(types) != 2 {
		t.Fatalf("entity types lost: %v %v", types, err)
	}
}

func TestAutoInitCompletesOnlyMissingFeatures(t *testing.T) {
	root := t.TempDir()
	writeInitFixture(t, root, "note.md", "# Note\n")
	runInit(t, root, "--preview")
	vault, err := storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	cfg := config.MustLoadFor(root)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := autoInitVault(log, cfg, vault, true); err != nil {
		t.Fatal(err)
	}
	if state := readInitState(t, root); len(state.Features) != len(vaultInitFeatures) {
		t.Fatalf("automatic features: %+v", state)
	}
	// A completed image step must not run again on ordinary startup.
	cfg.Server.Proxy = "invalid"
	if err := autoInitVault(log, cfg, vault, true); err != nil {
		t.Fatal(err)
	}
}
