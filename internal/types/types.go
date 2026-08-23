package types

// SkippedItem describes a line from a tracklist or comment that was skipped (e.g. speech, ID, incomplete).
type SkippedItem struct {
	Timestamp string `json:"timestamp,omitempty"`
	RawText   string `json:"rawText"`
	Reason    string `json:"reason,omitempty"`
}

// Track represents an individual track parsed from a set or YouTube comment.
type Track struct {
	Artist    string `json:"artist"`
	Title     string `json:"title"`
	RawString string `json:"rawString"`
	Timestamp string `json:"timestamp,omitempty"`
}

// SearchResult represents a set found via MixesDB or general web search.
type SearchResult struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// ScrapedSet represents a scraped DJ set page with extracted tracks and skipped items.
type ScrapedSet struct {
	Title        string        `json:"title"`
	URL          string        `json:"url"`
	Tracks       []Track       `json:"tracks"`
	SkippedItems []SkippedItem `json:"skippedItems,omitempty"`
}
