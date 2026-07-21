package che

// [>] 🤖🤖

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"gitlab.com/konradodwrot/go-modules/che/internal/database"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
)

// restoreFromBackup writes op's pre-mutation backup entry back onto the dest
// through FS (sudo/dry-run honored). Returns false when the op has no backup or
// the archive holds no entry for the dest (it pre-existed absent). Shared by
// Uninstaller and Restorer.
func (p *ProfileReady) restoreFromBackup(op database.OperationDone) (bool, error) {
	if op.Backup == nil {
		return false, nil
	}
	body, mode, found, err := fsutil.ReadFromArchive(op.Backup.Path, op.Dest)
	if err != nil || !found {
		return false, err
	}
	if err := p.FS.InstallFile(op.Dest, body, mode, ""); err != nil {
		return false, err
	}
	return true, nil
}

// driftedFromNext reports whether the live dest no longer matches the op's
// recorded Next state (kind/target/mode), the "look before you overwrite"
// guardrail. An absent dest that the op left present is drift too.
func driftedFromNext(live, next database.Object) bool {
	return live.Kind != next.Kind || live.Target != next.Target || live.Mode != next.Mode
}

// ListBackups prints the ledger's backup points, newest first, under a
// `# backups` heading: each line carries the run id, backup id, timestamp,
// archive size, and abbreviated path. No backup points prints nothing.
func ListBackups(ctx Context) error {
	home, err := resolveInvokingHome(ctx)
	if err != nil {
		return err
	}
	seams := NewSeams(home)
	defer func() { _ = seams.Ledger.Close() }()
	backups, err := seams.Ledger.Backups()
	if err != nil {
		return err
	}
	if len(backups) == 0 {
		return nil
	}
	log.EmitHeading(log.Levels.Info, 1, "backup-ls", "listing", "backups")
	for _, b := range backups {
		ts, backupID := fsutil.ParseBackupArchiveName(b.Path)
		line := fmt.Sprintf("run %s, backup %s, %s, %s, %s",
			b.RunID, backupID, ts, humanSize(archiveSize(b.Path)), abbreviateHome(b.Path, home))
		log.Emit(log.Event{
			Level: log.Levels.Info, Scope: "backup-ls", Msg: line, Depth: 1,
			Attrs: map[string]string{"runId": b.RunID, "backupId": backupID, "path": b.Path},
		})
	}
	return nil
}

// RestoreSelector picks the backup points a restore applies: exactly one field
// set (the mutually-exclusive --run-id | --backup-id | --timestamp flags).
type RestoreSelector struct {
	RunID     string // restore every archive of that run
	BackupID  string // restore the single archive carrying that backup id
	Timestamp string // PITR: per dest, the newest backup at or before this TsLayout stamp
}

// Restorer restores host state from recorded backup archives, ledger-driven
// like Uninstaller: it reuses ProfileReady as the execution surface so its
// restore ops record in the same normalized shape.
type Restorer struct {
	p      *ProfileReady
	dryRun bool
}

// NewRestorer builds a Restorer: resolve the invoking home, open the seams,
// and open this restore run's own SpecDone + ProfileDone so its ops record.
func NewRestorer(ctx Context, opts options.Options) (*Restorer, error) {
	home, err := resolveInvokingHome(ctx)
	if err != nil {
		return nil, err
	}
	seams := NewSeams(home)
	spec, err := seams.Ledger.StartSpec(ctx.RunID, "", "backup-restore")
	if err != nil {
		log.EmitTrace("ledger", "error", "restore start spec: "+err.Error())
	}
	prof, err := seams.Ledger.StartProfile(spec, "restore", "restore", "", home)
	if err != nil {
		log.EmitTrace("ledger", "error", "restore start profile: "+err.Error())
	}
	p := &ProfileReady{
		ref:         "restore",
		home:        home,
		opts:        opts,
		runID:       ctx.RunID,
		runTs:       ctx.RunTs,
		specDone:    spec,
		profileDone: prof,
		logDepth:    1, // [why] restores nest under their per-profile `## profile` heading
		Seams:       seams,
	}
	return &Restorer{p: p, dryRun: opts.DryRun != options.DryRun.Off}, nil
}

// restoreUnit is one dest to restore, paired with the archive holding its body.
type restoreUnit struct {
	dest    string
	archive database.Backup
}

// Restore applies the selector's backup points: each selected archive's entries
// restore back onto their dests, newest archive winning per dest, grouped by
// owning profile (`## profile <ref>` headings). A live dest that drifted from
// che's last recorded state skips (debug), never clobbered. A failing dest does
// not stop the rest: failures collect, report, and join. Dry run predicts
// `restore <dest> (dry run)`, writing nothing.
func (r *Restorer) Restore(sel RestoreSelector) error {
	p := r.p
	units, err := r.selectUnits(sel)
	if err != nil {
		return err
	}
	latest, err := r.latestByDest()
	if err != nil {
		return err
	}
	// [why] cluster the output's `## profile` headings: one per owning profile.
	slices.SortStableFunc(units, func(a, b restoreUnit) int {
		return strings.Compare(latest[a.dest].ProfileRef, latest[b.dest].ProfileRef)
	})
	var errs []error
	lastRef := ""
	for _, u := range units {
		if ref := latest[u.dest].ProfileRef; ref != "" && ref != lastRef {
			log.EmitHeading(log.Levels.Info, 2, "restore", "restoring", "profile "+ref)
			lastRef = ref
		}
		if err := r.restoreUnit(u, latest); err != nil {
			errs = append(errs, p.failItem("restore", u.dest, err))
		}
	}
	for _, err := range errs {
		log.EmitError("restore", "fail", err.Error())
	}
	return errors.Join(errs...)
}

// selectUnits resolves the selector against the ledger's archives, then lists
// each matching archive's entries: one unit per dest, the newest matching
// archive winning. No selector, an unreadable archive, or no matching entry is
// a clear error.
func (r *Restorer) selectUnits(sel RestoreSelector) ([]restoreUnit, error) {
	backups, err := r.p.Ledger.Backups()
	if err != nil {
		return nil, err
	}
	match, what, err := matchBackup(sel)
	if err != nil {
		return nil, err
	}
	// [why] the newest archive at or before the selection wins per dest:
	// order by archive ts (ledger recency breaks same-second ties).
	slices.SortStableFunc(backups, func(a, b database.Backup) int {
		return strings.Compare(archiveTs(b.Path), archiveTs(a.Path))
	})
	seen := map[string]bool{}
	var units []restoreUnit
	for _, b := range backups {
		if !match(b) {
			continue
		}
		dests, err := fsutil.ListArchiveEntries(b.Path)
		if err != nil {
			return nil, fmt.Errorf("read archive %s: %w", b.Path, err)
		}
		for _, dest := range dests {
			if seen[dest] {
				continue
			}
			seen[dest] = true
			units = append(units, restoreUnit{dest: dest, archive: b})
		}
	}
	if len(units) == 0 {
		return nil, fmt.Errorf("no backup matches %s", what)
	}
	return units, nil
}

// matchBackup maps the selector onto an archive predicate plus its description
// (error text). Exactly one selector field must be set.
func matchBackup(sel RestoreSelector) (func(database.Backup) bool, string, error) {
	switch {
	case sel.RunID != "":
		return func(b database.Backup) bool { return b.RunID == sel.RunID },
			fmt.Sprintf("run id %q", sel.RunID), nil
	case sel.BackupID != "":
		return func(b database.Backup) bool {
			_, id := fsutil.ParseBackupArchiveName(b.Path)
			return id == sel.BackupID
		}, fmt.Sprintf("backup id %q", sel.BackupID), nil
	case sel.Timestamp != "":
		limit, err := time.ParseInLocation(fsutil.TsLayout, sel.Timestamp, time.Local)
		if err != nil {
			return nil, "", fmt.Errorf("invalid --timestamp %q: want the %s layout", sel.Timestamp, fsutil.TsLayout)
		}
		return func(b database.Backup) bool {
			at, err := time.ParseInLocation(fsutil.TsLayout, archiveTs(b.Path), time.Local)
			return err == nil && !at.After(limit)
		}, fmt.Sprintf("timestamp %q", sel.Timestamp), nil
	default:
		return nil, "", fmt.Errorf("pass one of --run-id, --backup-id, --timestamp")
	}
}

// archiveTs is the archive filename's TsLayout stamp.
func archiveTs(path string) string {
	ts, _ := fsutil.ParseBackupArchiveName(path)
	return ts
}

// latestByDest maps every recorded dest to its last recorded op (removes
// included): the restore drift guard's baseline plus the profile grouping.
func (r *Restorer) latestByDest() (map[string]database.OperationDone, error) {
	ops, err := r.p.Ledger.LatestOps()
	if err != nil {
		return nil, err
	}
	out := make(map[string]database.OperationDone, len(ops))
	for _, op := range ops {
		out[op.Dest] = op
	}
	return out, nil
}

// restoreUnit restores one dest from its selected archive: skip when the live
// dest drifted from che's last recorded state (don't clobber user changes),
// else write the archived body back and record the restore op (kind carried
// over from the dest's last record; an unrecorded dest restores unrecorded).
func (r *Restorer) restoreUnit(u restoreUnit, latest map[string]database.OperationDone) error {
	p := r.p
	live := p.classifyDest(u.dest)
	last, known := latest[u.dest]
	if known && driftedFromNext(live, last.Next) {
		p.emitSkip(log.Levels.Debug, "restore", "restore", u.dest, "dest drifted from the recorded state")
		return nil
	}
	if r.dryRun {
		p.emitDryRun("restore", "restore", u.dest)
		return nil
	}
	body, mode, found, err := fsutil.ReadFromArchive(u.archive.Path, u.dest)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if err := p.FS.InstallFile(u.dest, body, mode, ""); err != nil {
		return err
	}
	p.emit(log.Levels.Info, "restore", "restored", u.dest)
	if known {
		p.currentArchive, p.currentSub = u.archive.Path, u.archive.Sub
		p.recordOperation(u.dest, opInfo{kind: last.Kind}, live)
	}
	return nil
}

// Close releases the restorer's ledger handle.
func (r *Restorer) Close() error { return r.p.Ledger.Close() }

// [<] 🤖🤖
