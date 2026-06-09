package basic

// Invite-token persistence on top of the engine's generic magic_token table.

import (
	"time"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/utils"
)

// inviteAction is the magic_token.action value used by basic auth invites.
const inviteAction = "invite"

// InviteTTL is the default lifetime of an invite token.
const InviteTTL = 7 * 24 * time.Hour

// CreateInvite generates a single-use invite token bound to the given
// invitee email. The returned token must be embedded in the URL that the
// admin sends out of band; the email is recovered when the invitee accepts.
func CreateInvite(inviteeEmail string) (string, error) {
	email, err := utils.CanonicalizeEmail(inviteeEmail)
	if err != nil {
		return "", err
	}

	token := utils.NewOpaqueID()
	expires := time.Now().UTC().Add(InviteTTL)

	err = db.Storage.StoreToken(token, email, inviteAction, expires)
	if err != nil {
		return "", err
	}

	return token, nil
}

// ConsumeInvite validates and consumes an invite token, returning the email
// it was issued for. An empty email (with nil error) means the token did not
// exist, was already used, or expired.
func ConsumeInvite(token string) (string, error) {
	return db.Storage.ConsumeToken(token, inviteAction)
}
