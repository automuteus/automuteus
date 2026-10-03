package premium

type Tier int16

const (
	FreeTier Tier = iota
	BronzeTier
	SilverTier
	GoldTier
	// 4 was the top.gg voting trial; the slot stays reserved so tiers stored in guilds.premium keep their meaning.
	_
	SelfHostTier
)

var TierStrings = []string{
	"Free",
	"Bronze",
	"Silver",
	"Gold",
	"Trial", // retired; kept so a stored 4 still has a name
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
