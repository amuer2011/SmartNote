package core

import (
	"context"
	"encoding/json"
	"fmt"
	"smartnote/internal/embedding"
	"time"
)

// Each entry is disposable, scoped to its workspace and keyed by the complete
// passage text (including title, tags and path) and bundled model resources.
func (s *Service) passageVectors(workspace string, notes []Note, passages []searchPassage, dimensions int) ([][]float64, error) {
	return s.passageVectorsWithOptions(workspace, notes, passages, dimensions, vectorOptions{ctx: context.Background()})
}

type vectorOptions struct {
	ctx        context.Context
	progress   func(int, int)
	rebuild    bool
	background bool
}

func (s *Service) passageVectorsWithOptions(workspace string, notes []Note, passages []searchPassage, dimensions int, opt vectorOptions) ([][]float64, error) {
	if err := opt.ctx.Err(); err != nil {
		return nil, err
	}
	model := embedding.Fingerprint()
	vectors := make([][]float64, len(passages))
	missing := []int{}
	wanted := map[string][]int{}
	for i, p := range passages {
		key := notes[p.note].ID + ":" + digest([]byte(p.text))
		wanted[key] = append(wanted[key], i)
	}
	rows, readErr := s.db.Query("SELECT note,model,fingerprint,vector FROM passage_vectors WHERE workspace=?", workspace)
	type staleKey struct{ note, model, fingerprint string }
	stale := []staleKey{}
	if readErr == nil {
		for rows.Next() {
			var note, storedModel, fingerprint string
			var raw []byte
			if err := rows.Scan(&note, &storedModel, &fingerprint, &raw); err != nil {
				continue
			}
			indices, ok := wanted[note+":"+fingerprint]
			if !ok || storedModel != model {
				stale = append(stale, staleKey{note, storedModel, fingerprint})
				continue
			}
			var v []float64
			if !opt.rebuild && json.Unmarshal(raw, &v) == nil && len(v) == dimensions {
				if _, err := cosine(v, v); err == nil {
					for _, i := range indices {
						vectors[i] = v
					}
					continue
				}
			}
			if !opt.rebuild {
				stale = append(stale, staleKey{note, storedModel, fingerprint})
			}
		}
		_ = rows.Close()
	}
	// Delete only keys seen in this snapshot, never another search's new entries.
	if len(stale) > 0 {
		if tx, err := s.db.Begin(); err == nil {
			valid := true
			for _, key := range stale {
				if _, err = tx.Exec("DELETE FROM passage_vectors WHERE workspace=? AND note=? AND model=? AND fingerprint=?", workspace, key.note, key.model, key.fingerprint); err != nil {
					valid = false
					break
				}
			}
			if valid {
				_ = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
		}
	}
	for i := range passages {
		if vectors[i] == nil {
			missing = append(missing, i)
		}
	}
	completed := len(passages) - len(missing)
	if opt.progress != nil {
		opt.progress(completed, len(passages))
	}
	batchSize := 16
	if opt.background {
		batchSize = 4
	}
	for start := 0; start < len(missing); start += batchSize {
		if err := opt.ctx.Err(); err != nil {
			return nil, err
		}
		end := start + batchSize
		if end > len(missing) {
			end = len(missing)
		}
		texts := make([]string, end-start)
		for j, i := range missing[start:end] {
			texts[j] = passages[i].text
		}
		batch, err := s.embedTexts(texts)
		if err != nil {
			return nil, err
		}
		if err := opt.ctx.Err(); err != nil {
			return nil, err
		}
		if len(batch) != len(texts) {
			return nil, fmt.Errorf("模型返回段落向量数量错误")
		}
		for j, v := range batch {
			if len(v) != dimensions {
				return nil, fmt.Errorf("模型返回段落向量维度错误")
			}
			if _, err = cosine(v, v); err != nil {
				return nil, err
			}
			i := missing[start+j]
			vectors[i] = v
		}
		// Write only validated batches; database errors leave the search usable and
		// simply cause recomputation next time. No network or source file is changed.
		tx, err := s.db.Begin()
		if err != nil {
			if opt.background {
				return nil, err
			}
			continue
		}
		valid := true
		for j, v := range batch {
			i := missing[start+j]
			p := passages[i]
			raw, _ := json.Marshal(v)
			if _, err = tx.Exec("INSERT INTO passage_vectors(workspace,note,model,fingerprint,vector) VALUES(?,?,?,?,?) ON CONFLICT(workspace,note,model,fingerprint) DO UPDATE SET vector=excluded.vector", workspace, notes[p.note].ID, model, digest([]byte(p.text)), raw); err != nil {
				valid = false
				break
			}
		}
		if valid {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		if err != nil && opt.background {
			return nil, err
		}
		completed += len(batch)
		if opt.progress != nil {
			opt.progress(completed, len(passages))
		}
		if opt.background {
			select {
			case <-opt.ctx.Done():
				return nil, opt.ctx.Err()
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
	return vectors, nil
}
