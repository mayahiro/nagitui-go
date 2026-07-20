//go:build (linux || darwin) && !(amd64 || arm64)

package ttyunix

// Nagi TUI's direct terminal ABI is intentionally limited to the layouts that
// are verified in CI and the platform SDKs.
var unsupportedArchitecture [0 - 1]int
