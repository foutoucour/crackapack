package play_test

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

func sampleSnapshot() play.Snapshot {
	return play.Snapshot{
		Version: play.SaveVersion,
		Opener:  &play.OpenerSnapshot{Seed: 42, State: []byte("pcg:0123456789abcdef")},
		Welcome: map[string]int{"RAV": 3, "TSP": 0},
		Collection: []play.CardCount{
			{ID: "rav-1", Finish: domain.FinishNonFoil, Quantity: 2},
			{ID: "rav-1", Finish: domain.FinishFoil, Quantity: 1},
		},
		Energy: &play.EnergySnapshot{
			Reserve: 4, Savings: 2, SavingsTick: 1,
			SyncedAt: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC),
		},
		LastOpening: &play.OpeningSnapshot{
			SetCode: "RAV",
			Cards:   []play.OpenedCard{{ID: "rav-1", Finish: domain.FinishFoil, New: true}},
		},
	}
}

func TestFileStore_loadsWhatWasSaved(t *testing.T) {
	t.Parallel()
	store := play.NewFileStore(filepath.Join(t.TempDir(), "save.json"))
	want := sampleSnapshot()

	if err := store.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, found, err := store.Load()

	if err != nil || !found {
		t.Fatalf("Load = found %v, err %v, want a snapshot", found, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("loaded snapshot = %+v, want %+v", got, want)
	}
}

func TestFileStore_aMissingFileIsANewGame(t *testing.T) {
	t.Parallel()
	store := play.NewFileStore(filepath.Join(t.TempDir(), "save.json"))

	_, found, err := store.Load()

	if err != nil || found {
		t.Errorf("Load of a missing file = found %v, err %v, want not found and no error", found, err)
	}
}

func TestFileStore_aFileItCannotReadIsAnErrorNamingThePathAndIsLeftAlone(t *testing.T) {
	t.Parallel()
	valid := `{"version": 1, "welcome": {}, "collection": []}`
	tests := []struct {
		name    string
		content string
	}{
		{"not json", "this is not a save"},
		{"empty", ""},
		{"truncated", valid[:len(valid)/2]},
		{"unknown field", `{"version": 1, "coins": 5}`},
		{"trailing data", valid + valid},
		{"no version", `{"welcome": {}}`},
		{"a version from the future", `{"version": 99}`},
		{"a number where a section goes", `{"version": 1, "welcome": 3}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "save.json")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			_, found, err := play.NewFileStore(path).Load()

			if err == nil || found {
				t.Fatalf("Load = found %v, err %v, want an error", found, err)
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("error %q does not name the path %q", err, path)
			}
			if got, _ := os.ReadFile(path); string(got) != tc.content {
				t.Errorf("the file now holds %q, want it left as it was", got)
			}
		})
	}
}

func TestFileStore_saveCreatesAPrivateFileInAMissingDirectory(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "crackapack")
	path := filepath.Join(dir, "save.json")

	if err := play.NewFileStore(path).Save(sampleSnapshot()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat the save: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("save mode = %v, want 0600", info.Mode().Perm())
	}
	assertOnlyFile(t, dir, "save.json")
}

func TestFileStore_aFailedSaveKeepsTheOldFile(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a directory cannot be made unwritable for this user")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "save.json")
	store := play.NewFileStore(path)
	if err := store.Save(sampleSnapshot()); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the save: %v", err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	changed := sampleSnapshot()
	changed.Welcome = map[string]int{"RAV": 0}

	if err := store.Save(changed); err == nil {
		t.Fatal("Save in an unwritable directory returned nil error")
	}

	if after, err := os.ReadFile(path); err != nil || string(after) != string(before) {
		t.Errorf("the save changed after a failed Save: err %v", err)
	}
	assertOnlyFile(t, dir, "save.json")
}

func TestFileStore_aFailedRenameLeavesNoTemporaryFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// A directory in the way of the final name makes the rename, which comes last, fail.
	if err := os.Mkdir(filepath.Join(dir, "save.json"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := play.NewFileStore(filepath.Join(dir, "save.json")).Save(sampleSnapshot()); err == nil {
		t.Fatal("Save over a directory returned nil error")
	}

	assertOnlyFile(t, dir, "save.json")
}

func assertOnlyFile(t *testing.T, dir, name string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		t.Errorf("directory holds %v, want only %q", entries, name)
	}
}

func FuzzFileStoreLoad(f *testing.F) {
	valid, err := os.ReadFile("testdata/save-v1.json")
	if err != nil {
		f.Fatalf("read the sample save: %v", err)
	}
	f.Add(valid)
	f.Add([]byte(`{"version": 1}`))
	f.Add([]byte(`{"version": 1, "collection": [{"id": "a", "finish": "foil", "quantity": -1}]}`))
	f.Add([]byte(`[`))
	path := filepath.Join(f.TempDir(), "save.json")
	f.Fuzz(func(t *testing.T, data []byte) {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		// Whatever the file holds, loading it returns a snapshot or an error, and never panics.
		_, _, _ = play.NewFileStore(path).Load()
	})
}
