package api

import (
	"fmt"
	"time"
)

// UsageData represents the complete usage information from Claude
type UsageData struct {
	FiveHour      UsageStat `json:"five_hour"`      // 5-hour session usage
	SevenDay      UsageStat `json:"seven_day"`      // Weekly usage
	SevenDayOpus  UsageStat `json:"seven_day_opus"` // Weekly Opus usage
	SevenDaySonnet UsageStat `json:"seven_day_sonnet"` // Weekly Sonnet usage
	LastUpdated   time.Time `json:"last_updated"`
}

// UsageStat represents a single usage metric
type UsageStat struct {
	Utilization float64   `json:"utilization"` // Percentage 0-100 (API returns this directly)
	ResetsAt    time.Time `json:"resets_at"`
	Label       string    `json:"label"`
}

// GetPercentage returns the utilization as a display percentage (already 0-100)
func (u *UsageStat) GetPercentage() float64 {
	return u.Utilization
}

// GetTimeUntilReset returns duration until reset
func (u *UsageStat) GetTimeUntilReset() time.Duration {
	return time.Until(u.ResetsAt)
}

// TimeUntilReset returns a human-readable string for time until reset
func TimeUntilReset(resetAt time.Time) string {
	duration := time.Until(resetAt)
	if duration < 0 {
		return "Now"
	}

	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60

	if hours > 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// OrganizationInfo holds the user's organization details
type OrganizationInfo struct {
	ID   string `json:"uuid"`
	Name string `json:"name"`
}

// UsageAPIResponse represents the raw response from Claude's usage API
// Endpoint: /api/organizations/{orgId}/usage
type UsageAPIResponse struct {
	FiveHour       *UsageMetric `json:"five_hour,omitempty"`
	SevenDay       *UsageMetric `json:"seven_day,omitempty"`
	SevenDayOpus   *UsageMetric `json:"seven_day_opus,omitempty"`
	SevenDaySonnet *UsageMetric `json:"seven_day_sonnet,omitempty"`
}

// UsageMetric represents a single metric from the API
type UsageMetric struct {
	Utilization float64 `json:"utilization"` // 0 to 100 (percentage)
	ResetsAt    string  `json:"resets_at"`   // ISO8601 timestamp
}

// ToUsageData converts the API response to our internal format
func (r *UsageAPIResponse) ToUsageData() *UsageData {
	data := &UsageData{
		LastUpdated: time.Now(),
	}

	if r.FiveHour != nil {
		data.FiveHour = UsageStat{
			Utilization: r.FiveHour.Utilization,
			ResetsAt:    parseTime(r.FiveHour.ResetsAt),
			Label:       "5-Hour",
		}
	}

	if r.SevenDay != nil {
		data.SevenDay = UsageStat{
			Utilization: r.SevenDay.Utilization,
			ResetsAt:    parseTime(r.SevenDay.ResetsAt),
			Label:       "Weekly",
		}
	}

	if r.SevenDayOpus != nil {
		data.SevenDayOpus = UsageStat{
			Utilization: r.SevenDayOpus.Utilization,
			ResetsAt:    parseTime(r.SevenDayOpus.ResetsAt),
			Label:       "Opus",
		}
	}

	if r.SevenDaySonnet != nil {
		data.SevenDaySonnet = UsageStat{
			Utilization: r.SevenDaySonnet.Utilization,
			ResetsAt:    parseTime(r.SevenDaySonnet.ResetsAt),
			Label:       "Sonnet",
		}
	}

	return data
}

// parseTime parses ISO8601 timestamp string
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}

	// Try various formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}

	return time.Time{}
}
