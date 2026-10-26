//go:build !windows || !amd64 || !cgo

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
	return Status{State: "unsupported", Model: ModelName, Dimensions: Dimensions, Message: "内置模型当前支持 Windows x64"}
}
func (*Engine) Embed([]string) ([][]float64, error) {
	return nil, fmt.Errorf("内置模型当前支持 Windows x64，构建时需要启用 CGO")
}
func (*Engine) Close() error { return nil }
