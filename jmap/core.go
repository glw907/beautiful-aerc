package jmap

// An Echo is the arguments to Core/echo, which the server returns
// unchanged (RFC 8620 section 4). Any JSON object will do, so this is
// the cheapest way to prove a session and a set of credentials reach
// a working API endpoint.
type Echo map[string]any

func (Echo) Name() string { return "Core/echo" }

func (Echo) Requires() []URI { return nil }
