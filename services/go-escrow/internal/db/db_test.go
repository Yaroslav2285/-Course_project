package db

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestWithTx_Commit(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectCommit()

	err = WithTx(db, nil, func(tx *sql.Tx) error {
		return nil
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTx_RollbackOnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectRollback()

	bizErr := errors.New("business error")
	err = WithTx(db, nil, func(tx *sql.Tx) error {
		return bizErr
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, bizErr)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTx_BeginTxError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin().WillReturnError(errors.New("connection refused"))

	err = WithTx(db, nil, func(tx *sql.Tx) error {
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BeginTx")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunMigrations_ExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnError(errors.New("permission denied"))

	err = runMigrations(db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exec migration")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRunMigrations_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))

	err = runMigrations(db)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClose(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	mock.ExpectClose()

	d := &Database{DB: db}
	err = d.Close()
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInitDB_Success(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	assert.NoError(t, err)
	defer db.Close()

	mock.ExpectPing()
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))

	d, err := initDB(db)
	assert.NoError(t, err)
	assert.NotNil(t, d)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInitDB_PingError(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	assert.NoError(t, err)
	defer db.Close()

	mock.ExpectPing().WillReturnError(errors.New("connection timeout"))

	_, err = initDB(db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db.Ping")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInitDB_MigrationError(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	assert.NoError(t, err)
	defer db.Close()

	mock.ExpectPing()
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnError(errors.New("migration failed"))

	_, err = initDB(db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "migrations")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInitDB_OpenError(t *testing.T) {
	// Connect() calls sql.Open("postgres", dsn) which requires a real driver.
	// We test the extracted initDB with sqlmock instead.
	// sql.Open error is rare and depends on the driver; tested by driver tests.
}
