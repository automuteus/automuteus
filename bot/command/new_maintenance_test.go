package command

import (
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
)

func TestNewResponse_MaintenanceIsPublicAndCarriesTheNotice(t *testing.T) {
	resp := NewResponse(NewMaintenance, NewInfo{Notice: "Galactus is restarting"}, settings.MakeGuildSettings())
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Error("maintenance refusal should be visible to the channel, not ephemeral")
	}
	if !strings.Contains(resp.Data.Content, "Galactus is restarting") || !strings.Contains(resp.Data.Content, "can't start games") {
		t.Errorf("content = %q", resp.Data.Content)
	}
	if len(resp.Data.Embeds) != 0 {
		t.Error("maintenance refusal should not include the success embed")
	}
}
