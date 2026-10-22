package main

import "strings"

// price is US dollars per million tokens.
type price struct {
	input, output, cacheRead, cacheWrite float64
}

// prices covers the models the bot is expected to run on. cacheWrite is the 1-hour cache write rate
// (2x input), which is what the system prompt uses.
var prices = map[string]price{
	"claude-sonnet-5-5": {input: 2, output: 10, cacheRead: 0.20, cacheWrite: 4},
	"claude-haiku-5-5":  {input: 0.10, output: 0.50, cacheRead: 0.01, cacheWrite: 0.20},
	"claude-opus-5-5":   {input: 4, output: 20, cacheRead: 0.20, cacheWrite: 8},
}

// usage is the token counts of one request.
type usage struct {
	Input, Output, CacheRead, CacheWrite int64
}

func (u *usage) add(o usage) {
	u.Input += o.Input
	u.Output += o.Output
	u.CacheRead += o.CacheRead
	u.CacheWrite += o.CacheWrite
}

// cost is the request's price in dollars. A model that isn't in the table (a refusal fallback, say) is
// priced as Opus, so the daily budget errs on the side of stopping early.
func cost(model string, u usage) float64 {
	p, ok := prices[model]
	if !ok {
		for name, candidate := range prices {
			if strings.HasPrefix(model, name) {
				p, ok = candidate, true
				break
			}
		}
	}
	if !ok {
		p = prices["claude-opus-5-5"]
	}
	return (float64(u.Input)*p.input + float64(u.Output)*p.output +
		float64(u.CacheRead)*p.cacheRead + float64(u.CacheWrite)*p.cacheWrite) / 1e6
}
