package build

var tools = map[string]tool{
	"go": {
		Version: "1.18.1",
		URL:     "https://go.dev/dl/go1.18.1.darwin-amd64.tar.gz",
		Hash:    "sha256:63e5035312a9906c98032d9c73d036b6ce54f8632b194228bd08fe3b9fe4ab01",
		Binaries: []string{
			"go/bin/go",
			"go/bin/gofmt",
		},
	},
	"golangci": {
		Version: "1.45.2",
		URL:     "https://github.com/golangci/golangci-lint/releases/download/v1.45.2/golangci-lint-1.45.2-darwin-amd64.tar.gz",
		Hash:    "sha256:995e509e895ca6a64ffc7395ac884d5961bdec98423cb896b17f345a9b4a19cf",
		Binaries: []string{
			"golangci-lint-1.45.2-darwin-amd64/golangci-lint",
		},
	},
	"ignite": {
		Version: "v0.20.4",
		URL:     "https://github.com/ignite-hq/cli/releases/download/v0.20.4/ignite_0.20.4_darwin_amd64.tar.gz",
		Hash:    "sha256:2e9366168de8b8dbf743ec0de21c93430eca79c76d947c6de4d7c728c757f05e",
		Binaries: []string{
			"ignite",
		},
	},
}
