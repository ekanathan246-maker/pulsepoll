package realtime

// Redis key builders. We prefix every key with `pp:` so that this database
// namespace is self-documenting inside redis-cli.

func PollCountsKey(slug string) string      { return "pp:poll:" + slug + ":counts" }
func PollVotersKey(slug string) string      { return "pp:poll:" + slug + ":voters" }
func PollTotalKey(slug string) string       { return "pp:poll:" + slug + ":total" }
func PollEventsChannel(slug string) string  { return "pp:poll:" + slug + ":events" }
func PollVersionKey(slug string) string     { return "pp:poll:" + slug + ":version" }
func PollStatusKey(slug string) string      { return "pp:poll:" + slug + ":status" }
func AppliedEventKey(eventID string) string { return "pp:outbox:applied:" + eventID }
