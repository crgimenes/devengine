package oauthproviders

import (
	"sync"
	"time"
)

type stateEntry struct {
	Verifier string
	Expires  time.Time
}

var (
	GitTag = "dev" + time.Now().UTC().Format("-20060102-150405")
	states = struct {
		sync.Mutex
		m map[string]stateEntry
	}{m: make(map[string]stateEntry)}
)

func putState(st, verifier string, ttl time.Duration) {
	states.Lock()
	states.m[st] = stateEntry{Verifier: verifier, Expires: time.Now().Add(ttl)}
	// simple opportunistic cleanup:
	for k, v := range states.m {
		if time.Now().After(v.Expires) {
			delete(states.m, k)
		}
	}
	states.Unlock()
}

func takeState(st string) (string, bool) {
	states.Lock()
	defer func() {
		delete(states.m, st)
		states.Unlock()
	}()
	ent, ok := states.m[st]
	if !ok || time.Now().After(ent.Expires) {
		return "", false
	}
	return ent.Verifier, true
}
