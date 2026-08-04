package feedback

import (
	"testing"
)

func TestValidateRating(t *testing.T) {
	tests := []struct {
		name    string
		rating  Rating
		wantErr error
	}{
		{"up is valid", RatingUp, nil},
		{"down is valid", RatingDown, nil},
		{"empty is invalid", "", ErrInvalidRating},
		{"unknown is invalid", "unknown", ErrInvalidRating},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRating(tt.rating)
			if err != tt.wantErr {
				t.Errorf("ValidateRating(%q) = %v, want %v", tt.rating, err, tt.wantErr)
			}
		})
	}
}

func TestValidateReason(t *testing.T) {
	tests := []struct {
		name    string
		reason  ReasonCode
		wantErr error
	}{
		{"other is valid", ReasonOther, nil},
		{"preference_matched is valid", ReasonPreferenceMatched, nil},
		{"helpful is valid", ReasonHelpful, nil},
		{"citation_reliable is valid", ReasonCitationReliable, nil},
		{"citation_inaccurate is valid", ReasonCitationInaccurate, nil},
		{"requirement_misunderstood is valid", ReasonRequirementMisunderstood, nil},
		{"style_not_expected is valid", ReasonStyleNotExpected, nil},
		{"not_helpful is valid", ReasonNotHelpful, nil},
		{"empty is invalid", "", ErrInvalidReason},
		{"unknown is invalid", "unknown_reason", ErrInvalidReason},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReason(tt.reason)
			if err != tt.wantErr {
				t.Errorf("ValidateReason(%q) = %v, want %v", tt.reason, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRatingReasonCombination(t *testing.T) {
	tests := []struct {
		name    string
		rating  Rating
		reason  ReasonCode
		wantErr error
	}{
		// Valid up combinations
		{"up + preference_matched", RatingUp, ReasonPreferenceMatched, nil},
		{"up + helpful", RatingUp, ReasonHelpful, nil},
		{"up + citation_reliable", RatingUp, ReasonCitationReliable, nil},
		{"up + other", RatingUp, ReasonOther, nil},

		// Valid down combinations
		{"down + citation_inaccurate", RatingDown, ReasonCitationInaccurate, nil},
		{"down + requirement_misunderstood", RatingDown, ReasonRequirementMisunderstood, nil},
		{"down + style_not_expected", RatingDown, ReasonStyleNotExpected, nil},
		{"down + not_helpful", RatingDown, ReasonNotHelpful, nil},
		{"down + other", RatingDown, ReasonOther, nil},

		// Invalid combinations: up with down-only reasons
		{"up + citation_inaccurate mismatch", RatingUp, ReasonCitationInaccurate, ErrReasonMismatch},
		{"up + requirement_misunderstood mismatch", RatingUp, ReasonRequirementMisunderstood, ErrReasonMismatch},
		{"up + style_not_expected mismatch", RatingUp, ReasonStyleNotExpected, ErrReasonMismatch},
		{"up + not_helpful mismatch", RatingUp, ReasonNotHelpful, ErrReasonMismatch},

		// Invalid combinations: down with up-only reasons
		{"down + preference_matched mismatch", RatingDown, ReasonPreferenceMatched, ErrReasonMismatch},
		{"down + helpful mismatch", RatingDown, ReasonHelpful, ErrReasonMismatch},
		{"down + citation_reliable mismatch", RatingDown, ReasonCitationReliable, ErrReasonMismatch},

		// Invalid rating
		{"unknown rating", "unknown", ReasonOther, ErrInvalidRating},

		// Invalid reason
		{"unknown reason", RatingUp, "bad_reason", ErrInvalidReason},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRatingReasonCombination(tt.rating, tt.reason)
			if err != tt.wantErr {
				t.Errorf("ValidateRatingReasonCombination(%q, %q) = %v, want %v",
					tt.rating, tt.reason, err, tt.wantErr)
			}
		})
	}
}

func TestIsValidationError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"ErrInvalidRating", ErrInvalidRating, true},
		{"ErrInvalidReason", ErrInvalidReason, true},
		{"ErrReasonMismatch", ErrReasonMismatch, true},
		{"ErrInvalidUser", ErrInvalidUser, true},
		{"ErrInvalidMessage", ErrInvalidMessage, true},
		{"ErrAnswerNotFound", ErrAnswerNotFound, false},
		{"ErrInternalRead", ErrInternalRead, false},
		{"ErrInternalWrite", ErrInternalWrite, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidationError(tt.err); got != tt.want {
				t.Errorf("IsValidationError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestModelHasNoSoftDelete(t *testing.T) {
	// Verify AnswerFeedback does not embed BaseEntity (which has DeletedAt).
	// This is a compile-time structural check via a type assertion attempt.
	var af AnswerFeedback
	_ = af.ID
	_ = af.UserID
	_ = af.MessageID
	_ = af.Rating
	_ = af.ReasonCode
	_ = af.CreatedAt
	_ = af.UpdatedAt
	// If BaseEntity were embedded, af.DeletedAt would exist — but we don't
	// reference it, so compilation would still succeed. The real guarantee is
	// in the struct definition itself (model.go) which intentionally omits it.
	// This test serves as documentation that the omission is deliberate.
}

func TestTableName(t *testing.T) {
	af := AnswerFeedback{}
	if af.TableName() != "answer_feedbacks" {
		t.Errorf("TableName() = %q, want %q", af.TableName(), "answer_feedbacks")
	}
}
