package auth

import (
	"context"

	"golang.org/x/oauth2"
	oauth2v2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// GetUserEmail fires a single Userinfo.Get call against Google's
// `https://www.googleapis.com/oauth2/v2/userinfo` endpoint and returns
// the canonical email address bound to the access token.
//
// AUTH-06: This is the canonical identity for everything downstream —
// _char_owner.owner_email, the wincred target-name suffix, the cached
// config.GoogleEmail, the slog `email` field. It is NOT the same as
// `Session.getActiveUser().getEmail()` in Apps Script (which returns
// the script owner, not the writer) — see ARCHITECTURE.md "Identity"
// section for the load-bearing distinction.
//
// The TokenSource MUST already have access to the openid +
// userinfo.email scopes; oauth.go's RunOAuth requests them by default.
func GetUserEmail(ctx context.Context, ts oauth2.TokenSource) (string, error) {
	svc, err := oauth2v2.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return "", err
	}
	info, err := svc.Userinfo.Get().Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return info.Email, nil
}
