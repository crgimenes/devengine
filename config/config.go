package config

import "time"

type Config struct {
	Addrs           string
	BaseURL         string
	DBFile          string
	DataPath        string // processed files storage path
	GitTag          string
	LoginURL        string // path the engine redirects unauthenticated requests to (relative to BaseURL)
	SessionDuration time.Duration
	SiteDescription string
	SiteTitle       string
	UploadPath      string // file upload storage path (temporary before processing)

	// Rate limiting for the public auth endpoints (login, signup). Token
	// bucket per client IP: PerMin is the sustained budget, Burst the
	// instant allowance. PerMin <= 0 disables it. TrustProxy switches the
	// client key to the first X-Forwarded-For hop — only behind a reverse
	// proxy, never with the port exposed directly.
	RateLimitPerMin     int
	RateLimitBurst      int
	RateLimitTrustProxy bool

	// FileQuotaMB caps the total bytes each user may keep in the file
	// manager (soft-deleted files count: they stay on disk until purged).
	// 0 disables the quota.
	FileQuotaMB int
}

var Cfg = &Config{
	Addrs:           ":3210",
	BaseURL:         "http://localhost:3210",
	SiteTitle:       "devengine",
	SiteDescription: "devengine",
	GitTag:          "dev",
	DBFile:          "devengine.db",
	LoginURL:        "/login",
	SessionDuration: 10 * 24 * time.Hour, // 10 days
	UploadPath:      "./uploads",
	DataPath:        "./data",
	RateLimitPerMin: 20,
	RateLimitBurst:  10,
	FileQuotaMB:     1024,
}
