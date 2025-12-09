package launchd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// PlistConfig holds the configuration for a launchd plist file.
type PlistConfig struct {
	Label      string   // Unique identifier, e.g., "com.styx.nomad"
	Program    string   // Path to executable (discovered via PATH lookup)
	Args       []string // Arguments to pass to program
	LogPath    string   // Path for stdout logs
	ErrLogPath string   // Path for stderr logs
	WorkingDir string   // Working directory for the process
}

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>

    <key>ProgramArguments</key>
    <array>
        <string>{{.Program}}</string>
{{- range .Args}}
        <string>{{.}}</string>
{{- end}}
    </array>

    <key>KeepAlive</key>
    <true/>

    <key>RunAtLoad</key>
    <true/>

    <key>StandardOutPath</key>
    <string>{{.LogPath}}</string>

    <key>StandardErrorPath</key>
    <string>{{.ErrLogPath}}</string>

    <key>WorkingDirectory</key>
    <string>{{.WorkingDir}}</string>
</dict>
</plist>
`

// GeneratePlist renders the launchd plist XML with the given config.
func GeneratePlist(cfg PlistConfig) ([]byte, error) {
	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plist template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return nil, fmt.Errorf("failed to render plist: %w", err)
	}

	return buf.Bytes(), nil
}

// WritePlist writes a launchd plist file to the specified path.
// For user agents: ~/Library/LaunchAgents/com.styx.nomad.plist
func WritePlist(path string, cfg PlistConfig) error {
	content, err := GeneratePlist(cfg)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create plist directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to write plist to %s: %w", path, err)
	}

	return nil
}
