package configutil

import "testing"

func TestLoadYAMLBytes(t *testing.T) {
	k, err := LoadYAMLBytes([]byte("server:\n  port: 8080\n"))
	if err != nil {
		t.Fatalf("LoadYAMLBytes() error = %v", err)
	}

	if got := k.Int("server.port"); got != 8080 {
		t.Fatalf("server.port = %d, want 8080", got)
	}
}

func TestLoadYAMLBytesRejectsInvalidYAML(t *testing.T) {
	_, err := LoadYAMLBytes([]byte("server: ["))
	if err == nil {
		t.Fatal("LoadYAMLBytes() error = nil, want parse error")
	}
}
