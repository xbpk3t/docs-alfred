package curate

// Candidate is the stable intermediate schema emitted by Step 1 (MAF
// resolve) and consumed downstream. Its JSON tags double as the structured
// output contract the agent must fill exactly.
type Candidate struct {
	Name           string `json:"name"`
	Medium         string `json:"medium"`
	Feed           string `json:"feed"`
	URL            string `json:"url"`
	Note           string `json:"note"`
	GinoPri        string `json:"gino_pri,omitempty"`
	SourceCategory string `json:"source_category,omitempty"`
	// Fatal reports a resolution failure from Step 1 (e.g. no_rss). When set,
	// downstream steps skip this candidate.
	Fatal string `json:"fatal,omitempty"`
}

// freqResult is the Step 2 outcome for one candidate after fetch/frequency.
type freqResult struct {
	Candidate Candidate `json:"candidate"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	Last      string    `json:"last,omitempty"`
	Titles    []string  `json:"titles,omitempty"`
	Posts90D  int       `json:"posts_90d"`
}

// verdict is Step 4 per-type structured output (curator). One per feed.
type verdict struct {
	Name       string `json:"name"`
	Decision   string `json:"decision"` // keep | drop
	ReasonCode string `json:"reason_code"`
	ReasonText string `json:"reason_text"`
}
