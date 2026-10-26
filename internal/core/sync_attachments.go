package core

import (
	"bytes"
	"encoding/json"
	"sort"
)

// Concurrent imports merge by ID. A deletion tombstone wins over a stale live
// record, so removed attachments cannot return during concurrent synchronization.
func mergeAttachmentManifests(local, remote []byte) ([]byte, bool) {
	var a, b []Attachment
	if json.Unmarshal(local, &a) != nil || json.Unmarshal(remote, &b) != nil {
		return nil, false
	}
	items := map[string]Attachment{}
	for _, set := range [][]Attachment{a, b} {
		for _, item := range set {
			if item.ID == "" {
				return nil, false
			}
			if previous, ok := items[item.ID]; ok {
				if previous.Deleted || item.Deleted {
					if previous.SHA256 != item.SHA256 {
						return nil, false
					}
					if previous.Deleted {
						item = previous
					}
					items[item.ID] = item
					continue
				}
				x, _ := json.Marshal(previous)
				y, _ := json.Marshal(item)
				if !bytes.Equal(x, y) {
					return nil, false
				}
			}
			items[item.ID] = item
		}
	}
	ordered := make([]Attachment, 0, len(items))
	for _, item := range items {
		ordered = append(ordered, item)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	out, e := json.MarshalIndent(ordered, "", "  ")
	return out, e == nil
}
