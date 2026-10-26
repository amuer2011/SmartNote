package embedding

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
)

func TestNativeRuntimeManifestAndIntegrity(t *testing.T) {
	original := platformResources
	defer func() { platformResources = original }()
	data := []byte("verified native library")
	digest := sha256.Sum256(data)
	native := nativeManifest{Platform: runtime.GOOS + "-" + runtime.GOARCH, Runtime: "test CPU runtime", Files: map[string]Resource{
		"native-library.bin": {SHA256: hex.EncodeToString(digest[:]), Size: int64(len(data))},
	}}
	configure := func(contents []byte) {
		raw, _ := json.Marshal(native)
		platformResources = fstest.MapFS{"runtime.json": &fstest.MapFile{Data: raw}, "native-library.bin": &fstest.MapFile{Data: contents}}
	}
	configure(data)
	manifest, _, err := resourceManifest()
	if err != nil {
		t.Fatal(err)
	}
	for name := range manifest.Files {
		if strings.HasSuffix(name, ".dll") {
			t.Fatal("Windows DLL leaked into native manifest:", name)
		}
	}
	if _, err = unpack(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	configure([]byte("altered runtime"))
	if _, err = unpack(t.TempDir()); err == nil {
		t.Fatal("corrupt runtime passed resource verification")
	}
	native.Platform = "unsupported-target"
	configure(data)
	if _, _, err = resourceManifest(); err == nil {
		t.Fatal("wrong architecture was accepted")
	}
	native.Platform = runtime.GOOS + "-" + runtime.GOARCH
	native.Files["../outside.bin"] = native.Files["native-library.bin"]
	configure(data)
	if _, _, err = resourceManifest(); err == nil {
		t.Fatal("unsafe resource name was accepted")
	}
}
