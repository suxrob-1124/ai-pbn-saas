package scheduler

import (
	"fmt"
	"strings"
	"time"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

// SkipDetail is an alias for sqlstore.SkipDetail.
type SkipDetail = sqlstore.SkipDetail

// FilterGenerationDomainsWithReasons returns eligible domains and skip details
// for a generation schedule run. A domain is eligible if its status is "waiting".
func FilterGenerationDomainsWithReasons(domains []sqlstore.Domain) ([]sqlstore.Domain, []SkipDetail) {
	eligible := make([]sqlstore.Domain, 0, len(domains))
	skipped := make([]SkipDetail, 0)
	for _, d := range domains {
		if !IsDomainWaiting(d) {
			skipped = append(skipped, SkipDetail{
				DomainID:  d.ID,
				DomainURL: d.URL,
				Reason:    fmt.Sprintf("domain status '%s' is not 'waiting'", strings.TrimSpace(d.Status)),
			})
			continue
		}
		eligible = append(eligible, d)
	}
	return eligible, skipped
}

// FilterLinkDomainsWithReasons returns eligible domains and skip details
// for a link schedule run.
func FilterLinkDomainsWithReasons(domains []sqlstore.Domain, now time.Time, scheduleRunAt time.Time) ([]sqlstore.Domain, []SkipDetail) {
	eligible := make([]sqlstore.Domain, 0, len(domains))
	skipped := make([]SkipDetail, 0)
	for _, d := range domains {
		if !IsDomainPublished(d) {
			skipped = append(skipped, SkipDetail{
				DomainID:  d.ID,
				DomainURL: d.URL,
				Reason:    fmt.Sprintf("domain not published (status='%s')", strings.TrimSpace(d.Status)),
			})
			continue
		}
		if !IsLinkStatusEligible(d) {
			skipped = append(skipped, SkipDetail{
				DomainID:  d.ID,
				DomainURL: d.URL,
				Reason:    fmt.Sprintf("link_status '%s' is not eligible", strings.TrimSpace(d.LinkStatus.String)),
			})
			continue
		}
		anchor := strings.TrimSpace(d.LinkAnchorText.String)
		target := strings.TrimSpace(d.LinkAcceptorURL.String)
		if !d.LinkAnchorText.Valid || !d.LinkAcceptorURL.Valid || anchor == "" || target == "" {
			reason := "link_anchor_text or link_acceptor_url is empty"
			if anchor == "" && target == "" {
				reason = "both link_anchor_text and link_acceptor_url are empty"
			} else if anchor == "" {
				reason = "link_anchor_text is empty"
			} else {
				reason = "link_acceptor_url is empty"
			}
			skipped = append(skipped, SkipDetail{
				DomainID:  d.ID,
				DomainURL: d.URL,
				Reason:    reason,
			})
			continue
		}
		effective := EffectiveLinkReadyAt(d, scheduleRunAt)
		if !effective.IsZero() && effective.After(now) {
			skipped = append(skipped, SkipDetail{
				DomainID:  d.ID,
				DomainURL: d.URL,
				Reason:    fmt.Sprintf("link_ready_at %s is in the future (now=%s)", effective.Format(time.RFC3339), now.Format(time.RFC3339)),
			})
			continue
		}
		eligible = append(eligible, d)
	}
	return eligible, skipped
}

// IsDomainPublished returns true if the domain has been published.
func IsDomainPublished(d sqlstore.Domain) bool {
	if d.PublishedAt.Valid {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(d.Status), "published")
}

// IsDomainWaiting returns true if the domain status is "waiting".
func IsDomainWaiting(d sqlstore.Domain) bool {
	return strings.EqualFold(strings.TrimSpace(d.Status), "waiting")
}

// IsLinkStatusEligible returns true if the domain's link status allows a new link task.
func IsLinkStatusEligible(d sqlstore.Domain) bool {
	if !d.LinkStatus.Valid {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(d.LinkStatus.String))
	if status == "" || status == "ready" {
		return true
	}
	return status == "needs_relink" || status == "pending"
}

// EffectiveLinkReadyAt returns the effective time after which a link task may run.
func EffectiveLinkReadyAt(domain sqlstore.Domain, scheduleRunAt time.Time) time.Time {
	effective := scheduleRunAt
	if effective.IsZero() && domain.LinkReadyAt.Valid {
		return domain.LinkReadyAt.Time
	}
	if domain.LinkReadyAt.Valid && domain.LinkReadyAt.Time.After(effective) {
		return domain.LinkReadyAt.Time
	}
	return effective
}
