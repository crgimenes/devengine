package db

// I18nOverride is one user adjustment to a UI translation.
type I18nOverride struct {
	Locale      string
	MsgKey      string
	Translation string
}

// ContentTranslation is one translated field of a user-created object.
type ContentTranslation struct {
	Locale string
	RefID  string
	Field  string
	Text   string
}
