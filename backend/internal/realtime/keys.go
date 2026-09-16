package realtime

// Redis key builders. We prefix every key with `pp:` so that this database
// namespace is self-documenting inside redis-cli.

func PollVotesKey(slug string) string      { return "pp:poll:" + slug + ":votes" }
func PollVotersKey(slug string) string     { return "pp:poll:" + slug + ":voters" }
func PollTotalKey(slug string) string      { return "pp:poll:" + slug + ":total" }
func PollEventsChannel(slug string) string { return "pp:poll:" + slug + ":events" }
