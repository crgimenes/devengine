package db

import "time"

type File struct {
	ID               int64
	UserID           int64
	OriginalFilename string
	Filename         string
	Filesize         int64
	Filetype         string
	Filehash         string
	Filetag          string
	Filedescription  string
	Processed        bool
	CreatedAt        string
	UpdatedAt        string
}

type User struct {
	ID           int64     `json:"id"`
	ReferenceID  string    `json:"reference_id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Enabled      bool      `json:"enabled"`
	Sysop        bool      `json:"sysop"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	Locale       string    `json:"locale,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitzero"`
	UpdatedAt    time.Time `json:"updated_at,omitzero"`
}
