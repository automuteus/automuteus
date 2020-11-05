package discord

import (
	"github.com/bwmarrin/discordgo"
	"log"
	"sync"
)

type SessionManager struct {
	PrimarySession *discordgo.Session
	AltSession     *discordgo.Session
	//AltSessionGuilds is a record of which guilds also have the 2nd bot added to them (and therefore should be allowed to
	//use the 2nd bot token
	AltSessionGuilds map[string]struct{}
	count            int
	lock             sync.RWMutex
}

func NewSessionManager(primary, secondary *discordgo.Session) *SessionManager {
	return &SessionManager{
		PrimarySession:   primary,
		AltSession:       secondary,
		AltSessionGuilds: make(map[string]struct{}),
		count:            0,
		lock:             sync.RWMutex{},
	}
}

func (sm *SessionManager) GetPrimarySession() *discordgo.Session {
	return sm.PrimarySession
}

func (sm *SessionManager) GetSessionForRequest(guildID string) *discordgo.Session {
	if sm.AltSession == nil {
		return sm.PrimarySession
	}
	sm.lock.Lock()
	defer sm.lock.Unlock()

	//only bother using a separate token/session if the guild also has that bot invited/a member
	if _, hasSecond := sm.AltSessionGuilds[guildID]; hasSecond {
		sm.count++
		if sm.count%2 == 0 {
			log.Println("Using primary session for request")
			return sm.PrimarySession
		} else {
			log.Println("Using secondary session for request")
			return sm.AltSession
		}
	} else {
		log.Println("Using primary session for request")
		return sm.PrimarySession
	}
}

func (sm *SessionManager) Close() {
	if sm.PrimarySession != nil {
		sm.PrimarySession.Close()
	}

	if sm.AltSession != nil {
		sm.AltSession.Close()
	}
}

func (sm *SessionManager) RegisterGuildSecondSession(guildID string) {
	sm.lock.Lock()
	sm.AltSessionGuilds[guildID] = struct{}{}
	sm.lock.Unlock()
}
