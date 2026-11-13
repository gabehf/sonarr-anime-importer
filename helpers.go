package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// parses the boolean param "name" from url.Values "values"
func parseBoolParam(values url.Values, name string) bool {
	param := values.Get(name)

	if param != "" {
		val, err := strconv.ParseBool(param)
		if err == nil {
			return val
		}
	} else if _, exists := values[name]; exists {
		return true
	}
	return false
}

// just the title, or "title a.k.a. english title" if both exist
func FullAnimeTitle(title, engtitle string) string {
	if engtitle != "" {
		return title + " a.k.a. " + engtitle
	} else {
		return title
	}
}

func RequestString(r *http.Request) string {
	return fmt.Sprintf("%s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
}

// CurrentAnimeSeason returns the AniList season name and year for the given time,
// following the same quarter mapping used by AniList and AniChart.
func CurrentAnimeSeason(t time.Time) (season string, year int) {
	year = t.Year()
	switch t.Month() {
	case time.January, time.February, time.March:
		season = "WINTER"
	case time.April, time.May, time.June:
		season = "SPRING"
	case time.July, time.August, time.September:
		season = "SUMMER"
	default:
		season = "FALL"
	}
	return
}
