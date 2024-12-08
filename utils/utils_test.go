package utils

import "testing"

func TestRenderMarkdownImageWithFigcaption(t *testing.T) {
	type args struct {
		url string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"Test RenderMarkdownImageWithFigcaption", args{url: "https://example.com/image.jpg"}, "![image](https://example.com/image.jpg)\n<center>*image.jpg*</center>\n\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RenderMarkdownImageWithFigcaption(tt.args.url); got != tt.want {
				t.Errorf("RenderMarkdownImageWithFigcaption() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderMarkdownAdmonitions(t *testing.T) {
	type args struct {
		admonitionType string
		title          string
		rex            string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// Test all admonition types
		{"Test RenderMarkdownAdmonitions", args{admonitionType: "note", title: "Note", rex: "This is a note"}, "\n---\n:::note[Note]\n\nThis is a note\n\n:::\n\n"},
		{"Test RenderMarkdownAdmonitions", args{admonitionType: "warning", title: "Warning", rex: "This is a warning"}, "\n---\n:::warning[Warning]\n\nThis is a warning\n\n:::\n\n"},
		{"Test RenderMarkdownAdmonitions", args{admonitionType: "danger", title: "Danger", rex: "This is a danger"}, "\n---\n:::danger[Danger]\n\nThis is a danger\n\n:::\n\n"},
		{"Test RenderMarkdownAdmonitions", args{admonitionType: "info", title: "Info", rex: "This is a info"}, "\n---\n:::info[Info]\n\nThis is a info\n\n:::\n\n"},
		{"Test RenderMarkdownAdmonitions", args{admonitionType: "tip", title: "Tip", rex: "This is a tip"}, "\n---\n:::tip[Tip]\n\nThis is a tip\n\n:::\n\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RenderMarkdownAdmonitions(tt.args.admonitionType, tt.args.title, tt.args.rex); got != tt.want {
				t.Errorf("RenderMarkdownAdmonitions() = %v, want %v", got, tt.want)
			}
		})
	}
}
