package feedback

import "errors"

var (
	ErrInvalidRating  = errors.New("无效的评价类型")
	ErrInvalidReason  = errors.New("无效的评价原因")
	ErrReasonMismatch = errors.New("评价原因与评价类型不匹配")
	ErrInvalidUser    = errors.New("无效的用户")
	ErrInvalidMessage = errors.New("无效的消息")
	ErrAnswerNotFound = errors.New("回答不存在")
	ErrInternalRead   = errors.New("读取反馈失败")
	ErrInternalWrite  = errors.New("写入反馈失败")
)

// validUpReasons contains reasons allowed for thumbs-up.
var validUpReasons = map[ReasonCode]bool{
	ReasonPreferenceMatched: true,
	ReasonHelpful:           true,
	ReasonCitationReliable:  true,
	ReasonOther:             true,
}

// validDownReasons contains reasons allowed for thumbs-down.
var validDownReasons = map[ReasonCode]bool{
	ReasonCitationInaccurate:     true,
	ReasonRequirementMisunderstood: true,
	ReasonStyleNotExpected:       true,
	ReasonNotHelpful:             true,
	ReasonOther:                  true,
}

// ValidateRating checks if the rating is a known value.
func ValidateRating(rating Rating) error {
	switch rating {
	case RatingUp, RatingDown:
		return nil
	default:
		return ErrInvalidRating
	}
}

// ValidateReason checks if the reason code is a known value.
func ValidateReason(reason ReasonCode) error {
	switch reason {
	case ReasonOther,
		ReasonPreferenceMatched, ReasonHelpful, ReasonCitationReliable,
		ReasonCitationInaccurate, ReasonRequirementMisunderstood,
		ReasonStyleNotExpected, ReasonNotHelpful:
		return nil
	default:
		return ErrInvalidReason
	}
}

// ValidateRatingReasonCombination checks if the reason is valid for the given rating.
func ValidateRatingReasonCombination(rating Rating, reason ReasonCode) error {
	if err := ValidateRating(rating); err != nil {
		return err
	}
	if err := ValidateReason(reason); err != nil {
		return err
	}

	var allowed map[ReasonCode]bool
	if rating == RatingUp {
		allowed = validUpReasons
	} else {
		allowed = validDownReasons
	}

	if !allowed[reason] {
		return ErrReasonMismatch
	}
	return nil
}

// IsValidationError reports whether err is a user-facing validation error
// that should be returned as CodeInvalidParam.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidRating) ||
		errors.Is(err, ErrInvalidReason) ||
		errors.Is(err, ErrReasonMismatch) ||
		errors.Is(err, ErrInvalidUser) ||
		errors.Is(err, ErrInvalidMessage)
}
