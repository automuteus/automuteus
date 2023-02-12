package storage

import (
	"github.com/jackc/pgconn"
	"github.com/pashagolub/pgxmock"
	"testing"
	"time"
)

func TestCanTransfer(t *testing.T) {
	origin := &PostgresGuild{
		GuildID: 123,
		Premium: 0,
	}
	dest := &PostgresGuild{
		GuildID: 321,
		Premium: 0,
	}
	err := CanTransfer(origin, dest)
	if err == nil {
		t.Error("can't transfer from a free tier server")
	}
	origin.Premium = 1
	err = CanTransfer(origin, dest)
	if err == nil {
		t.Error("can't transfer a server with no transaction details")
	}
	var tt = int32(0)
	origin.TxTimeUnix = &tt
	err = CanTransfer(origin, dest)
	if err == nil {
		t.Error("can't transfer a server with expired premium")
	}
	tt = int32(time.Now().Unix())
	err = CanTransfer(origin, dest)

	// valid transfer
	if err != nil {
		t.Error(err)
	}

	origin.TransferredTo = &dest.GuildID
	err = CanTransfer(origin, dest)
	if err == nil {
		t.Error("can't transfer a server that has already been transferred")
	}

	origin.TransferredTo = nil
	origin.InheritsFrom = &dest.GuildID
	err = CanTransfer(origin, dest)
	if err == nil {
		t.Error("can't transfer a server that inherits status from another")
	}
	origin.InheritsFrom = nil
	dest.TransferredTo = &origin.GuildID
	err = CanTransfer(origin, dest)
	if err == nil {
		t.Error("can't transfer to a server that has transferred its status to another")
	}

	dest.TransferredTo = nil
	dest.InheritsFrom = &origin.GuildID
	err = CanTransfer(origin, dest)
	if err == nil {
		t.Error("can't transfer to a server that inherits its status from another")
	}

	dest.InheritsFrom = nil
	dest.Premium = 2
	err = CanTransfer(origin, dest)
	if err == nil {
		t.Error("can't transfer to a server that has existing non-standard premium")
	}

	dest.TxTimeUnix = &tt
	err = CanTransfer(origin, dest)
	if err == nil {
		t.Error("can't transfer to a server with active premium")
	}

	var ttt = int32(0)
	dest.TxTimeUnix = &ttt
	err = CanTransfer(origin, dest)
	if err != nil {
		t.Error(err)
	}
}

func TestCanRevertTransfer(t *testing.T) {
	now := int32(time.Now().Unix())
	origin := &PostgresGuild{
		GuildID:    123,
		Premium:    1,
		TxTimeUnix: &now,
	}
	dest := &PostgresGuild{
		GuildID: 321,
		Premium: 0,
	}
	err := CanTransfer(origin, dest)
	if err != nil {
		t.Error(err)
	}

	// mark the transfer
	origin.TransferredTo = &dest.GuildID
	dest.InheritsFrom = &origin.GuildID

	// regular transfer check should fail
	err = CanTransfer(dest, origin)
	if err == nil {
		t.Error("CanTransfer should not permit transfers back to the original server")
	}

	// special case for transfer revert should succeed
	err = CanRevertTransfer(origin, dest)
	if err != nil {
		t.Error(err)
	}
}

func TestCanRevertTransferMock(t *testing.T) {
	origin, dest := uint64(123), uint64(321)
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	now := int32(time.Now().Unix())
	mock.ExpectQuery("^SELECT (.+) FROM guilds WHERE guild_id = (.+)$").
		WithArgs(origin).
		WillReturnRows(
			pgxmock.NewRows([]string{"guild_id", "guild_name", "premium", "tx_time_unix", "transferred_to", "inherits_from"}).
				AddRow(origin, "original", int16(1), &now, nil, nil))

	mock.ExpectQuery("^SELECT (.+) FROM guilds WHERE guild_id = (.+)$").
		WithArgs(dest).
		WillReturnRows(
			pgxmock.NewRows([]string{"guild_id", "guild_name", "premium", "tx_time_unix", "transferred_to", "inherits_from"}).
				AddRow(dest, "transferred", int16(0), &now, nil, nil))

	err = revertPremiumTransfer(mock, "123", "321")
	if err == nil {
		t.Error("should not be capable of transferring non-linked servers")
	}

	wrongOrigin, wrongDest := uint64(345), uint64(567)

	mock.ExpectQuery("^SELECT (.+) FROM guilds WHERE guild_id = (.+)$").
		WithArgs(origin).
		WillReturnRows(
			pgxmock.NewRows([]string{"guild_id", "guild_name", "premium", "tx_time_unix", "transferred_to", "inherits_from"}).
				AddRow(origin, "original", int16(1), &now, &wrongDest, nil))

	mock.ExpectQuery("^SELECT (.+) FROM guilds WHERE guild_id = (.+)$").
		WithArgs(dest).
		WillReturnRows(
			pgxmock.NewRows([]string{"guild_id", "guild_name", "premium", "tx_time_unix", "transferred_to", "inherits_from"}).
				AddRow(dest, "transferred", int16(0), &now, &origin, nil))

	err = revertPremiumTransfer(mock, "123", "321")
	if err == nil {
		t.Error("should not be capable of transferring non-linked servers")
	}

	mock.ExpectQuery("^SELECT (.+) FROM guilds WHERE guild_id = (.+)$").
		WithArgs(origin).
		WillReturnRows(
			pgxmock.NewRows([]string{"guild_id", "guild_name", "premium", "tx_time_unix", "transferred_to", "inherits_from"}).
				AddRow(origin, "original", int16(1), &now, &dest, nil))

	mock.ExpectQuery("^SELECT (.+) FROM guilds WHERE guild_id = (.+)$").
		WithArgs(dest).
		WillReturnRows(
			pgxmock.NewRows([]string{"guild_id", "guild_name", "premium", "tx_time_unix", "transferred_to", "inherits_from"}).
				AddRow(dest, "transferred", int16(0), &now, &wrongOrigin, nil))

	err = revertPremiumTransfer(mock, "123", "321")
	if err == nil {
		t.Error("should not be capable of transferring non-linked servers")
	}

	mock.ExpectQuery("^SELECT (.+) FROM guilds WHERE guild_id = (.+)$").
		WithArgs(origin).
		WillReturnRows(
			pgxmock.NewRows([]string{"guild_id", "guild_name", "premium", "tx_time_unix", "transferred_to", "inherits_from"}).
				AddRow(origin, "original", int16(1), &now, &dest, nil))

	mock.ExpectQuery("^SELECT (.+) FROM guilds WHERE guild_id = (.+)$").
		WithArgs(dest).
		WillReturnRows(
			pgxmock.NewRows([]string{"guild_id", "guild_name", "premium", "tx_time_unix", "transferred_to", "inherits_from"}).
				AddRow(dest, "transferred", int16(0), nil, nil, &origin))

	// correct case; expect inherits and transferred to be wiped from both servers
	mock.ExpectExec("^UPDATE guilds SET inherits_from = NULL WHERE guild_id = (.+)$").
		WithArgs("321").
		WillReturnResult(pgconn.CommandTag{})

	mock.ExpectExec("^UPDATE guilds SET transferred_to = NULL WHERE guild_id = (.+)$").
		WithArgs("123").
		WillReturnResult(pgconn.CommandTag{})

	err = revertPremiumTransfer(mock, "123", "321")
	if err != nil {
		t.Error(err)
	}

	if err = mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
