// Package database persists every OS-mutating operation che performs into a
// GORM-backed SQLite store (pure-Go glebarez driver, no CGO), so che can answer
// "what is installed onto this host" and back it out (che uninstall) or prune
// stale dests it no longer produces.
package database

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// [>] 🤖🤖 models

// SpecDone is one recorded che invocation (the root resolved spec of a run).
type SpecDone struct {
	ID            uint   `gorm:"primaryKey"`
	RunID         string `gorm:"index"` // TsLayout stamp; second-precision, not unique (same-second runs share it)
	CreatedAt     time.Time
	DefinitionURI string
	Command       string
}

// ProfileDone is one profile resolved+run within a SpecDone (FK -> SpecDone).
type ProfileDone struct {
	ID            uint `gorm:"primaryKey"`
	SpecDoneID    uint `gorm:"index"`
	CreatedAt     time.Time
	Ref           string
	ProfileName   string
	DefinitionURI string
	DirectoryPath string
}

// OperationDone is the smallest unit of installation: one dest mutation. Kind
// discriminates the op; kind-specific data rides in the nullable Target/SrcRel/
// Mode/Owner columns (only the ones that kind uses are set).
type OperationDone struct {
	ID            uint `gorm:"primaryKey"`
	ProfileDoneID uint `gorm:"index"`
	CreatedAt     time.Time
	OpType        string `gorm:"index"`
	Kind          string `gorm:"index"`
	Dest          string `gorm:"index"`
	Target        string
	SrcRel        string
	Mode          string
	Owner         string
	Prev          Object `gorm:"embedded;embeddedPrefix:prev_"`
	Next          Object `gorm:"embedded;embeddedPrefix:next_"`
	BackupID      *uint  `gorm:"index"`
	Backup        *Backup
}

// Object is a dest's classified state, embedded twice (prev/next) on OperationDone.
type Object struct {
	Kind    string
	Present bool
	Target  string
	Mode    string
}

// Backup is one per-run archive, FK'd to the SpecDone run it belongs to; many
// OperationDone reference it. Deduped by Path.
type Backup struct {
	ID         uint `gorm:"primaryKey"`
	SpecDoneID uint `gorm:"index"`
	CreatedAt  time.Time
	Path       string `gorm:"uniqueIndex"`
	Sub        string
}

// [<] 🤖🤖 models

// DB is the ledger's GORM handle.
type DB struct{ gorm *gorm.DB }

// Open opens (creating if absent) the ledger DB at path over the pure-Go
// glebarez sqlite driver, silences GORM's logger, and AutoMigrates the models.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	g, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	if err := g.AutoMigrate(&SpecDone{}, &ProfileDone{}, &OperationDone{}, &Backup{}); err != nil {
		return nil, err
	}
	return &DB{gorm: g}, nil
}

// StartSpec records one che invocation (nil DB -> nil row, records nothing).
func (d *DB) StartSpec(runID, uri, command string) (*SpecDone, error) {
	if d == nil {
		return nil, nil
	}
	s := &SpecDone{RunID: runID, DefinitionURI: uri, Command: command}
	if err := d.gorm.Create(s).Error; err != nil {
		return nil, err
	}
	return s, nil
}

// StartProfile records one profile run under spec (nil DB/spec -> nil row).
func (d *DB) StartProfile(spec *SpecDone, ref, name, uri, dir string) (*ProfileDone, error) {
	if d == nil || spec == nil {
		return nil, nil
	}
	p := &ProfileDone{SpecDoneID: spec.ID, Ref: ref, ProfileName: name, DefinitionURI: uri, DirectoryPath: dir}
	if err := d.gorm.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

// EnsureBackup find-or-creates the Backup row for archive path under spec's run
// (dedup: one row per archive). nil DB/spec/empty path -> nil row.
func (d *DB) EnsureBackup(spec *SpecDone, path, sub string) (*Backup, error) {
	if d == nil || spec == nil || path == "" {
		return nil, nil
	}
	b := &Backup{SpecDoneID: spec.ID, Path: path, Sub: sub}
	if err := d.gorm.Where(Backup{Path: path}).FirstOrCreate(b).Error; err != nil {
		return nil, err
	}
	return b, nil
}

// RecordOperation sets the profile FK and writes op (nil DB/prof -> no-op).
func (d *DB) RecordOperation(prof *ProfileDone, op OperationDone) error {
	if d == nil || prof == nil {
		return nil
	}
	op.ProfileDoneID = prof.ID
	return d.gorm.Create(&op).Error
}

// Installed returns the latest OperationDone per dest whose op_type != remove
// (the current installed set), newest-first, Backup preloaded. nil DB -> nil.
func (d *DB) Installed() ([]OperationDone, error) {
	return d.installedWhere("")
}

// InstalledForProfile is Installed scoped to one profile ref (per-profile prune
// stale set).
func (d *DB) InstalledForProfile(ref string) ([]OperationDone, error) {
	return d.installedWhere(ref)
}

// installedWhere returns the newest non-remove OperationDone per dest, optionally
// filtered to one profile ref via the ProfileDone join.
func (d *DB) installedWhere(ref string) ([]OperationDone, error) {
	if d == nil {
		return nil, nil
	}
	sub := d.gorm.Model(&OperationDone{}).
		Joins("JOIN profile_dones ON profile_dones.id = operation_dones.profile_done_id").
		Select("MAX(operation_dones.id) as id").
		Group("operation_dones.dest")
	if ref != "" {
		sub = sub.Where("profile_dones.ref = ?", ref)
	}
	var ops []OperationDone
	err := d.gorm.Preload("Backup").
		Where("id IN (?)", sub).
		Where("op_type <> ?", "remove").
		Order("id DESC").
		Find(&ops).Error
	return ops, err
}

// Close closes the underlying sql.DB (nil DB -> no-op).
func (d *DB) Close() error {
	if d == nil {
		return nil
	}
	sqlDB, err := d.gorm.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// [<] 🤖🤖
