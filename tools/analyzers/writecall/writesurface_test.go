package writecall

import (
	"go/types"
	"slices"
	"testing"

	"golang.org/x/tools/go/packages"
)

// storePkgPath is the package this analyzer exists to guard.
const storePkgPath = "github.com/glw907/poplar/internal/store"

// writeSurface names every internal/store export that carries a
// write: the writer, the calls that yield or migrate a write
// connection, and the transaction mutators the sync engine runs
// inside a writer transaction.
var writeSurface = []string{
	"CheckIntegrity",
	"CurrentSchemaVersion",
	"DeleteMailbox",
	"DeleteMailboxByID",
	"DeleteMessage",
	"DeleteMessageByID",
	"Migrate",
	"NewWriter",
	"Open",
	"OpenWriteConn",
	"RebuildIndex",
	"Recover",
	"RepairMailboxAssociations",
	"StaleMailboxIDs",
	"StaleMessageIDs",
	"SyncMessageMailboxes",
	"UpsertMailbox",
	"UpsertMessage",
	"Writer",
}

// readSurface names every internal/store export reviewed as
// carrying no write: the read pool and the shapes it returns, the
// pure flag encodings, the values a caller hands the writer, and
// the two startup calls that read or write the clean-shutdown
// marker beside the database rather than the database itself.
var readSurface = []string{
	"DecodeFlags",
	"DefaultReadPoolSize",
	"DefaultWriterConfig",
	"EncodeFlags",
	"FlagAnswered",
	"FlagDraft",
	"FlagFlagged",
	"FlagForwarded",
	"FlagSeen",
	"Flags",
	"MailboxCursor",
	"MailboxDetails",
	"MailboxPage",
	"MailboxRow",
	"MailboxUpsert",
	"MarkCleanShutdown",
	"MessageSummary",
	"MessageUpsert",
	"ShouldRunIntegrityCheck",
	"NewReadPool",
	"ReadPool",
	"RecoveredCounts",
	"Revision",
	"RevisionCounter",
	"WriterConfig",
}

// TestWriteSurfaceIsClassified holds the analyzer against the real
// internal/store rather than against testdata: every export is
// classified here, and the classifier has to agree with the
// classification. An export in neither list fails the test, so the
// analyzer cannot quietly stop covering the store as it grows.
func TestWriteSurfaceIsClassified(t *testing.T) {
	for _, name := range writeSurface {
		if slices.Contains(readSurface, name) {
			t.Errorf("%s is listed as both write and read surface", name)
		}
	}

	scope := loadStore(t).Scope()
	var seen []string
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		seen = append(seen, name)
		write := slices.Contains(writeSurface, name)
		if !write && !slices.Contains(readSurface, name) {
			t.Errorf("internal/store exports %s, which neither writeSurface nor readSurface names: classify it", name)
			continue
		}
		if got := writeEntryPoint(obj); got != write {
			t.Errorf("writeEntryPoint(%s) = %v, want %v", name, got, write)
		}
		checkMethods(t, obj, write)
	}

	for _, name := range slices.Concat(writeSurface, readSurface) {
		if !slices.Contains(seen, name) {
			t.Errorf("%s is classified here but internal/store no longer exports it", name)
		}
	}
}

// checkMethods asserts the exported methods of a store type carry
// the same classification as the type: every method on the writer
// is a write entry point, and no method on a read type is.
func checkMethods(t *testing.T, obj types.Object, write bool) {
	t.Helper()
	named, ok := obj.Type().(*types.Named)
	if !ok {
		return
	}
	for i := range named.NumMethods() {
		m := named.Method(i)
		if !m.Exported() {
			continue
		}
		if got := writeEntryPoint(m); got != write {
			t.Errorf("writeEntryPoint(%s.%s) = %v, want %v", obj.Name(), m.Name(), got, write)
		}
	}
}

// loadStore type-checks the real internal/store from the poplar
// module three directories up, the module this tools module gates.
func loadStore(t *testing.T) *types.Package {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedImports | packages.NeedDeps,
		Dir:  "../../..",
	}
	pkgs, err := packages.Load(cfg, storePkgPath)
	if err != nil {
		t.Fatalf("load %s: %v", storePkgPath, err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("load %s: got %d packages, want 1", storePkgPath, len(pkgs))
	}
	if errs := pkgs[0].Errors; len(errs) > 0 {
		t.Fatalf("load %s: %v", storePkgPath, errs)
	}
	return pkgs[0].Types
}
