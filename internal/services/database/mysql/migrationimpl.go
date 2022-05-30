package mysql

import (
	"database/sql"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/embedded"
)

type migrationFunc func(*sql.Tx) error

type migration struct {
	Version       int
	Applied       time.Time
	ReleaseTag    string
	ReleaseCommit string
}

func (m *MysqlMiddleware) Migrate() (err error) {
	mig, err := m.getLatestMigration()
	if err == sql.ErrNoRows {
		mig = &migration{
			Version: -1,
		}
	} else if err != nil {
		return
	}

	tx, err := m.Db.Begin()
	if err != nil {
		return
	}
	for i := mig.Version + 1; i < len(migrationFuncs); i++ {
		logrus.WithField("version", i).Info("DATABASE :: applying migration")
		if err = migrationFuncs[i](tx); err != nil {
			return
		}
		if err = putMigrationVersion(tx, i); err != nil {
			return
		}
	}
	err = tx.Commit()

	return
}

func (m *MysqlMiddleware) getLatestMigration() (mig *migration, err error) {
	mig = new(migration)
	row := m.Db.QueryRow(
		`SELECT version, applied, releaseTag, releaseCommit
		FROM migrations
		ORDER BY version DESC
		LIMIT 1`)
	err = row.Scan(&mig.Version, &mig.Applied, &mig.ReleaseTag, &mig.ReleaseCommit)
	return
}

func putMigrationVersion(tx *sql.Tx, i int) (err error) {
	_, err = tx.Exec(
		`INSERT INTO migrations (version, applied, releaseTag, releaseCommit)
		VALUES (?, ?, ?, ?)`,
		i, time.Now(), embedded.AppVersion, embedded.AppCommit)
	return
}

// --- UTILITIES ---

func createTableColumnIfNotExists(m *sql.Tx, table, definition string) (err error) {
	_, err = m.Exec(
		"ALTER TABLE `" + table +
			"` ADD COLUMN " + definition)

	if e, ok := err.(*mysql.MySQLError); ok && e.Number == 1060 {
		err = nil
	}

	return
}
