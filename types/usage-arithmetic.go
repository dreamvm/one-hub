package types

import (
	"errors"
	"math"
)

// countFields includes raw details before GetExtraTokens can omit nonpositive
// values. Keep this list aligned with the token counts in Usage and its details.
func (u *Usage) countFields() []*int {
	p, c := &u.PromptTokensDetails, &u.CompletionTokensDetails
	return []*int{&u.PromptTokens, &u.CompletionTokens, &u.TotalTokens,
		&p.AudioTokens, &p.CachedTokens, &p.TextTokens, &p.ImageTokens, &p.CachedTokensInternal, &p.CachedWriteTokens, &p.CachedReadTokens,
		&c.AudioTokens, &c.TextTokens, &c.ReasoningTokens, &c.AcceptedPredictionTokens, &c.RejectedPredictionTokens, &c.ImageTokens}
}

func (u *Usage) Validate() error {
	if u == nil {
		return errors.New("missing usage")
	}
	for _, count := range u.countFields() {
		if *count < 0 {
			return errors.New("negative usage count")
		}
	}
	if u.CompletionTokens > math.MaxInt-u.PromptTokens {
		return errors.New("usage count overflow")
	}
	for _, count := range u.ExtraTokens {
		if count < 0 {
			return errors.New("negative extra usage count")
		}
	}
	for _, billing := range u.ExtraBilling {
		if billing.CallCount < 0 {
			return errors.New("negative extra service count")
		}
	}
	return nil
}

// Merge validates a complete snapshot before replacing the accumulated event.
// Canonical extra counts are accumulated independently from logging details so
// explicit map values and provider detail fields do not overwrite one another.
func (u *UsageEvent) Merge(other *UsageEvent) error {
	if other == nil {
		return nil
	}
	current, next := u.ToChatUsage(), other.ToChatUsage()
	if err := current.Validate(); err != nil {
		return err
	}
	if err := next.Validate(); err != nil {
		return err
	}
	left, right := current.countFields(), next.countFields()
	leftExtra, rightExtra := current.GetExtraTokens(), next.GetExtraTokens()
	for i := range left {
		if *right[i] > math.MaxInt-*left[i] {
			return errors.New("realtime usage overflow")
		}
		*left[i] += *right[i]
	}
	for key, value := range rightExtra {
		if value > math.MaxInt-leftExtra[key] {
			return errors.New("realtime extra usage overflow")
		}
		leftExtra[key] += value
	}
	if err := current.Validate(); err != nil {
		return err
	}
	*u = UsageEvent{InputTokens: current.PromptTokens, OutputTokens: current.CompletionTokens, TotalTokens: current.TotalTokens,
		InputTokenDetails: current.PromptTokensDetails, OutputTokenDetails: current.CompletionTokensDetails, ExtraTokens: leftExtra}
	return nil
}
