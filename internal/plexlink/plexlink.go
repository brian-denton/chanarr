// Package plexlink implements the optional, prompted connection to a Plex
// server that lets chanarr push guide-reload notifications instead of
// waiting on Plex's own ~daily XMLTV re-poll. See spec.md §6 and
// docs/adr/0002-optional-prompted-plex-connection.md.
//
// Never required at onboarding: a channel works and plays without this.
// Uses Plex's PIN-based link flow (plex.tv/link) — the user is shown a
// short code and enters it at plex.tv; chanarr polls for the resulting
// token. No manually-copied X-Plex-Token.
package plexlink

// StartLink begins a PIN-link attempt and returns the code to show the
// user. TODO: implement against plex.tv's PIN API.
func StartLink() (code string, err error) {
	return "", errNotImplemented
}

// PollLink checks whether the user has completed linking at plex.tv, and if
// so returns the resulting token. TODO: implement.
func PollLink(pinID string) (token string, linked bool, err error) {
	return "", false, errNotImplemented
}
