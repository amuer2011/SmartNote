package core

import (
	"strings"
	"time"
)

// When one device moves a stable-ID note and another edits its old path,
// compare the edited original with the moved remote version as one conflict.
// Do not install a second active file with the same note ID.
func (s *Service) groupSyncMoves(w Workspace, local, downloads map[string][]byte, baseline, next map[string]string, st *syncLocalState) error {
	identities := map[string]string{}
	for key, b := range local {
		if strings.HasPrefix(key, "notes/") && strings.HasSuffix(strings.ToLower(key), ".md") {
			n := parseNote(strings.TrimPrefix(key, "notes/"), b, time.Time{})
			identities[n.ID] = key
		}
	}
	for key, b := range downloads {
		if !strings.HasPrefix(key, "notes/") || !strings.HasSuffix(strings.ToLower(key), ".md") {
			continue
		}
		n := parseNote(strings.TrimPrefix(key, "notes/"), b, time.Time{})
		old := identities[n.ID]
		if old == "" || old == key {
			continue
		}
		for i := range st.Conflicts {
			x := &st.Conflicts[i]
			if x.Path != old || (!x.RemoteDeleted && x.RemotePath != key) {
				continue
			}
			x.RemotePath = key
			x.RemoteHash = digest(b)
			x.RemoteDeleted = false
			if e := atomicWrite(s.syncPath(w, "conflicts/"+x.ID+"-remote"), b); e != nil {
				return e
			}
			delete(downloads, key)
			if previous, ok := baseline[key]; ok {
				next[key] = previous
			} else {
				delete(next, key)
			}
			break
		}
	}
	return nil
}
