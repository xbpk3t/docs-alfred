package pwgen

//nolint:revive // Test complexity is acceptable
//func TestGenerate(t *testing.T) {
//	tests := []struct {
//		name        string
//		secretKey   string
//		website     string
//		length      int
//		uppercase   bool
//		numbers     bool
//		punctuation bool
//		expected    string
//	}{
//		{
//			name:        "testsecret + github.com",
//			secretKey:   "testsecret",
//			website:     "github.com",
//			length:      16,
//			uppercase:   true,
//			numbers:     true,
//			punctuation: false,
//			expected:    "", // Will be filled after first run
//		},
//		{
//			name:        "longsecret + example.com",
//			secretKey:   "longsecret",
//			website:     "example.com",
//			length:      20,
//			uppercase:   true,
//			numbers:     true,
//			punctuation: false,
//			expected:    "", // Will be filled after first run
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			config := NewConfig(tt.secretKey, tt.length, tt.uppercase, tt.numbers, tt.punctuation)
//			generator := NewGenerator(config)
//
//			result, err := generator.Generate(tt.website)
//			if err != nil {
//				t.Fatalf("Generate() error = %v", err)
//			}
//
//			if len(result) != tt.length {
//				t.Errorf("Generate() length = %d, want %d", len(result), tt.length)
//			}
//
//			// For the first test case, verify exact output
//			if tt.expected != "" && result != tt.expected {
//				t.Errorf("Generate() = %v, want %v", result, tt.expected)
//			}
//
//			// Print result for manual verification
//			t.Logf("Generated password for %s: %s", tt.website, result)
//		})
//	}
//}
//
//func TestGenerateDeterministic(t *testing.T) {
//	// Test that same inputs always produce same output
//	config := NewConfig("testsecret", 16, true, true, false)
//	generator := NewGenerator(config)
//
//	result1, err := generator.Generate("github.com")
//	if err != nil {
//		t.Fatalf("Generate() error = %v", err)
//	}
//
//	result2, err := generator.Generate("github.com")
//	if err != nil {
//		t.Fatalf("Generate() error = %v", err)
//	}
//
//	if result1 != result2 {
//		t.Errorf("Generate() not deterministic: %v != %v", result1, result2)
//	}
//}
//
//func TestGenerateDifferentWebsites(t *testing.T) {
//	// Test that different websites produce different passwords
//	config := NewConfig("testsecret", 16, true, true, false)
//	generator := NewGenerator(config)
//
//	result1, err := generator.Generate("github.com")
//	if err != nil {
//		t.Fatalf("Generate() error = %v", err)
//	}
//
//	result2, err := generator.Generate("google.com")
//	if err != nil {
//		t.Fatalf("Generate() error = %v", err)
//	}
//
//	if result1 == result2 {
//		t.Errorf("Generate() same password for different websites: %v", result1)
//	}
//}
//
////nolint:revive // Test complexity is acceptable
//func TestGenerateWithPunctuation(t *testing.T) {
//	config := NewConfig("testsecret", 16, true, true, true)
//	generator := NewGenerator(config)
//
//	result, err := generator.Generate("github.com")
//	if err != nil {
//		t.Fatalf("Generate() error = %v", err)
//	}
//
//	// Check that result contains at least one punctuation character
//	hasPunctuation := false
//	punctuationChars := "~*-+()!@#$^&"
//	for _, c := range result {
//		for _, p := range punctuationChars {
//			if c == p {
//				hasPunctuation = true
//
//				break
//			}
//		}
//		if hasPunctuation {
//			break
//		}
//	}
//
//	if !hasPunctuation {
//		t.Logf("Warning: Generated password without punctuation: %s", result)
//	}
//}
