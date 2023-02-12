package premium

type Tier int16

const (
	FreeTier Tier = iota
	BronzeTier
	SilverTier
	GoldTier
	TrialTier
	SelfHostTier
)

var TierStrings = []string{
	"Free",
	"Bronze",
	"Silver",
	"Gold",
	"Trial",
	"SelfHost",
}

const SubDays = 31         // use 31 because there shouldn't ever be a gap; whenever a renewal happens on the 31st day, that should be valid
const NoExpiryCode = -9999 // dumb, but no one would ever have expired premium for 9999 days

type PremiumRecord struct {
	Tier Tier `json:"tier"`
	Days int  `json:"days"`
}

func IsExpired(tier Tier, days int) bool {
	return tier == FreeTier || (days != NoExpiryCode && days < 1)
}
