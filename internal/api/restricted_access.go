package api

import (
	"context"
	"net/http"

	"github.com/stashapp/stash/internal/manager/config"
)

func isSessionRestricted(ctx context.Context) bool {
	restricted, ok := ctx.Value(sessionRestrictedKey).(bool)
	if !ok {
		return true
	}

	return restricted
}

func isPathBlockedInRestrictedMode(ctx context.Context, path string) bool {
	if !isSessionRestricted(ctx) {
		return false
	}

	return config.GetInstance().GetStashPaths().IsPathRestricted(path)
}

func denyRestrictedPath(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
}
