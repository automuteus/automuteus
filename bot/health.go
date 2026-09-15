package bot

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// StaleHeartbeat is how long the gateway may go without acknowledging a heartbeat before the shard is reported
// unhealthy. Discord's interval is about 41s and discordgo only reconnects on its own after five missed acks, so
// this reports a silently dead socket well before the library reacts to it.
const StaleHeartbeat = 2 * time.Minute

// GatewayHealth reports whether this shard's gateway session is connected and still being acknowledged by Discord.
// It is a readiness check for internal/server.Health.
func (bot *Bot) GatewayHealth(context.Context) error {
	s := bot.PrimarySession
	s.RLock()
	ready := s.DataReady
	lastAck := s.LastHeartbeatAck
	s.RUnlock()

	if !ready {
		return errors.New("gateway not connected")
	}
	if age := time.Since(lastAck); age > StaleHeartbeat {
		return fmt.Errorf("no heartbeat ack for %s", age.Truncate(time.Second))
	}
	return nil
}
