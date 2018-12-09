//+build integration

package dblocker_test

import (
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	locker "github.com/ivan-kostko/DbLocker"

	"github.com/stretchr/testify/assert"
)

const (
	postgresDriverName = "postgres"
)

// Integration tests settings
var (
	// Connection string to postgres db where lock will be acquired
	psqlUri string

	// Sleep duretion in nanoseconds in order to let concurrent processes to
	// begin a new transaction(s) and start executing sql statement
	sleepDuration = 200000000

	// Number of seconds to wait for postges to come up
	WaitForPostgresSec = 10
)

func init() {
	psqlUri = os.Getenv("POSTGRES_URI")
}

func Test_ConcurrentlyObtainingLocks(t *testing.T) {

	logrus.SetLevel(logrus.TraceLevel)

	testCases := []struct {
		Alias                     string
		OriginalLockId            string
		CocncurrentLockIds        []string
		ObtainedConcurrentlyLocks []string
		ObtainedAfterReleaseLocks []string
	}{
		{
			Alias:              `No concurrent lockers`,
			OriginalLockId:     "Blah",
			CocncurrentLockIds: []string{},
		},
		{
			Alias:                     `Same concurrent locker`,
			OriginalLockId:            "Blah",
			CocncurrentLockIds:        []string{"Blah"},
			ObtainedConcurrentlyLocks: []string{},
			ObtainedAfterReleaseLocks: []string{"Blah"},
		},
		{
			Alias:                     `5 the same concurrent lockers`,
			OriginalLockId:            "Blah",
			CocncurrentLockIds:        []string{"Blah", "Blah", "Blah", "Blah", "Blah"},
			ObtainedConcurrentlyLocks: []string{},
			ObtainedAfterReleaseLocks: []string{"Blah", "Blah", "Blah", "Blah", "Blah"},
		},
		{
			Alias:                     `5 different concurrent lockers`,
			OriginalLockId:            "Blah",
			CocncurrentLockIds:        []string{"Blah1", "Blah2", "Blah3", "Blah4", "Blah5"},
			ObtainedConcurrentlyLocks: []string{"Blah1", "Blah2", "Blah3", "Blah4", "Blah5"},
			ObtainedAfterReleaseLocks: []string{"Blah1", "Blah2", "Blah3", "Blah4", "Blah5"},
		},
	}

	// General test settings
	db, err := sql.Open(postgresDriverName, psqlUri)
	if err != nil {
		t.Fatalf("Failed to connect to database due to error %s", err.Error())
	}

	defer db.Close()

	// Wait while postgres is coming up
	for i := 0; i < WaitForPostgresSec; i++ {
		if err := db.Ping(); err != nil {
			t.Logf("Server %s is not yet responding...", psqlUri)
			time.Sleep(1000000000)
		}
	}

	for _, tCase := range testCases {

		testFn := func(t *testing.T) {

			// Have
			lockFactory := locker.NewLocker(db)
			lock, err := lockFactory(tCase.OriginalLockId)
			if err != nil {
				t.Fatalf("Failed to obtain lock due to error %s", err.Error())
			}

			// Slice to report concurrently obtained locks.
			obtainedLocksReport := []string{}
			obtainedLocksChanel := make(chan string, len(tCase.CocncurrentLockIds))

			// When 1

			wg := sync.WaitGroup{}

			// Start obtaining locks concurrently
			for i, lockIdN := range tCase.CocncurrentLockIds {

				wg.Add(1)

				readyToRequestLock := make(chan struct{})
				// Start obtaining lock concurrently
				go func(i int, lockIdN string) {

					defer wg.Done()

					lockFactoryN := locker.NewLocker(db)

					// Report that we are ready to request a lock
					close(readyToRequestLock)
					lockN, _ := lockFactoryN(lockIdN)

					// Report obtained lock
					obtainedLocksChanel <- lockIdN

					// Release lock on exit
					if err := lockN.UnLock(); err != nil {
						t.Fatal(err)
					}

				}(i, lockIdN)

				// Wait while it is ready to request lock
				<-readyToRequestLock
			}

			// Then

			// Give it a bit time to open transaction and execute statement
			time.Sleep(time.Duration(sleepDuration))

			for len(obtainedLocksChanel) > 0 {
				x := <-obtainedLocksChanel
				obtainedLocksReport = append(obtainedLocksReport, x)
			}
			assert.ElementsMatch(t, obtainedLocksReport, tCase.ObtainedConcurrentlyLocks, "Concurrently obtained locks are not expected")

			// When 2

			lock.UnLock()
			wg.Wait()

			// Then
			for len(obtainedLocksChanel) > 0 {
				x := <-obtainedLocksChanel
				obtainedLocksReport = append(obtainedLocksReport, x)
			}
			assert.ElementsMatch(t, obtainedLocksReport, tCase.ObtainedAfterReleaseLocks, "Obtained locks after releasing original are not expected")

		}

		t.Run(tCase.Alias, testFn)
	}

}
