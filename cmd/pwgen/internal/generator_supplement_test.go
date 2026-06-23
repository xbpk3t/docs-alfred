package pwgen

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeHmacSha512(t *testing.T) {
	result1 := computeHmacSha512("message", "secret")
	assert.NotEmpty(t, result1, "should return non-empty base64 string")

	// Deterministic
	result2 := computeHmacSha512("message", "secret")
	assert.Equal(t, result1, result2, "same inputs should produce same output")

	// Different inputs produce different output
	result3 := computeHmacSha512("other", "secret")
	assert.NotEqual(t, result1, result3)
}

func TestComputeHmacSha512EmptyInputs(t *testing.T) {
	result := computeHmacSha512("", "")
	assert.NotEmpty(t, result)
}

func TestSha512Method(t *testing.T) {
	g := NewGenerator(NewConfig("secret", 16, true, true, false))

	hash1 := g.sha512("mykey", "example.com")
	assert.NotEmpty(t, hash1)

	// Deterministic
	hash2 := g.sha512("mykey", "example.com")
	assert.Equal(t, hash1, hash2)

	// Different website produces different hash
	hash3 := g.sha512("mykey", "other.com")
	assert.NotEqual(t, hash1, hash3)
}

func TestSha512MethodDifferentKeys(t *testing.T) {
	g := NewGenerator(NewConfig("secret", 16, true, true, false))

	hash1 := g.sha512("key1", "example.com")
	hash2 := g.sha512("key2", "example.com")
	assert.NotEqual(t, hash1, hash2, "different keys should produce different hashes")
}

func TestGeneratePwdAllFlagsTrue(t *testing.T) {
	g := NewGenerator(NewConfig("test", 16, true, true, true))
	hash := g.sha512("test", "example.com")

	pwd := g.generatePwd(hash, 16, true, true)
	assert.NotEmpty(t, pwd)
	assert.Equal(t, 16, len(pwd))
}

func TestGeneratePwdAllFlagsFalse(t *testing.T) {
	g := NewGenerator(NewConfig("test", 16, false, false, false))
	hash := g.sha512("test", "example.com")

	pwd := g.generatePwd(hash, 16, false, false)
	assert.NotEmpty(t, pwd)
	assert.Equal(t, 16, len(pwd))
}

func TestGeneratePwdMixedFlags(t *testing.T) {
	g := NewGenerator(NewConfig("test", 16, true, false, false))
	hash := g.sha512("test", "example.com")

	// uppercase=true, punctuation=false
	pwd := g.generatePwd(hash, 16, false, true)
	assert.NotEmpty(t, pwd)
	assert.Equal(t, 16, len(pwd))
}

func TestGeneratePwdPunctuationTrueUppercaseFalse(t *testing.T) {
	g := NewGenerator(NewConfig("test", 16, false, true, true))
	hash := g.sha512("test", "example.com")

	// isPunc=true, isUseUpper=false
	pwd := g.generatePwd(hash, 16, true, false)
	assert.NotEmpty(t, pwd)
	assert.Equal(t, 16, len(pwd))
}

func TestGeneratePwdShortLength(t *testing.T) {
	g := NewGenerator(NewConfig("test", 8, true, true, false))
	hash := g.sha512("test", "example.com")

	pwd := g.generatePwd(hash, 8, false, true)
	assert.NotEmpty(t, pwd)
	assert.Equal(t, 8, len(pwd))
}

func TestGeneratePwdLongLength(t *testing.T) {
	g := NewGenerator(NewConfig("test", 64, true, true, false))
	hash := g.sha512("test", "example.com")

	pwd := g.generatePwd(hash, 64, false, true)
	// May be empty if JS algorithm can't satisfy all constraints at this length
	// But should not panic
	_ = pwd
}

func TestGeneratePunctuationTrueUppercaseFalse(t *testing.T) {
	config := NewConfig("testsecret", 16, false, true, true)
	generator := NewGenerator(config)

	result, err := generator.Generate("github.com")
	require.NoError(t, err)
	assert.Equal(t, 16, len(result))

	// Should not contain uppercase
	for _, c := range result {
		assert.False(t, c >= 'A' && c <= 'Z', "password should not contain uppercase when IsUppercase=false")
	}
}

func TestNewConfigConstructor(t *testing.T) {
	cfg := NewConfig("mysecret", 24, true, false, true)

	assert.Equal(t, "mysecret", cfg.SecretKey)
	assert.Equal(t, 24, cfg.Length)
	assert.True(t, cfg.IsUppercase)
	assert.False(t, cfg.IsNum)
	assert.True(t, cfg.IsPunctuation)
}

func TestNewGeneratorConstructor(t *testing.T) {
	cfg := NewConfig("secret", 16, true, true, false)
	gen := NewGenerator(cfg)

	require.NotNil(t, gen)
	assert.Equal(t, cfg, gen.config)
}

func TestGenerateWithNumbersDisabled(t *testing.T) {
	// With numbers=false, only letters (and optionally punctuation) are in the alphabet
	config := NewConfig("testsecret", 16, true, false, false)
	generator := NewGenerator(config)

	result, err := generator.Generate("example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestGenerateDifferentSecrets(t *testing.T) {
	gen1 := NewGenerator(NewConfig("secret1", 16, true, true, false))
	gen2 := NewGenerator(NewConfig("secret2", 16, true, true, false))

	r1, err := gen1.Generate("example.com")
	require.NoError(t, err)
	r2, err := gen2.Generate("example.com")
	require.NoError(t, err)

	assert.NotEqual(t, r1, r2, "different secrets should produce different passwords")
}

// TestSha512MethodVariousInputs exercises the sha512 loop body with many
// key/website combinations to maximise coverage of the ParseFloat-continue
// and string-processing paths.
func TestSha512MethodVariousInputs(t *testing.T) {
	g := NewGenerator(NewConfig("test", 16, true, true, false))

	keys := []string{
		"alpha", "bravo", "charlie", "delta", "echo",
		"foxtrot", "golf", "hotel", "india", "juliet",
	}
	sites := []string{
		"github.com", "google.com", "example.org", "test.io",
		"mail.google.com", "sub.domain.example.co.uk",
		"a", "bb", "xyz123", "longdomainnamethatproducesdifferenthash",
	}

	for _, k := range keys {
		for _, s := range sites {
			hash := g.sha512(k, s)
			assert.NotEmpty(t, hash, "key=%s site=%s", k, s)
		}
	}
}

// TestSha512ProducesConsistentOutput adds additional varied inputs to the
// sha512 loop to ensure diverse base64 characters are iterated over.
func TestSha512ProducesConsistentOutput(t *testing.T) {
	g := NewGenerator(NewConfig("test", 16, true, true, false))

	// Special characters in website names
	sites := []string{
		"https://example.com/path?q=1&r=2",
		"user@domain.com",
		"192.168.1.1",
		"port:8080",
		"with spaces",
		"unicode-日本語-test",
		"UPPERCASE.COM",
		"mIxEd.CaSe.OrG",
	}

	for _, site := range sites {
		h1 := g.sha512("key", site)
		h2 := g.sha512("key", site)
		assert.Equal(t, h1, h2, "deterministic for site=%s", site)
		assert.NotEmpty(t, h1)
	}
}

// TestGeneratePwdHashTooShort verifies that generatePwd returns an empty
// string when the hash is shorter than the requested length.  The JS
// seekPassword loop `for (var i = 0; i <= hash.length - length; ++i)`
// never executes, so it returns "".
func TestGeneratePwdHashTooShort(t *testing.T) {
	g := NewGenerator(NewConfig("test", 16, true, true, false))

	// "abc" is only 3 chars, much shorter than length=16
	pwd := g.generatePwd("abc", 16, false, true)
	assert.Equal(t, "", pwd, "should return empty when hash shorter than length")
}

// TestGenerateNoUppercaseNoPunctuation exercises the combination where
// both uppercase and punctuation are disabled, covering the -1 flag paths
// in the JS function.
func TestGenerateNoUppercaseNoPunctuation(t *testing.T) {
	config := NewConfig("testsecret", 16, false, true, false)
	g := NewGenerator(config)

	result, err := g.Generate("github.com")
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Equal(t, 16, len(result))

	// Should not contain uppercase letters
	for _, c := range result {
		assert.False(t, c >= 'A' && c <= 'Z',
			"password should not contain uppercase when IsUppercase=false, got %q", c)
	}
}

// TestGenerateVeryShortPassword uses length=4, the practical minimum, to
// exercise the JS algorithm at very short lengths.
func TestGenerateVeryShortPassword(t *testing.T) {
	g := NewGenerator(NewConfig("test", 4, true, true, false))

	result, err := g.Generate("github.com")
	require.NoError(t, err)
	assert.NotEmpty(t, result, "should produce a password at length=4")
	assert.Equal(t, 4, len(result))
}

// TestSha512RuleLengthMismatch covers the "i >= len(rule)" branch (line 112)
// by mocking computeHmacFunc to return source and rule strings of different
// lengths.
func TestSha512RuleLengthMismatch(t *testing.T) {
	g := NewGenerator(NewConfig("test", 16, true, true, false))

	oldHmac := computeHmacFunc
	callCount := 0
	computeHmacFunc = func(message, secret string) string {
		callCount++
		switch callCount {
		case 1:
			return "aaa" // hexOne (3 chars)
		case 2:
			return "abcdefgh" // hexTwo -> source (8 chars)
		default:
			return "xy" // hexThree -> rule (2 chars, shorter than source)
		}
	}
	t.Cleanup(func() { computeHmacFunc = oldHmac })

	result := g.sha512("key", "site")
	// source has 8 chars, rule has 2 chars; iterations 2..7 hit the
	// "i >= len(rule)" continue path.
	assert.NotEmpty(t, result)
}

// TestSha512IsNaNPath exercises the math.IsNaN branch by mocking parseFloatFunc
// to return NaN for a specific character.  In practice, single characters from
// base64 can never produce NaN via strconv.ParseFloat, so this branch is dead
// code in production.  The mock lets us cover the branch body.
func TestSha512IsNaNPath(t *testing.T) {
	g := NewGenerator(NewConfig("test", 16, true, true, false))

	oldParse := parseFloatFunc
	parseFloatFunc = func(s string, bitSize int) (float64, error) {
		if s == "a" {
			return math.NaN(), nil
		}
		return strconv.ParseFloat(s, bitSize)
	}
	t.Cleanup(func() { parseFloatFunc = oldParse })

	// Use a key/site combo whose base64 HMAC output contains "a" characters,
	// so the mock triggers the NaN branch.  The rule string's characters
	// determine whether source[i] is upper-cased.
	result := g.sha512("key", "example.com")
	assert.NotEmpty(t, result)
}

// TestSha512IsNaNRuleNotInStr covers the inner "if !strings.Contains" branch
// inside the IsNaN block by mocking parseFloatFunc to return NaN and using
// a rule character that is NOT in "whenthecatisawaythemicewillplay666".
func TestSha512IsNaNRuleNotInStr(t *testing.T) {
	g := NewGenerator(NewConfig("test", 16, true, true, false))

	oldParse := parseFloatFunc
	parseFloatFunc = func(s string, bitSize int) (float64, error) {
		// Make every character return NaN, triggering the IsNaN branch
		// for every iteration.
		return math.NaN(), nil
	}
	t.Cleanup(func() { parseFloatFunc = oldParse })

	result := g.sha512("key", "example.com")
	assert.NotEmpty(t, result)
}

// TestGeneratePwdRunStringError covers the vm.RunString(jsScript) error path
// by temporarily replacing jsScript with syntactically invalid JavaScript.
func TestGeneratePwdRunStringError(t *testing.T) {
	g := NewGenerator(NewConfig("test", 16, true, true, false))

	oldScript := jsScript
	jsScript = "function(){ INVALID JS {{"
	t.Cleanup(func() { jsScript = oldScript })

	pwd := g.generatePwd("testhash1234567890abcdef", 16, false, true)
	assert.Equal(t, "", pwd, "should return empty string when JS fails to parse")
}

// TestGeneratePwdExportError covers the vm.ExportTo error path by providing
// valid JS that defines seekPassword as a non-function value, causing
// ExportTo to fail and the function to panic.
func TestGeneratePwdExportError(t *testing.T) {
	g := NewGenerator(NewConfig("test", 16, true, true, false))

	oldScript := jsScript
	// Valid JS, but seekPassword is a number, not a function.
	jsScript = "var seekPassword = 42;"
	t.Cleanup(func() { jsScript = oldScript })

	assert.Panics(t, func() {
		g.generatePwd("testhash1234567890abcdef", 16, false, true)
	}, "should panic when ExportTo fails")
}
