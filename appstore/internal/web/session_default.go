package web

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ErrNoCachedSession reports that default Apple ID resolution found no cached
// web session to fall back to.
var ErrNoCachedSession = errors.New("no cached web session is available")

var readDefaultSessionFromFileFn = readSessionFromFile

// AmbiguousCachedSessionError reports that more than one cached web session
// could serve as the default, so the caller has to name one.
type AmbiguousCachedSessionError struct {
	AppleIDs []string
}

func (e *AmbiguousCachedSessionError) Error() string {
	return "multiple cached web sessions are available: " + strings.Join(e.AppleIDs, ", ")
}

// DefaultCachedAppleID returns the Apple ID of the only cached web session so
// commands can omit --apple-id when there is nothing to choose between. It
// reports ErrNoCachedSession when the cache is disabled or empty and an
// *AmbiguousCachedSessionError listing every cached Apple ID when there are
// several. Entries that predate stored Apple ID metadata cannot be selected by
// name and are ignored.
func DefaultCachedAppleID() (string, error) {
	appleIDs, err := CachedSessionAppleIDs()
	if err != nil {
		return "", err
	}
	switch len(appleIDs) {
	case 0:
		return "", ErrNoCachedSession
	case 1:
		return appleIDs[0], nil
	default:
		return "", &AmbiguousCachedSessionError{AppleIDs: appleIDs}
	}
}

// CachedSessionAppleIDs lists the Apple IDs of every cached web session in the
// selected backend, sorted case-insensitively. Like the last-session lookup, an
// automatic backend consults the keychain only when the file cache is empty.
func CachedSessionAppleIDs() ([]string, error) {
	selection := resolveBackendSelection()
	sessions, err := listSessionsBySelection(selection)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	appleIDs := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		email := strings.TrimSpace(sess.UserEmail)
		if email == "" {
			continue
		}
		normalized := strings.ToLower(email)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		appleIDs = append(appleIDs, email)
	}
	sort.Slice(appleIDs, func(i, j int) bool {
		return strings.ToLower(appleIDs[i]) < strings.ToLower(appleIDs[j])
	})
	return appleIDs, nil
}

func listSessionsBySelection(selection backendSelection) ([]persistedSession, error) {
	switch selection.backend {
	case sessionBackendOff:
		return nil, nil
	case sessionBackendKeychain:
		sessions, err := listSessionsFromKeychain()
		if err != nil {
			if selection.fallbackFile && isKeyringUnavailable(err) {
				return listSessionsFromFile()
			}
			return nil, err
		}
		if len(sessions) == 0 && selection.fallbackFile {
			return listSessionsFromFile()
		}
		return sessions, nil
	case sessionBackendFile:
		sessions, err := listSessionsFromFile()
		if err != nil {
			return nil, err
		}
		if len(sessions) > 0 || !selection.fallbackKeychain {
			return sessions, nil
		}
		fallback, err := listSessionsFromKeychain()
		if err != nil {
			// Mirror the last-session lookup: a keychain that cannot be read
			// leaves the empty file result standing instead of failing.
			return nil, nil
		}
		return fallback, nil
	default:
		return nil, nil
	}
}

// listSessionsFromFile reads every well-formed, current-version session entry
// in the file cache. Malformed or stale entries are skipped: they cannot be
// resumed, so they must not block the default resolution either.
func listSessionsFromFile() ([]persistedSession, error) {
	dir, err := webSessionCacheDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sessions []persistedSession
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "session-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		key := strings.TrimSuffix(strings.TrimPrefix(name, "session-"), ".json")
		sess, ok, err := readDefaultSessionFromFileFn(key)
		if err != nil {
			if errors.Is(err, errMalformedSessionFile) {
				continue
			}
			return nil, fmt.Errorf("read cached web session %q: %w", name, err)
		}
		if !ok {
			continue
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

func listSessionsFromKeychain() ([]persistedSession, error) {
	kr, err := sessionKeyringOpen()
	if err != nil {
		return nil, err
	}
	store, ok, err := readSessionStoreFromKeyring(kr)
	if err != nil || !ok {
		return nil, err
	}
	sessions := make([]persistedSession, 0, len(store.Sessions))
	for _, sess := range store.Sessions {
		sessions = append(sessions, sess)
	}
	return sessions, nil
}
