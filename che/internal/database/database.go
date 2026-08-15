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

type SpecDone struct {
	ID            uint   `gorm:"primaryKey"`
	RunID         string `gorm:"index"`
	CreatedAt     time.Time
	DefinitionURI string
	Command       string
}

type ProfileDone struct {
	ID            uint `gorm:"primaryKey"`
	SpecDoneID    uint `gorm:"index"`
	CreatedAt     time.Time
	Ref           string
	ProfileName   string
	DefinitionURI string
	DirectoryPath string
}

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
	ProfileRef    string `gorm:"->;column:profile_ref"`
}

type Object struct {
	Kind    string
	Present bool
	Target  string
	Mode    string
}

type Backup struct {
	ID         uint `gorm:"primaryKey"`
	SpecDoneID uint `gorm:"index"`
	CreatedAt  time.Time
	Path       string `gorm:"uniqueIndex"`
	Sub        string
	RunID      string `gorm:"->;column:run_id"`
}

// [<] 🤖🤖 models

type DB struct{ gorm *gorm.DB }

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

func (d *DB) RecordOperation(prof *ProfileDone, op OperationDone) error {
	if d == nil || prof == nil {
		return nil
	}
	op.ProfileDoneID = prof.ID
	return d.gorm.Create(&op).Error
}

func (d *DB) Installed() ([]OperationDone, error) {
	return d.latestOps("", true)
}

func (d *DB) InstalledForProfile(ref string) ([]OperationDone, error) {
	return d.latestOps(ref, true)
}

func (d *DB) LatestOps() ([]OperationDone, error) {
	return d.latestOps("", false)
}

func (d *DB) latestOps(ref string, excludeRemoved bool) ([]OperationDone, error) {
	if d == nil {
		return nil, nil
	}
	sub := d.gorm.Model(&OperationDone{}).
		Select("MAX(operation_dones.id) as id").
		Group("operation_dones.dest")
	if ref != "" {
		sub = sub.Joins("JOIN profile_dones ON profile_dones.id = operation_dones.profile_done_id").
			Where("profile_dones.ref = ?", ref)
	}
	q := d.gorm.Model(&OperationDone{}).
		Joins("JOIN profile_dones ON profile_dones.id = operation_dones.profile_done_id").
		Select("operation_dones.*, profile_dones.ref as profile_ref").
		Where("operation_dones.id IN (?)", sub).
		Order("operation_dones.id DESC")
	if excludeRemoved {
		q = q.Preload("Backup").Where("operation_dones.op_type <> ?", "remove")
	}
	var ops []OperationDone
	err := q.Find(&ops).Error
	return ops, err
}

func (d *DB) Backups() ([]Backup, error) {
	if d == nil {
		return nil, nil
	}
	var out []Backup
	err := d.gorm.Model(&Backup{}).
		Joins("JOIN spec_dones ON spec_dones.id = backups.spec_done_id").
		Select("backups.*, spec_dones.run_id as run_id").
		Order("backups.id DESC").
		Find(&out).Error
	return out, err
}

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
