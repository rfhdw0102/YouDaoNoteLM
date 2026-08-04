package feedback

import "time"

// Rating identifies a thumbs-up or thumbs-down assessment.
type Rating string

const (
	RatingUp   Rating = "up"
	RatingDown Rating = "down"
)

// ReasonCode identifies a fixed reason the user selects after choosing a rating.
type ReasonCode string

const (
	// Reasons for both ratings.
	ReasonOther ReasonCode = "other"

	// Reasons specific to thumbs-up.
	ReasonPreferenceMatched ReasonCode = "preference_matched"
	ReasonHelpful           ReasonCode = "helpful"
	ReasonCitationReliable  ReasonCode = "citation_reliable"

	// Reasons specific to thumbs-down.
	ReasonCitationInaccurate     ReasonCode = "citation_inaccurate"
	ReasonRequirementMisunderstood ReasonCode = "requirement_misunderstood"
	ReasonStyleNotExpected       ReasonCode = "style_not_expected"
	ReasonNotHelpful             ReasonCode = "not_helpful"
)

// Feedback is the safe, structured view exposed to consumers and the API.
// It intentionally excludes database IDs and user/message identifiers.
type Feedback struct {
	Rating    Rating    `json:"rating"`
	Reason    ReasonCode `json:"reason"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// AdminFeedbackItem is the desensitized view exposed to admin APIs.
// It intentionally excludes user IDs, message IDs, content, and references.
type AdminFeedbackItem struct {
	CreatedAt time.Time  `json:"created_at"`
	Rating    Rating     `json:"rating"`
	Reason    ReasonCode `json:"reason"`
}

// FeedbackOverview contains aggregate statistics for the admin dashboard.
type FeedbackOverview struct {
	TotalCount    int64              `json:"total_count"`
	UpCount       int64              `json:"up_count"`
	DownCount     int64              `json:"down_count"`
	PositiveRatio float64            `json:"positive_ratio"`
	ReasonCounts  map[ReasonCode]int64 `json:"reason_counts"`
}

// AdminFilter holds the query parameters for admin feedback queries.
type AdminFilter struct {
	From   time.Time  `json:"from"`
	To     time.Time  `json:"to"`
	Rating *Rating    `json:"rating,omitempty"`
	Reason *ReasonCode `json:"reason,omitempty"`
}
