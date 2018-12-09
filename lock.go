package dblocker

import (
	"database/sql"
	"fmt"

	"github.com/sirupsen/logrus"

	_ "github.com/lib/pq"
	"github.com/pkg/errors"
)

const (
	slqSatementTemplate = "CREATE TABLE %s();"
)

// Begins new sql transactions
type TxBeginner interface {
	Begin() (*sql.Tx, error)
}

// Represents obtained lock
type UnLocker func() error

// Make it interfaceable. Not yet needed but really helpful for dependecy injection.
func (fn UnLocker) UnLock() error {
	return fn()
}

// Represents functions to obtain shared lock for provided `lockId`
type Locker func(lockId string) (UnLocker, error)

// Make it interfaceable. Not yet needed but really helpful for dependecy injection.
func (fn Locker) Lock(lockId string) (UnLocker, error) {
	return fn(lockId)
}

// Creates a new Locker based on provided `txBeginner`.
func NewLocker(txBeginner TxBeginner) Locker {

	return func(lockId string) (UnLocker, error) {

		log := logrus.WithField("lockId", lockId)
		log.Debug("Beginning database transaction")
		tx, err := txBeginner.Begin()
		if err != nil {
			e := errors.Wrap(err, "failed to begin database transaction")
			log.Error(e.Error())
			return nil, e
		}

		log.Debug("Executing sql statement")
		stmt := fmt.Sprintf(slqSatementTemplate, lockId)
		log.Tracef("Statement: %s", stmt)
		if _, err = tx.Exec(stmt); err != nil {
			e := errors.Wrap(err, "failed to begin database transaction")
			log.Error(e.Error())
			return nil, e
		}

		log.Info("Lock obtained")

		return func() error {
			log.Debug("Rolling back transaction")
			if err := tx.Rollback(); err != nil {
				e := errors.Wrap(err, "failed to rollback database transaction")
				log.Error(e.Error())
				return e
			}
			log.Info("Lock released")
			return nil
		}, nil
	}
}
