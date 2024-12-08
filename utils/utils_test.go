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
