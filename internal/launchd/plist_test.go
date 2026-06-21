package launchd

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestGeneratePlist(t *testing.T) {
	cfg := PlistConfig{
		Label:      "com.styx.nomad",
		Program:    "/bin/bash",
		Args:       []string{"/home/user/.styx/config/styx-agent.sh", "--flag"},
		LogPath:    "/logs/styx.log",
		ErrLogPath: "/logs/styx-error.log",
		WorkingDir: "/home/user/.styx/config",
	}

	out, err := GeneratePlist(cfg)
	if err != nil {
		t.Fatalf("GeneratePlist returned error: %v", err)
	}
	s := string(out)

	mustContain := []string{
		"<string>com.styx.nomad</string>",
		"<string>/bin/bash</string>",
		"<string>/home/user/.styx/config/styx-agent.sh</string>",
		"<string>--flag</string>",
		"<string>/logs/styx.log</string>",
		"<string>/logs/styx-error.log</string>",
		"<string>/home/user/.styx/config</string>",
		"<key>RunAtLoad</key>",
		"<key>KeepAlive</key>",
	}
	for _, frag := range mustContain {
		if !strings.Contains(s, frag) {
			t.Errorf("plist missing %q\n---\n%s", frag, s)
		}
	}

	// The output must be well-formed XML.
	if err := xml.Unmarshal(out, new(struct {
		XMLName xml.Name `xml:"plist"`
	})); err != nil {
		t.Errorf("generated plist is not valid XML: %v\n%s", err, s)
	}
}

// TestGeneratePlistNoArgs ensures the Args range renders cleanly with no args.
func TestGeneratePlistNoArgs(t *testing.T) {
	out, err := GeneratePlist(PlistConfig{
		Label:   "com.styx.test",
		Program: "/usr/bin/true",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<string>/usr/bin/true</string>") {
		t.Errorf("program not rendered:\n%s", s)
	}
	// Exactly one <string> inside ProgramArguments (the program itself).
	argsSection := s[strings.Index(s, "<array>"):strings.Index(s, "</array>")]
	if n := strings.Count(argsSection, "<string>"); n != 1 {
		t.Errorf("expected 1 ProgramArguments string, got %d", n)
	}
}
