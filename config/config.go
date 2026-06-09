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
}
