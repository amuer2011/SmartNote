//go:build !cgo || (!windows && !darwin && !linux) || (windows && !amd64) || ((darwin || linux) && !amd64 && !arm64)

package embedding

import "fmt"

type Status struct {
	State      string `json:"state"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
	Message    string `json:"message"`
	Cached     int    `json:"cached"`
}
type Engine struct{}

func New(string) *Engine { return &Engine{} }
func (*Engine) Status() Status {
	return Status{State: "unsupported", Model: ModelName, Dimensions: Dimensions, Message: "当前平台不支持内置模型，或构建未启用 CGO"}
}
func (*Engine) Embed([]string) ([][]float64, error) {
	return nil, fmt.Errorf("内置模型支持 Windows x64、macOS 和 Linux x64/ARM64，构建时需要启用 CGO")
}
func (*Engine) Close() error { return nil }
