package storage

import (
	"github.com/pashagolub/pgxmock/v4"
	"testing"
)

const UserIDInt uint64 = 123123123123123123

func TestInsertUser(t *testing.T) {
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	mock.ExpectExec("^INSERT INTO users VALUES ((.+), true, NULL)(.+)$").
		WithArgs(UserIDInt).WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = insertUser(mock, UserIDInt)
	if err != nil {
		t.Error(err)
	}

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestGetUser(t *testing.T) {
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	// make sure an empty response (no user) returns an error
	mock.ExpectQuery("^SELECT (.+) FROM users WHERE user_id = (.+)$").
		WithArgs(UserIDInt).
		WillReturnRows(
			pgxmock.NewRows([]string{"user_id", "opt"}))

	user, err := getUser(mock, UserIDInt)
	if err == nil {
		t.Error("error should not be nil when no users are returned")
	}
	if user != nil {
		t.Error("user should be nil")
	}

	// make sure a populated response doesn't return an error
	mock.ExpectQuery("^SELECT (.+) FROM users WHERE user_id = (.+)$").
		WithArgs(UserIDInt).
		WillReturnRows(
			pgxmock.NewRows([]string{"user_id", "opt"}).
				AddRow(UserIDInt, true))

	user, err = getUser(mock, UserIDInt)
	if err != nil {
		t.Error(err)
	}
	if user == nil {
		t.Error("expected user to not be nil")
	}
	if user.UserID != UserIDInt || !user.Opt {
		t.Error("userID or opt mismatches what was returned from Postgres")
	}

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestOptUser(t *testing.T) {
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	mock.ExpectQuery("^SELECT (.+) FROM users WHERE user_id = (.+)$").
		WithArgs(UserIDInt).
		WillReturnRows(
			pgxmock.NewRows([]string{"user_id", "opt"}).
				AddRow(UserIDInt, true))

	err = optUser(mock, UserIDInt, true)
	if err == nil {
		t.Error("Expected opting a user that is already opted to fail with error")
	}

	mock.ExpectQuery("^SELECT (.+) FROM users WHERE user_id = (.+)$").
		WithArgs(UserIDInt).
		WillReturnRows(
			pgxmock.NewRows([]string{"user_id", "opt"}).
				AddRow(UserIDInt, true))

	// expect to de-op the user
	mock.ExpectExec("^UPDATE users SET opt = (.+) WHERE user_id = (.+)$").
		WithArgs(false, UserIDInt).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// expect the respective game_events to be unlinked from the user
	mock.ExpectExec("^UPDATE game_events SET user_id = NULL WHERE user_id = (.+)$").
		WithArgs(UserIDInt).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// expect all the user's games to be deleted
	mock.ExpectExec("^DELETE FROM users_games WHERE user_id = (.+)$").
		WithArgs(UserIDInt).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = optUser(mock, UserIDInt, false)
	if err != nil {
		t.Error(err)
	}

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
