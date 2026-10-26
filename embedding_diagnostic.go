package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"smartnote/internal/embedding"
	"time"
)

// Headless package diagnostic used to verify the installed, embedded resources
// without opening a workspace or reading a user's profile.
func embeddingDiagnostic(root string) error {
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	engine := embedding.New(filepath.Join(root, "models"))
	defer engine.Close()
	start := time.Now()
	vectors, err := engine.Embed([]string{"数据库慢查询分析和索引优化", "SQL查询性能与数据库索引调优", "海边旅行住宿酒店与景点攻略"})
	result := map[string]any{"ok": false, "model": embedding.ModelName, "dimensions": embedding.Dimensions}
	if err == nil {
		dot := func(a, b []float64) float64 {
			var sum float64
			for i := range a {
				sum += a[i] * b[i]
			}
			return sum
		}
		related, unrelated := dot(vectors[0], vectors[1]), dot(vectors[0], vectors[2])
		var norm float64
		for _, x := range vectors[0] {
			norm += x * x
		}
		result["relatedSimilarity"] = related
		result["unrelatedSimilarity"] = unrelated
		result["elapsedMs"] = time.Since(start).Milliseconds()
		if related <= unrelated+0.1 || math.Abs(norm-1) > 1e-6 {
			err = fmt.Errorf("模型自检未通过语义相似度校验")
		}
	}
	if err != nil {
		result["error"] = err.Error()
	} else {
		result["ok"] = true
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	if writeErr := os.WriteFile(filepath.Join(root, "embedding-self-test.json"), b, 0600); writeErr != nil {
		return writeErr
	}
	return err
}
