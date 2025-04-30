package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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
