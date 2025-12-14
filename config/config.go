package config

import "time"

type Config struct {
	Addrs               string
	BaseURL             string
	DBFile              string
	DataPath            string //  procesed files storage path
	DiscordClientID     string
	DiscordClientSecret string
	DiscordOAuthEnabled bool
	EmailDomain         string
	GitHubClientID      string
	GitHubClientSecret  string
	GitTag              string
	GithubOAuthEnabled  bool
	ResendAPIKey        string
	SessionDuration     time.Duration
	SiteDescription     string
	SiteTitle           string
	UploadPath          string // file upload storage path (temporary before processing)
}

var Cfg = &Config{
	Addrs:           ":3210",
	BaseURL:         "http://localhost:3210",
	SiteTitle:       "devengine",
	SiteDescription: "devengine",
	GitTag:          "dev",
	DBFile:          "devengine.db",
	SessionDuration: 10 * 24 * time.Hour, // 10 days
	UploadPath:      "./uploads",
	DataPath:        "./data",
}
