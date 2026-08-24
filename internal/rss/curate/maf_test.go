package curate

import "testing"

func TestJSONBytes(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`{"a":1}`, `{"a":1}`},
		{"```json\n{\"a\":1}\n```", `{"a":1}`},
		{"Sure, here:\n```\n{\"a\":1}\n```\nDone.", `{"a":1}`},
		{"prefix {  \"a\" :2 } suffix", `{  "a" :2 }`},
	}
	for _, c := range cases {
		b, err := jsonBytes(c.in)
		if err != nil {
			t.Fatalf("jsonBytes(%q) err=%v", c.in, err)
		}
		if string(b) != c.want {
			t.Errorf("jsonBytes(%q)=%s want %s", c.in, b, c.want)
		}
	}
	if _, err := jsonBytes("no json here"); err == nil {
		t.Fatal("expected error for non-JSON")
	}
}
