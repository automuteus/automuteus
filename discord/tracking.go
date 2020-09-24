package discord

import (
	"bytes"
	"fmt"
	"sync"
)

// Tracking struct
type TrackingChannel struct {
	channelID   string
	channelName string
	forGhosts   bool
}

type Tracking struct {
	tracking map[string]TrackingChannel
	lock     sync.RWMutex
}

func MakeTracking() Tracking {
	return Tracking{
		tracking: map[string]TrackingChannel{},
		lock:     sync.RWMutex{},
	}
}

func (tracking *Tracking) IsTracked(channelID string) bool {
	tracking.lock.Lock()
	defer tracking.lock.Unlock()

	if channelID == "" || len(tracking.tracking) == 0 {
		return true
	}

	for _, v := range tracking.tracking {
		if v.channelID == channelID {
			return true
		}
	}
	return false
}

func (tracking *Tracking) ToStatusString() string {
	tracking.lock.RLock()
	defer tracking.lock.RUnlock()

	if len(tracking.tracking) == 0 {
		return "Any Voice Channel"
	}

	buf := bytes.NewBuffer([]byte{})
	i := 0
	for _, v := range tracking.tracking {
		buf.WriteString(fmt.Sprintf("%s ", v.channelName))
		if v.forGhosts {
			buf.WriteString(" (ghosts) ")
		}
		if i < len(tracking.tracking)-1 {
			buf.WriteString("or ")
		}
		i++
	}
	return buf.String()
}

func (tracking *Tracking) Reset() {
	tracking.lock.Lock()
	tracking.tracking = map[string]TrackingChannel{}
	tracking.lock.Unlock()
}

func (tracking *Tracking) FindAnyTrackedChannel(forGhosts bool) (TrackingChannel, error) {
	tracking.lock.RLock()
	defer tracking.lock.RUnlock()

	for _, v := range tracking.tracking {
		if v.forGhosts == forGhosts {
			return v, nil
		}
	}
	return TrackingChannel{}, fmt.Errorf("No voice channel found forGhosts: %v", forGhosts)
}

func (tracking *Tracking) AddTrackedChannel(id, name string, forGhosts bool) {
	tracking.lock.Lock()
	tracking.tracking[id] = TrackingChannel{
		channelID:   id,
		channelName: name,
		forGhosts:   forGhosts,
	}
	tracking.lock.Unlock()
}
