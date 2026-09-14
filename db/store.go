package db

import (
	"database/sql"
	"time"
)

// Store is the storage contract the engine programs against. The global
// Storage variable holds the active implementation; db/sqlite is the only one
// today, future backends (PostgreSQL, ...) live in their own packages with
// SQL written for each database — no generic SQL layer.
//
// The contract is split by domain so a future backend can be built and
// reviewed one domain at a time, and so call sites can declare the narrow
// dependency they actually use.
type Store interface {
	CoreStore
	TokenStore
	UserStore
	FileStore
	EAVStore
	FormStore
	MenuStore
	I18nStore
	SchemaStore
	APITokenStore
}

// Tx is a write transaction. Query results stay valid until Commit or
// Rollback; the EAV helpers bundle the multi-statement record operations
// that must not interleave with other writers.
type Tx interface {
	Commit() error
	Rollback() error
	Exec(query string, args ...any) error
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *Row

	InsertEAVRecord(entityTypeID int64) (int64, string, error)
	InsertEAVRecordWithRef(refID string, entityTypeID int64, status string) (int64, error)
	BumpEAVRecordRev(recordID int64) error
	UpsertEAVValue(recordID, attributeID int64, vBool *bool, vInt *int64, vReal *float64, vText, vDatetime *string) error
	ActivateEAVRecord(recordID int64) error
	SaveRecordValues(recordID int64, attributes []EAVAttribute, values EAVRecordValues) error
}

// CoreStore exposes connection-level operations: raw statements, the
// transaction entry point and pool lifecycle. RW/RO leak *sql.DB on purpose
// for integrations (filodb adapter, snapshot CLI) that need direct access.
type CoreStore interface {
	BeginTransaction() (Tx, error)
	Exec(query string, args ...any) error
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *Row
	QueryRW(query string, args ...any) (*sql.Rows, error)
	QueryRowRW(query string, args ...any) *Row
	RW() *sql.DB
	RO() *sql.DB
	CheckpointWAL() error
	Close()
}

// TokenStore persists single-use action tokens (invites, magic links).
// expiresAt MUST be in UTC: expiry is compared against the database clock.
type TokenStore interface {
	StoreToken(token string, email string, action string, expiresAt time.Time) error
	ConsumeToken(token, action string) (string, error)
	PurgeExpiredTokens() error
}

// APITokenStore manages long-lived bearer credentials for the REST API.
// Implementations store only the SHA-256 hash; lookups receive the hash of
// the presented token. GetUserByAPITokenHash returns nil, nil when the hash
// is unknown or the owner is disabled.
type APITokenStore interface {
	CreateAPIToken(userID int64, tokenHash, label string) (*APIToken, error)
	ListAPITokensByUserID(userID int64) ([]APIToken, error)
	DeleteAPIToken(id, userID int64) error
	GetUserByAPITokenHash(tokenHash string) (*User, error)
}

// UserStore manages accounts, credentials and per-user preferences.
type UserStore interface {
	CreateUser(username string, email string, passwordHash string, sysop bool) (*User, error)
	GetUserByID(userID int64) (*User, error)
	GetUserByEmail(email string) (*User, error)
	GetUserByUsername(username string) (*User, error)
	GetUserByRefID(refID string) (*User, error)
	ListUsers(limit, offset int) ([]User, int, error)
	CountUsers() (int, error)
	CountUsersWithUsernamePrefix(prefix string) (int, error)
	GenerateUniqueUsername(baseUsername string) (string, error)
	SetPasswordHash(userID int64, passwordHash string) error
	UpdateUserProfile(userID int64, username string, avatarURL string, email string) (*User, error)
	UpdateUserSysop(userID int64, sysop bool, currentSysopID int64) error
	UpdateUserEnabled(userID int64, enabled bool, currentSysopID int64) error
	UpdateUserLocale(userID int64, locale string) error
}

// FileStore manages filemanager metadata (the bytes live on disk).
type FileStore interface {
	SaveFileMetadata(f *File) (*File, error)
	GetFileByFilename(filename string) (*File, error)
	GetFileByUserIDAndFilename(userID int64, filename string) (*File, error)
	GetFileByUserReferenceIDAndFilename(userRefID string, filename string) (*File, error)
	ListFilesByUserID(userID int64, offset int, limit int) ([]*File, error)
	ListFilesByUserIDSorted(userID int64, sort string, offset int, limit int) ([]*File, error)
	SearchFilesByUserIDFTS(userID int64, query string, sort string, offset int, limit int) ([]*File, error)
	CountFilesByUserID(userID int64) (int, error)
	SumFileSizesByUserID(userID int64) (int64, error)
	UpdateFileMetadataByUserAndFilename(userID int64, filename string, description string, tag string) error
	SoftDeleteFileByUserAndFilename(userID int64, filename string) error
}

// EAVStore manages entity types, attributes, records and values.
// Get* methods return ErrNotFound for missing rows (unlike UserStore and
// FileStore, whose getters return nil, nil); stale-rev writes return
// ErrConflict.
type EAVStore interface {
	CreateEAVEntityType(name, machineName, description, preSave, posLoad string) (*EAVEntityType, error)
	GetEAVEntityTypeByID(id int64) (*EAVEntityType, error)
	GetEAVEntityTypeByRefID(refID string) (*EAVEntityType, error)
	GetEAVEntityTypeByMachineName(machineName string) (*EAVEntityType, error)
	ListEAVEntityTypes() ([]EAVEntityType, error)
	UpdateEAVEntityType(id int64, name, description, preSave, posLoad string) (*EAVEntityType, error)
	SoftDeleteEAVEntityType(id int64) error

	CreateEAVAttribute(
		entityTypeID int64,
		machineName, label, helpText, primitiveKind string,
		isRequired, isUnique, isIndexed bool,
		maxLength *int,
		isComputed bool,
		computedExpr string,
		defaultVBool *bool,
		defaultVInt *int64,
		defaultVReal *float64,
		defaultVText, defaultVDatetime *string,
	) (*EAVAttribute, error)
	GetEAVAttributeByID(id int64) (*EAVAttribute, error)
	GetEAVAttributeByRefID(refID string) (*EAVAttribute, error)
	ListEAVAttributesByEntityTypeID(entityTypeID int64) ([]EAVAttribute, error)
	UpdateEAVAttribute(
		id int64,
		machineName, label, helpText, primitiveKind string,
		isRequired, isUnique, isIndexed bool,
		maxLength *int,
		isComputed bool,
		computedExpr string,
		defaultVBool *bool,
		defaultVInt *int64,
		defaultVReal *float64,
		defaultVText, defaultVDatetime *string,
	) (*EAVAttribute, error)
	SoftDeleteEAVAttribute(id int64) error

	CreateEAVRecord(entityTypeID int64) (*EAVRecord, error)
	GetEAVRecordByID(id int64) (*EAVRecord, error)
	GetEAVRecordByRefID(refID string) (*EAVRecord, error)
	ListEAVRecordsByEntityTypeID(entityTypeID int64, limit, offset int) ([]EAVRecord, int, error)
	ListEAVRecordsCursor(entityTypeID int64, cursorID int64, limit int, textFilter string) ([]EAVRecord, error)
	UpdateEAVRecordRev(id int64, currentRev int) (int, error)
	UpdateEAVRecordStatus(id int64, currentRev int, status string) error
	SoftDeleteEAVRecord(id int64) error

	CheckEAVValueUnique(attributeID int64, primitiveKind string, value any, excludeRecordID int64) (bool, error)
	UpsertEAVValue(recordID, attributeID int64, vBool *bool, vInt *int64, vReal *float64, vText, vDatetime *string) error
	UpsertEAVValueWithRev(recordID, attributeID int64, currentRev int, vBool *bool, vInt *int64, vReal *float64, vText, vDatetime *string) (int, error)
	DeleteEAVValue(recordID, attributeID int64) error
	GetEAVValuesByRecordID(recordID int64) ([]EAVValue, error)
	GetEAVValuesForRecordIDs(ids []int64) (map[int64][]EAVValue, error)

	CountEAVRecords(entityTypeID int64) (int, error)
	CountEAVRecordsWhere(entityTypeID, attributeID int64, primitiveKind string, value any) (int, error)
	ListEAVRecordsByAttributeValue(entityTypeID, attributeID int64, value string) ([]EAVRecord, error)
}

// FormStore manages form definitions and their elements.
type FormStore interface {
	CreateForm(machineName, label, description string, eavEntityTypeID *int64) (*Form, error)
	GetFormByRefID(refID string) (*Form, error)
	GetFormByMachineName(machineName string) (*Form, error)
	ListForms() ([]Form, error)
	UpdateForm(id int64, machineName, label, description string, eavEntityTypeID *int64, hideSubmitButton, hideCancelButton, hideTitle, showSystemInfo bool, menuID *int64, isSearch, exposeAPI bool) error
	SoftDeleteForm(id int64) error

	CreateFormElement(
		formID int64,
		parentID *int64,
		machineName, elementKind, label, helpText string,
		zOrder, colSpan int,
		uiKind, uiMetaJSON string,
		eavAttributeID *int64,
		isUIOnly, isReadonly bool,
	) (*FormElement, error)
	GetFormElementByRefID(refID string) (*FormElement, error)
	ListFormElements(formID int64) ([]FormElement, error)
	ListGroupElements(formID int64) ([]FormElement, error)
	UpdateFormElement(
		id int64,
		parentID *int64,
		machineName, elementKind, label, helpText string,
		zOrder, colSpan int,
		alignment string,
		uiKind, uiMetaJSON string,
		eavAttributeID *int64,
		isUIOnly, isReadonly, hideLabel, hideHelpText bool,
		validateExpr string,
		buttonFiloCode string,
		buttonRunSave bool,
		buttonJSCode, buttonStyle, buttonConfirmMsg string,
	) error
	DeleteFormElement(id int64) error
	MoveElementUp(elementID int64) error
	MoveElementDown(elementID int64) error
}

// MenuStore manages menus and their (possibly nested) items.
type MenuStore interface {
	CreateMenu(machineName, label, description string) (*Menu, error)
	GetMenuByID(id int64) (*Menu, error)
	GetMenuByRefID(refID string) (*Menu, error)
	GetMenuByMachineName(machineName string) (*Menu, error)
	ListMenus() ([]Menu, error)
	UpdateMenu(id int64, machineName, label, description string) error
	SoftDeleteMenu(id int64) error

	CreateMenuItem(menuID int64, parentID *int64, machineName, label, icon, itemType, url, jsCode, filoCode string, zOrder int) (*MenuItem, error)
	GetMenuItemByRefID(refID string) (*MenuItem, error)
	ListMenuItems(menuID int64) ([]MenuItem, error)
	ListSubmenuItems(menuID int64) ([]MenuItem, error)
	UpdateMenuItem(id int64, parentID *int64, machineName, label, icon, itemType, url, jsCode, filoCode string, zOrder int) error
	DeleteMenuItem(id int64) error
	MoveMenuItemUp(itemID int64) error
	MoveMenuItemDown(itemID int64) error
}

// I18nStore persists translation overrides for system strings and content
// translations for user-created labels (the i18n package caches both).
type I18nStore interface {
	ListI18nOverrides() ([]I18nOverride, error)
	UpsertI18nOverride(locale, msgKey, translation string) error
	DeleteI18nOverride(locale, msgKey string) error

	ListContentTranslations() ([]ContentTranslation, error)
	UpsertContentTranslation(locale, refID, field, text string) error
	DeleteContentTranslation(locale, refID, field string) error
}

// SchemaStore introspects the relational schema for the tools UI.
type SchemaStore interface {
	ListRelationalTables() ([]TableInfo, error)
}
