package wiki

import (
	"strings"
	"testing"
)

func TestRenderPrompt(t *testing.T) {
	prompt, err := renderPrompt("classify-type.txt", &promptData{
		Title:   "A title",
		URL:     "https://example.com/post",
		Content: "A summary",
	})
	if err != nil {
		t.Fatalf("renderPrompt() error = %v", err)
	}

	for _, want := range []string{"A title", "https://example.com/post", "A summary"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("rendered prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "{{") {
		t.Fatalf("rendered prompt still contains template marker:\n%s", prompt)
	}
}
