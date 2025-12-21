package internal

import (
	"time"
)

// Holiday represents a holiday with associated content themes.
type Holiday struct {
	Name           string
	MonthDay       string   // "MM-DD" for fixed dates, empty for floating
	FloatingRule   string   // e.g., "4th-thursday-november" for Thanksgiving
	SearchKeywords []string // Keywords to add to search queries
	Genres         []string // Preferred genres during this holiday
	Moods          []string // Preferred moods during this holiday
}

// USHolidays contains major US holidays with content associations.
var USHolidays = []Holiday{
	{
		Name:           "New Year's Day",
		MonthDay:       "01-01",
		SearchKeywords: []string{"new year", "new beginnings", "celebration", "party"},
		Genres:         []string{"comedy", "romance"},
		Moods:          []string{"uplifting", "hopeful", "festive"},
	},
	{
		Name:           "Valentine's Day",
		MonthDay:       "02-14",
		SearchKeywords: []string{"valentine", "love", "romance", "romantic", "couples"},
		Genres:         []string{"romance", "romantic comedy", "drama"},
		Moods:          []string{"romantic", "heartwarming", "passionate"},
	},
	{
		Name:           "St. Patrick's Day",
		MonthDay:       "03-17",
		SearchKeywords: []string{"irish", "ireland", "luck", "green"},
		Genres:         []string{"comedy", "drama"},
		Moods:          []string{"fun", "festive", "whimsical"},
	},
	{
		Name:           "Easter",
		FloatingRule:   "easter-sunday", // Calculated separately
		SearchKeywords: []string{"easter", "spring", "rebirth", "family"},
		Genres:         []string{"family", "animation", "drama"},
		Moods:          []string{"uplifting", "hopeful", "family-friendly"},
	},
	{
		Name:           "Mother's Day",
		FloatingRule:   "2nd-sunday-may",
		SearchKeywords: []string{"mother", "mom", "family", "maternal"},
		Genres:         []string{"drama", "family", "comedy"},
		Moods:          []string{"heartwarming", "emotional", "touching"},
	},
	{
		Name:           "Memorial Day",
		FloatingRule:   "last-monday-may",
		SearchKeywords: []string{"memorial", "veteran", "military", "honor", "sacrifice"},
		Genres:         []string{"war", "drama", "documentary"},
		Moods:          []string{"patriotic", "emotional", "reflective"},
	},
	{
		Name:           "Father's Day",
		FloatingRule:   "3rd-sunday-june",
		SearchKeywords: []string{"father", "dad", "family", "paternal"},
		Genres:         []string{"drama", "comedy", "action"},
		Moods:          []string{"heartwarming", "funny", "touching"},
	},
	{
		Name:           "Independence Day",
		MonthDay:       "07-04",
		SearchKeywords: []string{"july 4th", "independence", "america", "patriotic", "fireworks"},
		Genres:         []string{"action", "war", "drama"},
		Moods:          []string{"patriotic", "exciting", "epic"},
	},
	{
		Name:           "Labor Day",
		FloatingRule:   "1st-monday-september",
		SearchKeywords: []string{"labor", "work", "workers"},
		Genres:         []string{"drama", "documentary"},
		Moods:          []string{"inspiring", "uplifting"},
	},
	{
		Name:           "Halloween",
		MonthDay:       "10-31",
		SearchKeywords: []string{"halloween", "spooky", "scary", "horror", "costume", "trick or treat"},
		Genres:         []string{"horror", "thriller", "supernatural", "comedy horror"},
		Moods:          []string{"scary", "spooky", "dark", "fun-scary", "atmospheric"},
	},
	{
		Name:           "Veterans Day",
		MonthDay:       "11-11",
		SearchKeywords: []string{"veteran", "military", "service", "honor"},
		Genres:         []string{"war", "drama", "documentary"},
		Moods:          []string{"patriotic", "emotional", "respectful"},
	},
	{
		Name:           "Thanksgiving",
		FloatingRule:   "4th-thursday-november",
		SearchKeywords: []string{"thanksgiving", "family", "gratitude", "feast", "turkey"},
		Genres:         []string{"comedy", "family", "drama"},
		Moods:          []string{"heartwarming", "funny", "cozy", "family-friendly"},
	},
	{
		Name:           "Christmas",
		MonthDay:       "12-25",
		SearchKeywords: []string{"christmas", "holiday", "santa", "xmas", "winter", "snow"},
		Genres:         []string{"family", "comedy", "romance", "animation"},
		Moods:          []string{"heartwarming", "festive", "magical", "cozy", "uplifting"},
	},
	{
		Name:           "Christmas Eve",
		MonthDay:       "12-24",
		SearchKeywords: []string{"christmas eve", "holiday", "santa", "night before christmas"},
		Genres:         []string{"family", "comedy", "romance"},
		Moods:          []string{"magical", "festive", "cozy"},
	},
	{
		Name:           "New Year's Eve",
		MonthDay:       "12-31",
		SearchKeywords: []string{"new year's eve", "countdown", "celebration", "party"},
		Genres:         []string{"comedy", "romance"},
		Moods:          []string{"festive", "exciting", "romantic"},
	},
}

// HolidayWindow defines how many days before/after a holiday to consider it "active".
const HolidayWindow = 7

// GetActiveHolidays returns holidays that are currently active (within the window).
func GetActiveHolidays(now time.Time) []Holiday {
	var active []Holiday

	for _, h := range USHolidays {
		if isHolidayActive(h, now) {
			active = append(active, h)
		}
	}

	return active
}

// GetUpcomingHoliday returns the next upcoming holiday within the given days.
func GetUpcomingHoliday(now time.Time, withinDays int) *Holiday {
	for _, h := range USHolidays {
		holidayDate := getHolidayDate(h, now.Year())
		if holidayDate.IsZero() {
			continue
		}

		// If holiday is this year but already passed, check next year
		if holidayDate.Before(now) {
			holidayDate = getHolidayDate(h, now.Year()+1)
		}

		daysUntil := int(holidayDate.Sub(now).Hours() / 24)
		if daysUntil >= 0 && daysUntil <= withinDays {
			return &h
		}
	}
	return nil
}

func isHolidayActive(h Holiday, now time.Time) bool {
	holidayDate := getHolidayDate(h, now.Year())
	if holidayDate.IsZero() {
		return false
	}

	// Check if we're within the window
	windowStart := holidayDate.AddDate(0, 0, -HolidayWindow)
	windowEnd := holidayDate.AddDate(0, 0, HolidayWindow)

	return now.After(windowStart) && now.Before(windowEnd)
}

func getHolidayDate(h Holiday, year int) time.Time {
	if h.MonthDay != "" {
		// Fixed date holiday
		dateStr := h.MonthDay
		month := int(dateStr[0]-'0')*10 + int(dateStr[1]-'0')
		day := int(dateStr[3]-'0')*10 + int(dateStr[4]-'0')
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	}

	// Floating holiday
	switch h.FloatingRule {
	case "2nd-sunday-may": // Mother's Day
		return nthWeekdayOfMonth(year, time.May, time.Sunday, 2)
	case "last-monday-may": // Memorial Day
		return lastWeekdayOfMonth(year, time.May, time.Monday)
	case "3rd-sunday-june": // Father's Day
		return nthWeekdayOfMonth(year, time.June, time.Sunday, 3)
	case "1st-monday-september": // Labor Day
		return nthWeekdayOfMonth(year, time.September, time.Monday, 1)
	case "4th-thursday-november": // Thanksgiving
		return nthWeekdayOfMonth(year, time.November, time.Thursday, 4)
	case "easter-sunday":
		return calculateEaster(year)
	default:
		return time.Time{}
	}
}

// nthWeekdayOfMonth returns the nth occurrence of a weekday in a month.
func nthWeekdayOfMonth(year int, month time.Month, weekday time.Weekday, n int) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	// Find first occurrence of the weekday
	daysUntilWeekday := int(weekday) - int(first.Weekday())
	if daysUntilWeekday < 0 {
		daysUntilWeekday += 7
	}

	// Add weeks to get nth occurrence
	day := 1 + daysUntilWeekday + (n-1)*7
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// lastWeekdayOfMonth returns the last occurrence of a weekday in a month.
func lastWeekdayOfMonth(year int, month time.Month, weekday time.Weekday) time.Time {
	// Start from the last day of the month
	nextMonth := month + 1
	if nextMonth > 12 {
		nextMonth = 1
		year++
	}
	last := time.Date(year, nextMonth, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)

	// Go back to the desired weekday
	daysBack := int(last.Weekday()) - int(weekday)
	if daysBack < 0 {
		daysBack += 7
	}

	return last.AddDate(0, 0, -daysBack)
}

// calculateEaster uses the Anonymous Gregorian algorithm to calculate Easter Sunday.
func calculateEaster(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := ((h + l - 7*m + 114) % 31) + 1

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// SeasonalContent returns content suggestions based on the current season.
type SeasonalContent struct {
	Season   string
	Genres   []string
	Moods    []string
	Keywords []string
}

// GetSeasonalContent returns content suggestions for the current season.
func GetSeasonalContent(now time.Time, latitude float64) SeasonalContent {
	// Determine season based on month and hemisphere
	month := now.Month()
	isNorthern := latitude >= 0

	var season string
	switch {
	case month >= 3 && month <= 5:
		if isNorthern {
			season = "spring"
		} else {
			season = "fall"
		}
	case month >= 6 && month <= 8:
		if isNorthern {
			season = "summer"
		} else {
			season = "winter"
		}
	case month >= 9 && month <= 11:
		if isNorthern {
			season = "fall"
		} else {
			season = "spring"
		}
	default: // December, January, February
		if isNorthern {
			season = "winter"
		} else {
			season = "summer"
		}
	}

	switch season {
	case "spring":
		return SeasonalContent{
			Season:   "spring",
			Genres:   []string{"romance", "comedy", "adventure"},
			Moods:    []string{"uplifting", "fresh", "hopeful", "light"},
			Keywords: []string{"spring", "renewal", "bloom", "new beginnings"},
		}
	case "summer":
		return SeasonalContent{
			Season:   "summer",
			Genres:   []string{"action", "adventure", "comedy", "blockbuster"},
			Moods:    []string{"exciting", "fun", "escapist", "adventurous"},
			Keywords: []string{"summer", "beach", "vacation", "road trip", "outdoor"},
		}
	case "fall":
		return SeasonalContent{
			Season:   "fall",
			Genres:   []string{"drama", "thriller", "horror", "mystery"},
			Moods:    []string{"atmospheric", "cozy", "mysterious", "contemplative"},
			Keywords: []string{"autumn", "fall", "harvest", "change"},
		}
	case "winter":
		return SeasonalContent{
			Season:   "winter",
			Genres:   []string{"family", "drama", "romance", "fantasy"},
			Moods:    []string{"cozy", "heartwarming", "magical", "introspective"},
			Keywords: []string{"winter", "snow", "cold", "fireplace", "hygge"},
		}
	default:
		return SeasonalContent{Season: season}
	}
}
