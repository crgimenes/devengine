// Package storetest is the conformance battery for db.Store implementations.
// Every backend package runs Run against a real database it provisions
// itself: db/sqlite with a temp file, db/postgres with a container. The
// battery exercises the contract only — anything implementation-specific
// (PRAGMAs, dialect SQL) belongs in the backend's own tests.
package storetest

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/crgimenes/devengine/db"
)

// Factory returns a migrated, empty store. It is called once per subtest so
// domains do not contaminate each other; cleanup belongs to the factory
// (t.Cleanup with Close).
type Factory func(t *testing.T) db.Store

// Run exercises a db.Store implementation against the contract.
func Run(t *testing.T, open Factory) {
	t.Run("Users", func(t *testing.T) { testUsers(t, open(t)) })
	t.Run("Tokens", func(t *testing.T) { testTokens(t, open(t)) })
	t.Run("Files", func(t *testing.T) { testFiles(t, open(t)) })
	t.Run("EAV", func(t *testing.T) { testEAV(t, open(t)) })
	t.Run("EAVCursor", func(t *testing.T) { testEAVCursor(t, open(t)) })
	t.Run("Forms", func(t *testing.T) { testForms(t, open(t)) })
	t.Run("Menus", func(t *testing.T) { testMenus(t, open(t)) })
	t.Run("I18n", func(t *testing.T) { testI18n(t, open(t)) })
	t.Run("Schema", func(t *testing.T) { testSchema(t, open(t)) })
	t.Run("APITokens", func(t *testing.T) { testAPITokens(t, open(t)) })
	t.Run("Tx", func(t *testing.T) { testTx(t, open(t)) })
	t.Run("Core", func(t *testing.T) { testCore(t, open(t)) })
}

// testCore exercises the raw CoreStore surface external integrations use
// (filodb adapter, snapshot CLI). Statements avoid placeholders on purpose:
// their syntax is dialect-specific and outside this battery's scope.
func testCore(t *testing.T, s db.Store) {
	if s.RW() == nil || s.RO() == nil {
		t.Fatal("RW/RO must expose the underlying pools")
	}

	err := s.Exec("CREATE TABLE qa_core (id INTEGER, name TEXT)")
	if err != nil {
		t.Fatalf("Exec create: %v", err)
	}
	err = s.Exec("INSERT INTO qa_core (id, name) VALUES (1, 'ana'), (2, 'bruno')")
	if err != nil {
		t.Fatalf("Exec insert: %v", err)
	}

	var n int
	err = s.QueryRow("SELECT count(*) FROM qa_core").Scan(&n)
	if err != nil || n != 2 {
		t.Fatalf("QueryRow = %d, %v", n, err)
	}
	err = s.QueryRowRW("SELECT count(*) FROM qa_core").Scan(&n)
	if err != nil || n != 2 {
		t.Fatalf("QueryRowRW = %d, %v", n, err)
	}

	for _, q := range []func(string, ...any) (*sql.Rows, error){s.Query, s.QueryRW} {
		rows, err := q("SELECT name FROM qa_core ORDER BY id")
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		var names []string
		for rows.Next() {
			var name string
			err = rows.Scan(&name)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			names = append(names, name)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil || len(names) != 2 || names[0] != "ana" {
			t.Fatalf("rows = %v, %v", names, err)
		}
	}

	err = s.CheckpointWAL()
	if err != nil {
		t.Fatalf("CheckpointWAL: %v", err)
	}
}

func testUsers(t *testing.T, s db.Store) {
	ana, err := s.CreateUser("ana", "ana@example.com", "hash-1", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if ana.ID == 0 || ana.ReferenceID == "" {
		t.Fatalf("CreateUser returned incomplete user: %+v", ana)
	}
	if !ana.Enabled {
		t.Fatal("created user must start enabled")
	}

	_, err = s.CreateUser("ana", "other@example.com", "hash-2", false)
	if err == nil {
		t.Fatal("duplicate username must fail")
	}

	boss, err := s.CreateUser("boss", "boss@example.com", "hash-3", true)
	if err != nil {
		t.Fatalf("CreateUser sysop: %v", err)
	}
	if !boss.Sysop {
		t.Fatal("sysop flag not persisted")
	}

	byID, err := s.GetUserByID(ana.ID)
	if err != nil || byID == nil || byID.Username != "ana" {
		t.Fatalf("GetUserByID: %+v, %v", byID, err)
	}
	byEmail, err := s.GetUserByEmail("ana@example.com")
	if err != nil || byEmail == nil || byEmail.ID != ana.ID {
		t.Fatalf("GetUserByEmail: %+v, %v", byEmail, err)
	}
	byName, err := s.GetUserByUsername("ana")
	if err != nil || byName == nil || byName.PasswordHash != "hash-1" {
		t.Fatalf("GetUserByUsername: %+v, %v", byName, err)
	}
	byRef, err := s.GetUserByRefID(ana.ReferenceID)
	if err != nil || byRef == nil || byRef.ID != ana.ID {
		t.Fatalf("GetUserByRefID: %+v, %v", byRef, err)
	}
	missing, err := s.GetUserByUsername("nobody")
	if err != nil || missing != nil {
		t.Fatalf("GetUserByUsername(nobody) = %+v, %v; want nil, nil", missing, err)
	}

	users, total, err := s.ListUsers(10, 0)
	if err != nil || total != 2 || len(users) != 2 {
		t.Fatalf("ListUsers = %d users, total %d, %v", len(users), total, err)
	}
	page, total, err := s.ListUsers(1, 1)
	if err != nil || total != 2 || len(page) != 1 {
		t.Fatalf("ListUsers paginated = %d users, total %d, %v", len(page), total, err)
	}

	n, err := s.CountUsers()
	if err != nil || n != 2 {
		t.Fatalf("CountUsers = %d, %v", n, err)
	}

	testUserMutations(t, s, ana, boss)
}

// testUserMutations covers the write half of UserStore: password, profile,
// flags and locale.
func testUserMutations(t *testing.T, s db.Store, ana, boss *db.User) {
	n, err := s.CountUsersWithUsernamePrefix("an")
	if err != nil || n != 1 {
		t.Fatalf("CountUsersWithUsernamePrefix(an) = %d, %v", n, err)
	}
	uniq, err := s.GenerateUniqueUsername("ana")
	if err != nil || uniq == "ana" || uniq == "" {
		t.Fatalf("GenerateUniqueUsername(ana) = %q, %v", uniq, err)
	}

	err = s.SetPasswordHash(ana.ID, "hash-new")
	if err != nil {
		t.Fatalf("SetPasswordHash: %v", err)
	}
	byName, _ := s.GetUserByUsername("ana")
	if byName.PasswordHash != "hash-new" {
		t.Fatal("SetPasswordHash not persisted")
	}

	updated, err := s.UpdateUserProfile(ana.ID, "ana2", "http://a/av.png", "ana2@example.com")
	if err != nil || updated.Username != "ana2" || updated.Email != "ana2@example.com" {
		t.Fatalf("UpdateUserProfile: %+v, %v", updated, err)
	}

	err = s.UpdateUserSysop(ana.ID, true, boss.ID)
	if err != nil {
		t.Fatalf("UpdateUserSysop: %v", err)
	}
	err = s.UpdateUserSysop(boss.ID, false, boss.ID)
	if err == nil {
		t.Fatal("removing your own sysop flag must fail")
	}

	err = s.UpdateUserEnabled(ana.ID, false, boss.ID)
	if err != nil {
		t.Fatalf("UpdateUserEnabled: %v", err)
	}
	byID, _ := s.GetUserByID(ana.ID)
	if byID.Enabled {
		t.Fatal("UpdateUserEnabled(false) not persisted")
	}

	err = s.UpdateUserLocale(ana.ID, "pt-BR")
	if err != nil {
		t.Fatalf("UpdateUserLocale: %v", err)
	}
	byID, _ = s.GetUserByID(ana.ID)
	if byID.Locale != "pt-BR" {
		t.Fatalf("locale = %q, want pt-BR", byID.Locale)
	}
}

func testTokens(t *testing.T, s db.Store) {
	// The contract requires UTC expiry timestamps.
	err := s.StoreToken("tok-live", "ana@example.com", "invite", time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("StoreToken: %v", err)
	}

	email, err := s.ConsumeToken("tok-live", "other-action")
	if err != nil || email != "" {
		t.Fatalf("ConsumeToken wrong action = %q, %v; want empty", email, err)
	}

	email, err = s.ConsumeToken("tok-live", "invite")
	if err != nil || email != "ana@example.com" {
		t.Fatalf("ConsumeToken = %q, %v", email, err)
	}
	email, err = s.ConsumeToken("tok-live", "invite")
	if err != nil || email != "" {
		t.Fatalf("ConsumeToken twice = %q, %v; want empty", email, err)
	}

	err = s.StoreToken("tok-old", "old@example.com", "invite", time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatalf("StoreToken expired: %v", err)
	}
	email, err = s.ConsumeToken("tok-old", "invite")
	if err != nil || email != "" {
		t.Fatalf("expired token consumed: %q, %v", email, err)
	}
	err = s.PurgeExpiredTokens()
	if err != nil {
		t.Fatalf("PurgeExpiredTokens: %v", err)
	}
}

func testFiles(t *testing.T, s db.Store) {
	owner, err := s.CreateUser("owner", "owner@example.com", "x", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	mk := func(name, original, tag string, size int64) *db.File {
		f, err := s.SaveFileMetadata(&db.File{
			UserID:           owner.ID,
			OriginalFilename: original,
			Filename:         name,
			Filesize:         size,
			Filetype:         "text/plain",
			Filehash:         "hash-" + name,
			Filetag:          tag,
			Filedescription:  "desc " + original,
		})
		if err != nil {
			t.Fatalf("SaveFileMetadata(%s): %v", name, err)
		}
		return f
	}
	a := mk("opaque-a", "relatorio anual.txt", "document", 100)
	mk("opaque-b", "avatar.png", "user_avatar", 50)

	got, err := s.GetFileByFilename("opaque-a")
	if err != nil || got == nil || got.OriginalFilename != "relatorio anual.txt" {
		t.Fatalf("GetFileByFilename: %+v, %v", got, err)
	}
	got, err = s.GetFileByUserIDAndFilename(owner.ID, "opaque-b")
	if err != nil || got == nil || got.Filesize != 50 {
		t.Fatalf("GetFileByUserIDAndFilename: %+v, %v", got, err)
	}
	got, err = s.GetFileByUserReferenceIDAndFilename(owner.ReferenceID, "opaque-a")
	if err != nil || got == nil || got.ID != a.ID {
		t.Fatalf("GetFileByUserReferenceIDAndFilename: %+v, %v", got, err)
	}

	list, err := s.ListFilesByUserID(owner.ID, 0, 10)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListFilesByUserID = %d files, %v", len(list), err)
	}
	sorted, err := s.ListFilesByUserIDSorted(owner.ID, "name_asc", 0, 10)
	if err != nil || len(sorted) != 2 || sorted[0].OriginalFilename != "avatar.png" {
		t.Fatalf("ListFilesByUserIDSorted: %v (first: %v)", err, first(sorted))
	}

	found, err := s.SearchFilesByUserIDFTS(owner.ID, "relatorio", "", 0, 10)
	if err != nil || len(found) != 1 || found[0].Filename != "opaque-a" {
		t.Fatalf("SearchFilesByUserIDFTS(relatorio) = %d files, %v", len(found), err)
	}

	n, err := s.CountFilesByUserID(owner.ID)
	if err != nil || n != 2 {
		t.Fatalf("CountFilesByUserID = %d, %v", n, err)
	}
	sum, err := s.SumFileSizesByUserID(owner.ID)
	if err != nil || sum != 150 {
		t.Fatalf("SumFileSizesByUserID = %d, %v; want 150", sum, err)
	}

	err = s.UpdateFileMetadataByUserAndFilename(owner.ID, "opaque-a", "nova descricao", "archive")
	if err != nil {
		t.Fatalf("UpdateFileMetadataByUserAndFilename: %v", err)
	}
	got, _ = s.GetFileByFilename("opaque-a")
	if got.Filedescription != "nova descricao" || got.Filetag != "archive" {
		t.Fatalf("metadata not updated: %+v", got)
	}

	err = s.SoftDeleteFileByUserAndFilename(owner.ID, "opaque-a")
	if err != nil {
		t.Fatalf("SoftDeleteFileByUserAndFilename: %v", err)
	}
	list, _ = s.ListFilesByUserID(owner.ID, 0, 10)
	if len(list) != 1 {
		t.Fatalf("soft-deleted file still listed: %d files", len(list))
	}
	// Soft-deleted bytes still count against the quota until a purge exists.
	sum, _ = s.SumFileSizesByUserID(owner.ID)
	if sum != 150 {
		t.Fatalf("quota sum after soft delete = %d, want 150", sum)
	}
}

func first(files []*db.File) *db.File {
	if len(files) == 0 {
		return nil
	}
	return files[0]
}

// seedEntity creates an entity type with one attribute per primitive kind.
func seedEntity(t *testing.T, s db.Store, machine string) (*db.EAVEntityType, map[string]db.EAVAttribute) {
	t.Helper()
	et, err := s.CreateEAVEntityType("Entity "+machine, machine, "conformance", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType(%s): %v", machine, err)
	}
	attrs := map[string]db.EAVAttribute{}
	maxLen := 256
	for _, kind := range []string{"BOOL", "INT", "REAL", "TEXT", "DATETIME"} {
		var ml *int
		if kind == "TEXT" {
			ml = &maxLen
		}
		a, err := s.CreateEAVAttribute(et.ID, "f_"+strings.ToLower(kind), kind+" field", "",
			kind, false, false, false, ml, false, "", nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateEAVAttribute(%s): %v", kind, err)
		}
		attrs[kind] = *a
	}
	return et, attrs
}

func testEAV(t *testing.T, s db.Store) {
	et, attrs := seedEntity(t, s, "cliente")

	byID, err := s.GetEAVEntityTypeByID(et.ID)
	if err != nil || byID == nil || byID.MachineName != "cliente" {
		t.Fatalf("GetEAVEntityTypeByID: %+v, %v", byID, err)
	}
	byRef, err := s.GetEAVEntityTypeByRefID(et.ReferenceID)
	if err != nil || byRef == nil || byRef.ID != et.ID {
		t.Fatalf("GetEAVEntityTypeByRefID: %+v, %v", byRef, err)
	}
	// machine_name lookup is case-insensitive (NOCASE in SQLite, LOWER in PG).
	byMachine, err := s.GetEAVEntityTypeByMachineName("CLIENTE")
	if err != nil || byMachine == nil || byMachine.ID != et.ID {
		t.Fatalf("GetEAVEntityTypeByMachineName(CLIENTE): %+v, %v", byMachine, err)
	}

	list, err := s.ListEAVEntityTypes()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListEAVEntityTypes = %d, %v", len(list), err)
	}

	upd, err := s.UpdateEAVEntityType(et.ID, "Clientes", "tabela de clientes", "(set ok #t)", "")
	if err != nil || upd.Name != "Clientes" || upd.PreSave != "(set ok #t)" {
		t.Fatalf("UpdateEAVEntityType: %+v, %v", upd, err)
	}

	// Attributes.
	text := attrs["TEXT"]
	aByID, err := s.GetEAVAttributeByID(text.ID)
	if err != nil || aByID == nil || aByID.PrimitiveKind != "TEXT" {
		t.Fatalf("GetEAVAttributeByID: %+v, %v", aByID, err)
	}
	aByRef, err := s.GetEAVAttributeByRefID(text.ReferenceID)
	if err != nil || aByRef == nil || aByRef.ID != text.ID {
		t.Fatalf("GetEAVAttributeByRefID: %+v, %v", aByRef, err)
	}
	all, err := s.ListEAVAttributesByEntityTypeID(et.ID)
	if err != nil || len(all) != 5 {
		t.Fatalf("ListEAVAttributesByEntityTypeID = %d, %v", len(all), err)
	}
	defText := "padrao"
	updAttr, err := s.UpdateEAVAttribute(text.ID, text.MachineName, "Nome", "ajuda",
		"TEXT", true, true, true, text.MaxLength, false, "", nil, nil, nil, &defText, nil)
	if err != nil || !updAttr.IsRequired || !updAttr.IsUnique ||
		updAttr.DefaultVText == nil || *updAttr.DefaultVText != "padrao" {
		t.Fatalf("UpdateEAVAttribute: %+v, %v", updAttr, err)
	}

	testEAVValues(t, s, et, attrs)
}

// testEAVValues covers records, typed values and uniqueness checks.
func testEAVValues(t *testing.T, s db.Store, et *db.EAVEntityType, attrs map[string]db.EAVAttribute) {
	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil || rec.Rev != 1 {
		t.Fatalf("CreateEAVRecord: %+v, %v", rec, err)
	}
	rByID, err := s.GetEAVRecordByID(rec.ID)
	if err != nil || rByID == nil || rByID.ReferenceID != rec.ReferenceID {
		t.Fatalf("GetEAVRecordByID: %+v, %v", rByID, err)
	}
	rByRef, err := s.GetEAVRecordByRefID(rec.ReferenceID)
	if err != nil || rByRef == nil || rByRef.ID != rec.ID {
		t.Fatalf("GetEAVRecordByRefID: %+v, %v", rByRef, err)
	}

	vb, vi, vr, vt, vd := true, int64(42), 3.14, "Ana Souza", "2026-06-12T10:00"
	err = s.UpsertEAVValue(rec.ID, attrs["BOOL"].ID, &vb, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue BOOL: %v", err)
	}
	err = s.UpsertEAVValue(rec.ID, attrs["INT"].ID, nil, &vi, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue INT: %v", err)
	}
	err = s.UpsertEAVValue(rec.ID, attrs["REAL"].ID, nil, nil, &vr, nil, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue REAL: %v", err)
	}
	err = s.UpsertEAVValue(rec.ID, attrs["TEXT"].ID, nil, nil, nil, &vt, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue TEXT: %v", err)
	}
	err = s.UpsertEAVValue(rec.ID, attrs["DATETIME"].ID, nil, nil, nil, nil, &vd)
	if err != nil {
		t.Fatalf("UpsertEAVValue DATETIME: %v", err)
	}

	vals, err := s.GetEAVValuesByRecordID(rec.ID)
	if err != nil || len(vals) != 5 {
		t.Fatalf("GetEAVValuesByRecordID = %d, %v", len(vals), err)
	}
	checkTypedValues(t, vals, attrs)

	byRecord, err := s.GetEAVValuesForRecordIDs([]int64{rec.ID})
	if err != nil || len(byRecord[rec.ID]) != 5 {
		t.Fatalf("GetEAVValuesForRecordIDs: %v, %v", byRecord, err)
	}

	// Update in place: same attribute, new value.
	vt2 := "Bruno Lima"
	err = s.UpsertEAVValue(rec.ID, attrs["TEXT"].ID, nil, nil, nil, &vt2, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue update: %v", err)
	}
	vals, _ = s.GetEAVValuesByRecordID(rec.ID)
	if v := findValue(vals, attrs["TEXT"].ID); v == nil || v.VText == nil || *v.VText != "Bruno Lima" {
		t.Fatalf("upsert did not replace TEXT value: %+v", v)
	}

	// Uniqueness check: another record with the same TEXT value is a conflict.
	rec2, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord 2: %v", err)
	}
	unique, err := s.CheckEAVValueUnique(attrs["TEXT"].ID, "TEXT", "Bruno Lima", rec2.ID)
	if err != nil || unique {
		t.Fatalf("CheckEAVValueUnique(taken) = %v, %v; want false", unique, err)
	}
	unique, err = s.CheckEAVValueUnique(attrs["TEXT"].ID, "TEXT", "Livre", rec2.ID)
	if err != nil || !unique {
		t.Fatalf("CheckEAVValueUnique(free) = %v, %v; want true", unique, err)
	}

	testEAVLocking(t, s, et, attrs, rec, rec2)
}

// testEAVLocking covers rev-based optimistic locking and the soft deletes.
func testEAVLocking(t *testing.T, s db.Store, et *db.EAVEntityType, attrs map[string]db.EAVAttribute, rec, rec2 *db.EAVRecord) {
	vi := int64(42)
	newRev, err := s.UpdateEAVRecordRev(rec.ID, 1)
	if err != nil || newRev != 2 {
		t.Fatalf("UpdateEAVRecordRev = %d, %v", newRev, err)
	}
	_, err = s.UpdateEAVRecordRev(rec.ID, 1)
	if !errors.Is(err, db.ErrConflict) {
		t.Fatalf("UpdateEAVRecordRev stale = %v; want ErrConflict", err)
	}
	err = s.UpdateEAVRecordStatus(rec.ID, 2, "active")
	if err != nil {
		t.Fatalf("UpdateEAVRecordStatus: %v", err)
	}
	err = s.UpdateEAVRecordStatus(rec.ID, 2, "draft")
	if !errors.Is(err, db.ErrConflict) {
		t.Fatalf("UpdateEAVRecordStatus stale = %v; want ErrConflict", err)
	}

	rev, err := s.UpsertEAVValueWithRev(rec.ID, attrs["INT"].ID, 3, nil, &vi, nil, nil, nil)
	if err != nil || rev != 4 {
		t.Fatalf("UpsertEAVValueWithRev = %d, %v", rev, err)
	}
	_, err = s.UpsertEAVValueWithRev(rec.ID, attrs["INT"].ID, 3, nil, &vi, nil, nil, nil)
	if !errors.Is(err, db.ErrConflict) {
		t.Fatalf("UpsertEAVValueWithRev stale = %v; want ErrConflict", err)
	}

	err = s.DeleteEAVValue(rec.ID, attrs["BOOL"].ID)
	if err != nil {
		t.Fatalf("DeleteEAVValue: %v", err)
	}
	vals, _ := s.GetEAVValuesByRecordID(rec.ID)
	if findValue(vals, attrs["BOOL"].ID) != nil {
		t.Fatal("DeleteEAVValue did not remove the value")
	}

	// Listing and soft deletes.
	recs, totalRecs, err := s.ListEAVRecordsByEntityTypeID(et.ID, 10, 0)
	if err != nil || totalRecs != 2 || len(recs) != 2 {
		t.Fatalf("ListEAVRecordsByEntityTypeID = %d/%d, %v", len(recs), totalRecs, err)
	}
	err = s.SoftDeleteEAVRecord(rec2.ID)
	if err != nil {
		t.Fatalf("SoftDeleteEAVRecord: %v", err)
	}
	_, totalRecs, _ = s.ListEAVRecordsByEntityTypeID(et.ID, 10, 0)
	if totalRecs != 1 {
		t.Fatalf("soft-deleted record still counted: %d", totalRecs)
	}

	err = s.SoftDeleteEAVAttribute(attrs["REAL"].ID)
	if err != nil {
		t.Fatalf("SoftDeleteEAVAttribute: %v", err)
	}
	all, _ := s.ListEAVAttributesByEntityTypeID(et.ID)
	if len(all) != 4 {
		t.Fatalf("soft-deleted attribute still listed: %d", len(all))
	}

	err = s.SoftDeleteEAVEntityType(et.ID)
	if err != nil {
		t.Fatalf("SoftDeleteEAVEntityType: %v", err)
	}
	// EAV getters signal missing rows with ErrNotFound (not nil, nil).
	gone, err := s.GetEAVEntityTypeByMachineName("cliente")
	if gone != nil || !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("soft-deleted entity still resolvable: %+v, %v", gone, err)
	}
}

func checkTypedValues(t *testing.T, vals []db.EAVValue, attrs map[string]db.EAVAttribute) {
	t.Helper()
	if v := findValue(vals, attrs["BOOL"].ID); v == nil || v.VBool == nil || !*v.VBool {
		t.Fatalf("BOOL value wrong: %+v", v)
	}
	if v := findValue(vals, attrs["INT"].ID); v == nil || v.VInt == nil || *v.VInt != 42 {
		t.Fatalf("INT value wrong: %+v", v)
	}
	if v := findValue(vals, attrs["REAL"].ID); v == nil || v.VReal == nil || *v.VReal != 3.14 {
		t.Fatalf("REAL value wrong: %+v", v)
	}
	if v := findValue(vals, attrs["TEXT"].ID); v == nil || v.VText == nil || *v.VText != "Ana Souza" {
		t.Fatalf("TEXT value wrong: %+v", v)
	}
	if v := findValue(vals, attrs["DATETIME"].ID); v == nil || v.VDatetime == nil || *v.VDatetime == "" {
		t.Fatalf("DATETIME value wrong: %+v", v)
	}
}

func findValue(vals []db.EAVValue, attrID int64) *db.EAVValue {
	for i := range vals {
		if vals[i].AttributeID == attrID {
			return &vals[i]
		}
	}
	return nil
}

func testEAVCursor(t *testing.T, s db.Store) {
	// Two entities: pedidos reference clientes by reference_id; the cursor
	// text filter must follow that one level of indirection.
	cliente, err := s.CreateEAVEntityType("Cliente", "cliente", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType cliente: %v", err)
	}
	maxLen := 256
	nome, err := s.CreateEAVAttribute(cliente.ID, "nome", "Nome", "", "TEXT",
		false, false, false, &maxLen, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute nome: %v", err)
	}
	pedido, err := s.CreateEAVEntityType("Pedido", "pedido", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType pedido: %v", err)
	}
	clienteRefAttr, err := s.CreateEAVAttribute(pedido.ID, "cliente", "Cliente", "", "TEXT",
		false, false, false, &maxLen, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute cliente ref: %v", err)
	}

	ana, err := s.CreateEAVRecord(cliente.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord ana: %v", err)
	}
	anaName := "Ana Souza"
	err = s.UpsertEAVValue(ana.ID, nome.ID, nil, nil, nil, &anaName, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue ana: %v", err)
	}

	var pedidoIDs []int64
	for range 3 {
		p, err := s.CreateEAVRecord(pedido.ID)
		if err != nil {
			t.Fatalf("CreateEAVRecord pedido: %v", err)
		}
		err = s.UpsertEAVValue(p.ID, clienteRefAttr.ID, nil, nil, nil, &ana.ReferenceID, nil)
		if err != nil {
			t.Fatalf("UpsertEAVValue pedido: %v", err)
		}
		pedidoIDs = append(pedidoIDs, p.ID)
	}

	// Unfiltered, newest first, cursor pagination.
	pageOne, err := s.ListEAVRecordsCursor(pedido.ID, 0, 2, "")
	if err != nil || len(pageOne) != 2 || pageOne[0].ID != pedidoIDs[2] {
		t.Fatalf("cursor page 1 = %v, %v", ids(pageOne), err)
	}
	pageTwo, err := s.ListEAVRecordsCursor(pedido.ID, pageOne[1].ID, 2, "")
	if err != nil || len(pageTwo) != 1 || pageTwo[0].ID != pedidoIDs[0] {
		t.Fatalf("cursor page 2 = %v, %v", ids(pageTwo), err)
	}

	// Direct text filter.
	direct, err := s.ListEAVRecordsCursor(cliente.ID, 0, 10, "souza")
	if err != nil || len(direct) != 1 || direct[0].ID != ana.ID {
		t.Fatalf("text filter = %v, %v", ids(direct), err)
	}

	// Reference indirection: searching pedidos by the client's display text.
	indirect, err := s.ListEAVRecordsCursor(pedido.ID, 0, 10, "Ana")
	if err != nil || len(indirect) != 3 {
		t.Fatalf("reference filter = %v, %v; want all 3 orders", ids(indirect), err)
	}
}

func ids(recs []db.EAVRecord) []int64 {
	out := make([]int64, len(recs))
	for i, r := range recs {
		out[i] = r.ID
	}
	return out
}

func testForms(t *testing.T, s db.Store) {
	et, attrs := seedEntity(t, s, "tarefa")

	form, err := s.CreateForm("cadastro", "Cadastro", "form de teste", &et.ID)
	if err != nil || form.ReferenceID == "" {
		t.Fatalf("CreateForm: %+v, %v", form, err)
	}
	byRef, err := s.GetFormByRefID(form.ReferenceID)
	if err != nil || byRef == nil || byRef.MachineName != "cadastro" {
		t.Fatalf("GetFormByRefID: %+v, %v", byRef, err)
	}
	byMachine, err := s.GetFormByMachineName("cadastro")
	if err != nil || byMachine == nil || byMachine.ID != form.ID {
		t.Fatalf("GetFormByMachineName: %+v, %v", byMachine, err)
	}

	err = s.UpdateForm(form.ID, "cadastro", "Cadastro v2", "atualizado", &et.ID,
		true, true, false, true, nil, true, true)
	if err != nil {
		t.Fatalf("UpdateForm: %v", err)
	}
	byMachine, _ = s.GetFormByMachineName("cadastro")
	if byMachine.Label != "Cadastro v2" || !byMachine.IsSearch || !byMachine.HideSubmitButton || !byMachine.ExposeAPI {
		t.Fatalf("UpdateForm not persisted: %+v", byMachine)
	}

	testFormElements(t, s, form, attrs)

	forms, err := s.ListForms()
	if err != nil || len(forms) != 1 {
		t.Fatalf("ListForms = %d, %v", len(forms), err)
	}
	err = s.SoftDeleteForm(form.ID)
	if err != nil {
		t.Fatalf("SoftDeleteForm: %v", err)
	}
	forms, _ = s.ListForms()
	if len(forms) != 0 {
		t.Fatalf("soft-deleted form still listed: %d", len(forms))
	}
}

// testFormElements covers element CRUD, grouping and move ordering.
func testFormElements(t *testing.T, s db.Store, form *db.Form, attrs map[string]db.EAVAttribute) {
	group, err := s.CreateFormElement(form.ID, nil, "grupo", "group", "Grupo", "",
		1, 12, "", "", nil, true, false)
	if err != nil {
		t.Fatalf("CreateFormElement group: %v", err)
	}
	textAttr := attrs["TEXT"]
	fieldA, err := s.CreateFormElement(form.ID, &group.ID, "campo_a", "field", "Campo A", "",
		1, 6, "text", "{}", &textAttr.ID, false, false)
	if err != nil {
		t.Fatalf("CreateFormElement field A: %v", err)
	}
	intAttr := attrs["INT"]
	fieldB, err := s.CreateFormElement(form.ID, &group.ID, "campo_b", "field", "Campo B", "",
		2, 6, "int", "{}", &intAttr.ID, false, false)
	if err != nil {
		t.Fatalf("CreateFormElement field B: %v", err)
	}

	elements, err := s.ListFormElements(form.ID)
	if err != nil || len(elements) != 3 {
		t.Fatalf("ListFormElements = %d, %v", len(elements), err)
	}
	groups, err := s.ListGroupElements(form.ID)
	if err != nil || len(groups) != 1 || groups[0].ID != group.ID {
		t.Fatalf("ListGroupElements = %d, %v", len(groups), err)
	}
	eByRef, err := s.GetFormElementByRefID(fieldA.ReferenceID)
	if err != nil || eByRef == nil || eByRef.MachineName != "campo_a" {
		t.Fatalf("GetFormElementByRefID: %+v, %v", eByRef, err)
	}

	err = s.UpdateFormElement(fieldA.ID, &group.ID, "campo_a", "field", "Campo A2", "ajuda",
		1, 4, "left", "text", `{"max": 10}`, &textAttr.ID,
		false, true, false, false, "(> (len value) 1)", "", false, "", "", "")
	if err != nil {
		t.Fatalf("UpdateFormElement: %v", err)
	}
	eByRef, _ = s.GetFormElementByRefID(fieldA.ReferenceID)
	if eByRef.Label != "Campo A2" || !eByRef.IsReadonly || eByRef.ValidateExpr == "" {
		t.Fatalf("UpdateFormElement not persisted: %+v", eByRef)
	}

	err = s.MoveElementUp(fieldB.ID)
	if err != nil {
		t.Fatalf("MoveElementUp: %v", err)
	}
	elements, _ = s.ListFormElements(form.ID)
	if pos(elements, fieldB.ID) > pos(elements, fieldA.ID) {
		t.Fatal("MoveElementUp did not reorder")
	}
	err = s.MoveElementDown(fieldB.ID)
	if err != nil {
		t.Fatalf("MoveElementDown: %v", err)
	}

	err = s.DeleteFormElement(fieldB.ID)
	if err != nil {
		t.Fatalf("DeleteFormElement: %v", err)
	}
	elements, _ = s.ListFormElements(form.ID)
	if len(elements) != 2 {
		t.Fatalf("element not deleted: %d left", len(elements))
	}
}

func pos(elements []db.FormElement, id int64) int {
	for i := range elements {
		if elements[i].ID == id {
			return i
		}
	}
	return -1
}

func testMenus(t *testing.T, s db.Store) {
	menu, err := s.CreateMenu("principal", "Principal", "menu de teste")
	if err != nil || menu.ReferenceID == "" {
		t.Fatalf("CreateMenu: %+v, %v", menu, err)
	}
	byID, err := s.GetMenuByID(menu.ID)
	if err != nil || byID == nil || byID.MachineName != "principal" {
		t.Fatalf("GetMenuByID: %+v, %v", byID, err)
	}
	byRef, err := s.GetMenuByRefID(menu.ReferenceID)
	if err != nil || byRef == nil || byRef.ID != menu.ID {
		t.Fatalf("GetMenuByRefID: %+v, %v", byRef, err)
	}
	byMachine, err := s.GetMenuByMachineName("principal")
	if err != nil || byMachine == nil || byMachine.ID != menu.ID {
		t.Fatalf("GetMenuByMachineName: %+v, %v", byMachine, err)
	}

	err = s.UpdateMenu(menu.ID, "principal", "Menu Principal", "atualizado")
	if err != nil {
		t.Fatalf("UpdateMenu: %v", err)
	}

	testMenuItems(t, s, menu)

	menus, err := s.ListMenus()
	if err != nil || len(menus) != 1 {
		t.Fatalf("ListMenus = %d, %v", len(menus), err)
	}
	err = s.SoftDeleteMenu(menu.ID)
	if err != nil {
		t.Fatalf("SoftDeleteMenu: %v", err)
	}
	menus, _ = s.ListMenus()
	if len(menus) != 0 {
		t.Fatalf("soft-deleted menu still listed: %d", len(menus))
	}
}

// testMenuItems covers item CRUD, submenus and move ordering.
func testMenuItems(t *testing.T, s db.Store, menu *db.Menu) {
	home, err := s.CreateMenuItem(menu.ID, nil, "home", "Home", "bi-house", "link",
		"/", "", "", 1)
	if err != nil {
		t.Fatalf("CreateMenuItem: %v", err)
	}
	sub, err := s.CreateMenuItem(menu.ID, nil, "mais", "Mais", "", "submenu",
		"", "", "", 2)
	if err != nil {
		t.Fatalf("CreateMenuItem submenu: %v", err)
	}
	child, err := s.CreateMenuItem(menu.ID, &sub.ID, "sobre", "Sobre", "", "link",
		"/sobre", "", "", 1)
	if err != nil {
		t.Fatalf("CreateMenuItem child: %v", err)
	}

	items, err := s.ListMenuItems(menu.ID)
	if err != nil || len(items) != 3 {
		t.Fatalf("ListMenuItems = %d, %v", len(items), err)
	}
	subs, err := s.ListSubmenuItems(menu.ID)
	if err != nil || len(subs) != 1 || subs[0].ID != sub.ID {
		t.Fatalf("ListSubmenuItems = %d, %v", len(subs), err)
	}
	iByRef, err := s.GetMenuItemByRefID(child.ReferenceID)
	if err != nil || iByRef == nil || iByRef.ParentID == nil || *iByRef.ParentID != sub.ID {
		t.Fatalf("GetMenuItemByRefID: %+v, %v", iByRef, err)
	}

	err = s.UpdateMenuItem(home.ID, nil, "home", "Inicio", "bi-house-fill", "link",
		"/inicio", "", "", 1)
	if err != nil {
		t.Fatalf("UpdateMenuItem: %v", err)
	}
	iByRef, _ = s.GetMenuItemByRefID(home.ReferenceID)
	if iByRef.Label != "Inicio" || iByRef.URL != "/inicio" {
		t.Fatalf("UpdateMenuItem not persisted: %+v", iByRef)
	}

	err = s.MoveMenuItemUp(sub.ID)
	if err != nil {
		t.Fatalf("MoveMenuItemUp: %v", err)
	}
	items, _ = s.ListMenuItems(menu.ID)
	if itemPos(items, sub.ID) > itemPos(items, home.ID) {
		t.Fatal("MoveMenuItemUp did not reorder")
	}
	err = s.MoveMenuItemDown(sub.ID)
	if err != nil {
		t.Fatalf("MoveMenuItemDown: %v", err)
	}

	err = s.DeleteMenuItem(child.ID)
	if err != nil {
		t.Fatalf("DeleteMenuItem: %v", err)
	}
	items, _ = s.ListMenuItems(menu.ID)
	if len(items) != 2 {
		t.Fatalf("item not deleted: %d left", len(items))
	}
}

func itemPos(items []db.MenuItem, id int64) int {
	for i := range items {
		if items[i].ID == id {
			return i
		}
	}
	return -1
}

func testI18n(t *testing.T, s db.Store) {
	err := s.UpsertI18nOverride("pt-BR", "Save", "Salvar")
	if err != nil {
		t.Fatalf("UpsertI18nOverride: %v", err)
	}
	err = s.UpsertI18nOverride("pt-BR", "Save", "Gravar") // overwrite
	if err != nil {
		t.Fatalf("UpsertI18nOverride overwrite: %v", err)
	}
	overrides, err := s.ListI18nOverrides()
	if err != nil || len(overrides) != 1 || overrides[0].Translation != "Gravar" {
		t.Fatalf("ListI18nOverrides = %+v, %v", overrides, err)
	}
	err = s.DeleteI18nOverride("pt-BR", "Save")
	if err != nil {
		t.Fatalf("DeleteI18nOverride: %v", err)
	}
	overrides, _ = s.ListI18nOverrides()
	if len(overrides) != 0 {
		t.Fatalf("override not deleted: %+v", overrides)
	}

	err = s.UpsertContentTranslation("en-US", "ref-1", "label", "Customers")
	if err != nil {
		t.Fatalf("UpsertContentTranslation: %v", err)
	}
	err = s.UpsertContentTranslation("en-US", "ref-1", "label", "Clients") // overwrite
	if err != nil {
		t.Fatalf("UpsertContentTranslation overwrite: %v", err)
	}
	content, err := s.ListContentTranslations()
	if err != nil || len(content) != 1 || content[0].Text != "Clients" {
		t.Fatalf("ListContentTranslations = %+v, %v", content, err)
	}
	err = s.DeleteContentTranslation("en-US", "ref-1", "label")
	if err != nil {
		t.Fatalf("DeleteContentTranslation: %v", err)
	}
	content, _ = s.ListContentTranslations()
	if len(content) != 0 {
		t.Fatalf("content translation not deleted: %+v", content)
	}
}

func testAPITokens(t *testing.T, s db.Store) {
	ana, err := s.CreateUser("ana", "ana@example.com", "x", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	tok, err := s.CreateAPIToken(ana.ID, "hash-aaa", "ci script")
	if err != nil || tok.ID == 0 || tok.UserID != ana.ID || tok.Label != "ci script" {
		t.Fatalf("CreateAPIToken: %+v, %v", tok, err)
	}
	_, err = s.CreateAPIToken(ana.ID, "hash-aaa", "duplicate")
	if err == nil {
		t.Fatal("duplicate token hash must fail")
	}
	tok2, err := s.CreateAPIToken(ana.ID, "hash-bbb", "")
	if err != nil {
		t.Fatalf("CreateAPIToken 2: %v", err)
	}

	list, err := s.ListAPITokensByUserID(ana.ID)
	if err != nil || len(list) != 2 || list[0].ID != tok2.ID {
		t.Fatalf("ListAPITokensByUserID = %d (newest first?), %v", len(list), err)
	}

	owner, err := s.GetUserByAPITokenHash("hash-aaa")
	if err != nil || owner == nil || owner.ID != ana.ID {
		t.Fatalf("GetUserByAPITokenHash: %+v, %v", owner, err)
	}
	owner, err = s.GetUserByAPITokenHash("hash-unknown")
	if err != nil || owner != nil {
		t.Fatalf("unknown hash must return nil, nil: %+v, %v", owner, err)
	}

	// Disabled owner: the token stops resolving.
	boss, err := s.CreateUser("boss", "boss@example.com", "x", true)
	if err != nil {
		t.Fatalf("CreateUser boss: %v", err)
	}
	err = s.UpdateUserEnabled(ana.ID, false, boss.ID)
	if err != nil {
		t.Fatalf("UpdateUserEnabled: %v", err)
	}
	owner, err = s.GetUserByAPITokenHash("hash-aaa")
	if err != nil || owner != nil {
		t.Fatalf("disabled owner token must not resolve: %+v, %v", owner, err)
	}
	err = s.UpdateUserEnabled(ana.ID, true, boss.ID)
	if err != nil {
		t.Fatalf("re-enable: %v", err)
	}

	// Delete guard: another user's id must not revoke the token.
	err = s.DeleteAPIToken(tok.ID, boss.ID)
	if err != nil {
		t.Fatalf("DeleteAPIToken wrong owner: %v", err)
	}
	if owner, _ := s.GetUserByAPITokenHash("hash-aaa"); owner == nil {
		t.Fatal("token deleted by non-owner")
	}
	err = s.DeleteAPIToken(tok.ID, ana.ID)
	if err != nil {
		t.Fatalf("DeleteAPIToken: %v", err)
	}
	if owner, _ := s.GetUserByAPITokenHash("hash-aaa"); owner != nil {
		t.Fatal("deleted token still resolves")
	}
}

func testSchema(t *testing.T, s db.Store) {
	tables, err := s.ListRelationalTables()
	if err != nil {
		t.Fatalf("ListRelationalTables: %v", err)
	}
	names := make(map[string]bool, len(tables))
	for _, tb := range tables {
		names[tb.Name] = true
	}
	if !names["users"] {
		t.Fatalf("ListRelationalTables missing users: %v", names)
	}
	if names["schema_migrations"] {
		t.Fatal("ListRelationalTables must hide schema_migrations")
	}
	for _, hidden := range []string{"eav_entity_types", "eav_values", "forms"} {
		if names[hidden] {
			t.Fatalf("ListRelationalTables must hide %s", hidden)
		}
	}
}

func testTx(t *testing.T, s db.Store) {
	et, attrsByKind := seedEntity(t, s, "estoque")
	attrs, err := s.ListEAVAttributesByEntityTypeID(et.ID)
	if err != nil {
		t.Fatalf("ListEAVAttributesByEntityTypeID: %v", err)
	}

	// Commit path: insert + values + rev bump, all visible afterwards.
	tx, err := s.BeginTransaction()
	if err != nil {
		t.Fatalf("BeginTransaction: %v", err)
	}
	id, refID, err := tx.InsertEAVRecord(et.ID)
	if err != nil || id == 0 || refID == "" {
		t.Fatalf("InsertEAVRecord = %d, %q, %v", id, refID, err)
	}
	err = tx.SaveRecordValues(id, attrs, db.EAVRecordValues{
		"f_text": "produto x",
		"f_int":  int64(7),
		"f_bool": true,
	})
	if err != nil {
		t.Fatalf("SaveRecordValues: %v", err)
	}
	err = tx.BumpEAVRecordRev(id)
	if err != nil {
		t.Fatalf("BumpEAVRecordRev: %v", err)
	}
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	rec, err := s.GetEAVRecordByRefID(refID)
	if err != nil || rec == nil || rec.Rev != 2 {
		t.Fatalf("record after commit: %+v, %v", rec, err)
	}
	vals, _ := s.GetEAVValuesByRecordID(id)
	if v := findValue(vals, attrsByKind["TEXT"].ID); v == nil || v.VText == nil || *v.VText != "produto x" {
		t.Fatalf("TEXT value after commit: %+v", v)
	}
	if v := findValue(vals, attrsByKind["INT"].ID); v == nil || v.VInt == nil || *v.VInt != 7 {
		t.Fatalf("INT value after commit: %+v", v)
	}

	// SaveRecordValues overwrite inside a second transaction.
	tx, err = s.BeginTransaction()
	if err != nil {
		t.Fatalf("BeginTransaction 2: %v", err)
	}
	err = tx.SaveRecordValues(id, attrs, db.EAVRecordValues{"f_text": "produto y"})
	if err != nil {
		t.Fatalf("SaveRecordValues overwrite: %v", err)
	}
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Commit 2: %v", err)
	}
	vals, _ = s.GetEAVValuesByRecordID(id)
	if v := findValue(vals, attrsByKind["TEXT"].ID); v == nil || v.VText == nil || *v.VText != "produto y" {
		t.Fatalf("TEXT value after overwrite: %+v", v)
	}

	// Rollback path: nothing leaks.
	tx, err = s.BeginTransaction()
	if err != nil {
		t.Fatalf("BeginTransaction 3: %v", err)
	}
	ghostID, ghostRef, err := tx.InsertEAVRecord(et.ID)
	if err != nil || ghostID == 0 {
		t.Fatalf("InsertEAVRecord ghost: %v", err)
	}
	err = tx.Rollback()
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	ghost, err := s.GetEAVRecordByRefID(ghostRef)
	if ghost != nil || !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("rolled-back record visible: %+v, %v", ghost, err)
	}
}
