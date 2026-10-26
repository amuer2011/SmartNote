package core

func (s *Service) indexNotes(workspace string, notes []Note) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Older versions have an FTS table without fingerprints; rebuild once on upgrade.
	var count int
	if err = tx.QueryRow("SELECT count(*) FROM note_fingerprints WHERE workspace=?", workspace).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err = tx.Exec("DELETE FROM notes_fts WHERE workspace=?", workspace); err != nil {
			return err
		}
	}
	rows, err := tx.Query("SELECT id,hash FROM note_fingerprints WHERE workspace=?", workspace)
	if err != nil {
		return err
	}
	existing := map[string]string{}
	for rows.Next() {
		var id, hash string
		if err = rows.Scan(&id, &hash); err != nil {
			rows.Close()
			return err
		}
		existing[id] = hash
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, n := range notes {
		fingerprint := digest([]byte(n.Hash + "\x00" + n.Path))
		if existing[n.ID] == fingerprint {
			delete(existing, n.ID)
			continue
		}
		if _, err = tx.Exec("DELETE FROM notes_fts WHERE workspace=? AND id=?", workspace, n.ID); err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT INTO notes_fts(workspace,id,title,content) VALUES(?,?,?,?)", workspace, n.ID, n.Title, n.Content); err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT INTO note_fingerprints(workspace,id,hash) VALUES(?,?,?) ON CONFLICT(workspace,id) DO UPDATE SET hash=excluded.hash", workspace, n.ID, fingerprint); err != nil {
			return err
		}
		delete(existing, n.ID)
	}
	for id := range existing {
		if _, err = tx.Exec("DELETE FROM notes_fts WHERE workspace=? AND id=?", workspace, id); err != nil {
			return err
		}
		if _, err = tx.Exec("DELETE FROM note_fingerprints WHERE workspace=? AND id=?", workspace, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
