//go:build windows && amd64 && cgo

package embedding

import (
	"fmt"
	"math"

	"path/filepath"
	"sync"
	"time"

	ort "github.com/yalue/onnxruntime_go"
	"golang.org/x/sys/windows"
)

type Status struct {
	State      string `json:"state"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
	Message    string `json:"message"`
	Cached     int    `json:"cached"`
}

type Engine struct {
	mu          sync.Mutex
	stateMu     sync.Mutex
	root        string
	state       Status
	session     *ort.DynamicAdvancedSession
	tokenizer   *Tokenizer
	cache       map[string][]float64
	closed      bool
	runtimeHeld bool
}

func New(root string) *Engine {
	return &Engine{root: root, state: Status{State: "idle", Model: ModelName, Dimensions: Dimensions, Message: "内置离线模型，首次使用时自动加载"}, cache: map[string][]float64{}}
}
func (e *Engine) Status() Status { e.stateMu.Lock(); defer e.stateMu.Unlock(); return e.state }
func (e *Engine) setState(state, message string) {
	e.stateMu.Lock()
	defer e.stateMu.Unlock()
	e.state.State = state
	e.state.Message = message
	e.state.Cached = len(e.cache)
}

var runtimeMu sync.Mutex
var runtimeReady bool
var runtimeUsers int
var runtimeHandles []windows.Handle

func initializeRuntime(dir string) error {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if runtimeReady {
		runtimeUsers++
		return nil
	}
	// Load the bundled C++ runtime explicitly, with dependency lookup restricted to
	// this verified resource directory and Windows system libraries.
	handles := []windows.Handle{}
	for _, name := range []string{"vcruntime140.dll", "vcruntime140_1.dll", "msvcp140.dll", "msvcp140_1.dll", "onnxruntime.dll"} {
		h, err := windows.LoadLibraryEx(filepath.Join(dir, name), 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
		if err != nil {
			for i := len(handles) - 1; i >= 0; i-- {
				windows.FreeLibrary(handles[i])
			}
			return fmt.Errorf("加载内置推理组件 %s：%w", name, err)
		}
		handles = append(handles, h)
	}
	ort.SetSharedLibraryPath(filepath.Join(dir, "onnxruntime.dll"))
	if err := ort.InitializeEnvironment(); err != nil {
		for i := len(handles) - 1; i >= 0; i-- {
			windows.FreeLibrary(handles[i])
		}
		return err
	}
	// Keep the embedded runtime's optional telemetry disabled in local mode.
	if err := ort.DisableTelemetry(); err != nil {
		_ = ort.DestroyEnvironment()
		for i := len(handles) - 1; i >= 0; i-- {
			windows.FreeLibrary(handles[i])
		}
		return err
	}
	runtimeHandles = handles
	runtimeReady = true
	runtimeUsers = 1
	return nil
}
func releaseRuntime() error {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if !runtimeReady {
		return nil
	}
	runtimeUsers--
	if runtimeUsers > 0 {
		return nil
	}
	if err := ort.DestroyEnvironment(); err != nil {
		return err
	}
	for i := len(runtimeHandles) - 1; i >= 0; i-- {
		windows.FreeLibrary(runtimeHandles[i])
	}
	runtimeHandles = nil
	runtimeReady = false
	return nil
}
func (e *Engine) load() error {
	if e.closed {
		return fmt.Errorf("模型已关闭")
	}
	if e.session != nil {
		return nil
	}
	e.setState("loading", "正在准备内置离线模型…")
	dir, err := unpack(e.root)
	if err != nil {
		return err
	}
	if err = initializeRuntime(dir); err != nil {
		return err
	}
	e.runtimeHeld = true
	defer func() {
		if e.session == nil && e.runtimeHeld {
			_ = releaseRuntime()
			e.runtimeHeld = false
		}
	}()
	vocab, err := resources.ReadFile("assets/vocab.txt")
	if err != nil {
		return err
	}
	e.tokenizer, err = NewTokenizer(vocab)
	if err != nil {
		return err
	}
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return err
	}
	defer opts.Destroy()
	if err = opts.SetIntraOpNumThreads(2); err != nil {
		return err
	}
	if err = opts.SetInterOpNumThreads(1); err != nil {
		return err
	}
	if err = opts.SetGraphOptimizationLevel(ort.GraphOptimizationLevelEnableAll); err != nil {
		return err
	}
	e.session, err = ort.NewDynamicAdvancedSession(filepath.Join(dir, "model.onnx"), []string{"input_ids", "attention_mask", "token_type_ids"}, []string{"last_hidden_state"}, opts)
	if err != nil {
		return err
	}
	e.setState("ready", "内置中文模型已就绪 · 全程离线")
	return nil
}
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closed = true
	e.cache = nil
	if e.session != nil {
		if err := e.session.Destroy(); err != nil {
			return err
		}
		e.session = nil
	}
	if e.runtimeHeld {
		e.runtimeHeld = false
		return releaseRuntime()
	}
	return nil
}
func (e *Engine) Embed(texts []string) ([][]float64, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.load(); err != nil {
		e.setState("error", "内置模型加载失败，可重试；笔记已保留："+err.Error())
		return nil, err
	}
	out := make([][]float64, len(texts))
	start := time.Now()
	for i, text := range texts {
		// Token IDs are the key: equivalent normalized input reuses the same vector.
		ids := e.tokenizer.Encode(text, 8192)
		key := fmt.Sprint(ids)
		if v, ok := e.cache[key]; ok {
			out[i] = append([]float64(nil), v...)
			continue
		}
		if time.Since(start) > 60*time.Second {
			return nil, fmt.Errorf("模型分析超时，请减少启用的分类后重试")
		}
		v, err := e.vectorize(ids)
		if err != nil {
			e.setState("error", "模型分析失败，笔记保留原位："+err.Error())
			return nil, err
		}
		if len(e.cache) >= 256 {
			e.cache = map[string][]float64{}
		}
		e.cache[key] = v
		out[i] = append([]float64(nil), v...)
	}
	e.setState("ready", "内置中文模型已就绪 · 全程离线")
	return out, nil
}

// Longer text is split into overlapping BERT windows. Each window uses CLS
// pooling; the weighted average is normalized again for cosine comparisons.
func (e *Engine) vectorize(ids []int64) ([]float64, error) {
	if len(ids) <= 512 {
		return e.run(ids)
	}
	tokens := ids[1 : len(ids)-1]
	sum := make([]float64, Dimensions)
	for start := 0; start < len(tokens); start += 446 {
		end := start + 510
		if end > len(tokens) {
			end = len(tokens)
		}
		chunk := make([]int64, 0, end-start+2)
		chunk = append(chunk, e.tokenizer.clsID)
		chunk = append(chunk, tokens[start:end]...)
		chunk = append(chunk, e.tokenizer.sepID)
		v, err := e.run(chunk)
		if err != nil {
			return nil, err
		}
		weight := end - start
		if start > 0 {
			weight -= 64
		}
		if weight < 1 {
			weight = 1
		}
		for i, x := range v {
			sum[i] += x * float64(weight)
		}
		if end == len(tokens) {
			break
		}
	}
	var norm float64
	for _, x := range sum {
		norm += x * x
	}
	if norm == 0 {
		return nil, fmt.Errorf("长文向量为空")
	}
	norm = math.Sqrt(norm)
	for i := range sum {
		sum[i] /= norm
	}
	return sum, nil
}
func (e *Engine) run(ids []int64) ([]float64, error) {
	shape := ort.NewShape(1, int64(len(ids)))
	input, err := ort.NewTensor(shape, ids)
	if err != nil {
		return nil, err
	}
	defer input.Destroy()
	maskData := make([]int64, len(ids))
	for i := range maskData {
		maskData[i] = 1
	}
	mask, err := ort.NewTensor(shape, maskData)
	if err != nil {
		return nil, err
	}
	defer mask.Destroy()
	types, err := ort.NewTensor(shape, make([]int64, len(ids)))
	if err != nil {
		return nil, err
	}
	defer types.Destroy()
	outputs := []ort.Value{nil}
	err = e.session.Run([]ort.Value{input, mask, types}, outputs)
	defer func() {
		for _, v := range outputs {
			if v != nil {
				v.Destroy()
			}
		}
	}()
	if err != nil {
		return nil, err
	}
	tensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("模型输出类型错误")
	}
	dims := tensor.GetShape()
	if len(dims) != 3 || dims[0] != 1 || dims[1] != int64(len(ids)) || dims[2] != Dimensions {
		return nil, fmt.Errorf("模型输出形状错误：%v", dims)
	}
	// BGE v1.5 uses the CLS embedding, followed by L2 normalization.
	values := tensor.GetData()
	out := make([]float64, Dimensions)
	var length float64
	for i := range out {
		out[i] = float64(values[i])
		length += out[i] * out[i]
	}
	if length == 0 || math.IsNaN(length) || math.IsInf(length, 0) {
		return nil, fmt.Errorf("模型返回无效向量")
	}
	length = math.Sqrt(length)
	for i := range out {
		out[i] /= length
	}
	return out, nil
}
