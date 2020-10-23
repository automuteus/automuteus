package storage

import (
	"crypto/sha256"
	"encoding/hex"
)

type HashedID string

func HashUserID(userID string) HashedID {
	return genericHash(userID)
}

func HashGuildID(guildID string) HashedID {
	return genericHash(guildID)
}

func genericHash(s string) HashedID {
	h := sha256.New()
	h.Write([]byte(s))
	return HashedID(hex.EncodeToString(h.Sum(nil)))
}
