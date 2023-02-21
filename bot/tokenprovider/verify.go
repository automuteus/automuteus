package tokenprovider

import (
	"log"
)

func (tokenProvider *TokenProvider) verifyBotMembership(guildID string, limit int, uniqueTokensUsed map[string]struct{}) {
	tokenProvider.sessionLock.RLock()
	defer tokenProvider.sessionLock.RUnlock()

	i := 0
	for hToken, sess := range tokenProvider.activeSessions {
		// only check tokens that weren't used successfully already (obv we're members if mute/deafen was successful earlier)
		if !mapHasEntry(uniqueTokensUsed, hToken) {
			_, err := sess.GuildMember(guildID, sess.State.User.ID)
			if err != nil {
				//log.Println(err)
			} else {
				i++ // successfully checked self's membership; we are a member of this server
			}

			// if the bot is verified as a member of too many servers for the premium status, then we should leave them
			if i > limit {
				log.Println("Token/Bot " + hToken + " leaving server " + guildID + " due to lack of premium membership")

				err = sess.GuildLeave(guildID)
				if err != nil {
					log.Println(err)
				}
			}
		}
	}
}
