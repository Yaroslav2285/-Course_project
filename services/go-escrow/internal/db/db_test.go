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
