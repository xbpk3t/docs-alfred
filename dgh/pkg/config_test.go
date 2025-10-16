package gh

//func TestManagerLoad(t *testing.T) {
//	// Create a test config file
//	testConfig := `- type: test-type
//  tag: test-tag
//  repo:
//    - url: https://github.com/test/repo1
//      des: Test repository 1
//    - url: https://github.com/test/repo2
//      des: Test repository 2
//      doc: data/test
//`
//
//	// Write test config to temp file
//	tmpFile, err := os.CreateTemp(t.TempDir(), "dgh-test-*.yml")
//	if err != nil {
//		t.Fatalf("Failed to create temp file: %v", err)
//	}
//	defer func() { _ = os.Remove(tmpFile.Name()) }()
//
//	if _, err := tmpFile.WriteString(testConfig); err != nil {
//		t.Fatalf("Failed to write test config: %v", err)
//	}
//	_ = tmpFile.Close()
//
//	// Create manager and load config
//	manager := NewManager(tmpFile.Name(), "")
//	if err := manager.Load(); err != nil {
//		t.Fatalf("Load() error = %v", err)
//	}
//
//	// Verify repos were loaded
//	repos := manager.GetRepos()
//	if len(repos) == 0 {
//		t.Error("Expected repos to be loaded, got 0")
//	}
//
//	// Verify repo properties
//	for _, repo := range repos {
//		if repo.Type != "test-type" {
//			t.Errorf("Expected type 'test-type', got '%s'", repo.Type)
//		}
//		if repo.Tag != "test-tag" {
//			t.Errorf("Expected tag 'test-tag', got '%s'", repo.Tag)
//		}
//	}
//}
//
//func TestManagerFilter(t *testing.T) {
//	// Create a test config file
//	testConfig := `- type: golang
//  tag: lang
//  repo:
//    - url: https://github.com/golang/go
//      des: The Go programming language
//    - url: https://github.com/gin-gonic/gin
//      des: Gin Web Framework
//- type: python
//  tag: lang
//  repo:
//    - url: https://github.com/python/cpython
//      des: The Python programming language
//`
//
//	tmpFile, err := os.CreateTemp(t.TempDir(), "dgh-test-*.yml")
//	if err != nil {
//		t.Fatalf("Failed to create temp file: %v", err)
//	}
//	defer func() { _ = os.Remove(tmpFile.Name()) }()
//
//	if _, err := tmpFile.WriteString(testConfig); err != nil {
//		t.Fatalf("Failed to write test config: %v", err)
//	}
//	_ = tmpFile.Close()
//
//	manager := NewManager(tmpFile.Name(), "")
//	if err := manager.Load(); err != nil {
//		t.Fatalf("Load() error = %v", err)
//	}
//
//	tests := []struct {
//		name     string
//		query    string
//		expected int
//	}{
//		{
//			name:     "filter by golang",
//			query:    "golang",
//			expected: 2, // Matches golang/go and gin-gonic/gin
//		},
//		{
//			name:     "filter by gin",
//			query:    "gin",
//			expected: 1,
//		},
//		{
//			name:     "filter by python",
//			query:    "python",
//			expected: 1,
//		},
//		{
//			name:     "filter by lang tag",
//			query:    "lang",
//			expected: 3,
//		},
//		{
//			name:     "no filter",
//			query:    "",
//			expected: 3,
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			filtered := manager.Filter(tt.query)
//			if len(filtered) != tt.expected {
//				t.Errorf("Filter(%q) returned %d repos, expected %d", tt.query, len(filtered), tt.expected)
//			}
//		})
//	}
//}
//
//func TestRepositoryMethods(t *testing.T) {
//	repo := &Repository{
//		URL:    "https://github.com/test/repo",
//		Des:    "Test description",
//		Type:   "test-type",
//		Tag:    "test-tag",
//		Doc:    "data/test",
//		Topics: Topics{{Topic: "test topic"}},
//	}
//
//	if !repo.IsValid() {
//		t.Error("Expected repo to be valid")
//	}
//
//	if repo.FullName() != "test/repo" {
//		t.Errorf("FullName() = %s, want test/repo", repo.FullName())
//	}
//
//	if repo.GetDes() != "Test description" {
//		t.Errorf("GetDes() = %s, want Test description", repo.GetDes())
//	}
//
//	if repo.GetURL() != "https://github.com/test/repo" {
//		t.Errorf("GetURL() = %s, want https://github.com/test/repo", repo.GetURL())
//	}
//
//	if !repo.HasQs() {
//		t.Error("Expected repo to have topics")
//	}
//}
