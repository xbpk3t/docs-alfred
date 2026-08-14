package presenter

// strptr returns a pointer to s for constructing pointer model fields in tests.
func strptr(s string) *string { return &s }

// intptr returns a pointer to i for constructing pointer model fields in tests.
func intptr(i int) *int { return &i }
