package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const anilistQuery = `
query (
  $page: Int
  $type: MediaType
  $isAdult: Boolean
  $search: String
  $format: [MediaFormat]
  $status: MediaStatus
  $countryOfOrigin: CountryCode
  $season: MediaSeason
  $seasonYear: Int
  $year: String
  $onList: Boolean
  $yearLesser: FuzzyDateInt
  $yearGreater: FuzzyDateInt
  $averageScoreGreater: Int
  $averageScoreLesser: Int
  $genres: [String]
  $excludedGenres: [String]
  $tags: [String]
  $excludedTags: [String]
  $minimumTagRank: Int
  $sort: [MediaSort]
) {
  Page(page: $page, perPage: 20) {
    pageInfo {
      hasNextPage
    }
    media(
      type: $type
      season: $season
      format_in: $format
      status: $status
      countryOfOrigin: $countryOfOrigin
      search: $search
      onList: $onList
      seasonYear: $seasonYear
      startDate_like: $year
      startDate_lesser: $yearLesser
      startDate_greater: $yearGreater
      averageScore_greater: $averageScoreGreater
      averageScore_lesser: $averageScoreLesser
      genre_in: $genres
      genre_not_in: $excludedGenres
      tag_in: $tags
      tag_not_in: $excludedTags
      minimumTagRank: $minimumTagRank
      sort: $sort
      isAdult: $isAdult
    ) {
      id
	  idMal
      title {
        romaji
        english
      }
    }
  }
}
`

type AniListPageInfo struct {
	HasNextPage bool `json:"hasNextPage"`
}
type AniListMediaItem struct {
	Id    int          `json:"id"`
	IdMal int          `json:"idMal"`
	Title AniListTitle `json:"title"`
}
type AniListTitle struct {
	Romaji  string `json:"romaji"`
	English string `json:"english"`
}
type AniListResponsePage struct {
	PageInfo AniListPageInfo    `json:"pageInfo"`
	Media    []AniListMediaItem `json:"media"`
}
type AniListResponseData struct {
	Page AniListResponsePage `json:"Page"`
}
type AniListApiResponse struct {
	Data AniListResponseData `json:"data"`
}

func handleAniListAnimeSearch(idMap *ConcurrentMap) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		opts, err := SearchOptsFromAniListRequest(r)
		if err != nil {
			log.Printf("Error creating search options: %v", err)
			return
		}
		search, err := makeApiRequest(idMap, AniList, opts)
		if err != nil {
			w.WriteHeader(500)
			if _, writeErr := w.Write([]byte(err.Error())); writeErr != nil {
				log.Printf("Error writing error response: %v", writeErr)
			}
		} else {
			w.WriteHeader(http.StatusOK)
			if _, writeErr := w.Write(search); writeErr != nil {
				log.Printf("Error writing response: %v", writeErr)
			}
		}
	}
}

func SearchOptsFromAniListRequest(r *http.Request) (*SearchOpts, error) {
	q := r.URL.Query()

	// set default params
	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil {
		return nil, errors.New(" Required parameter \"limit\" not specified")
	}

	// dont include limit in the AniList api call as its already hard coded at 20 per page
	q.Del("limit")

	q.Set("type", "ANIME")

	skipDedup := parseBoolParam(q, "allowDuplicates")
	mergeSeasons := parseBoolParam(q, "mergeSeasons")

	return &SearchOpts{
		AllowDuplicates: skipDedup,
		MergeSeasons:    mergeSeasons,
		Query:           q,
		Limit:           limit,
	}, nil
}

func makeAniListApiCall(q url.Values) (*AniListApiResponse, error) {
	// Build the GraphQL request body
	variables := BuildGraphQLVariables(q)

	body := map[string]any{
		"query":     anilistQuery,
		"variables": variables,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	// Make the POST request
	resp, err := http.Post("https://graphql.anilist.co", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("Error closing response body: %v", closeErr)
		}
	}()

	respData := new(AniListApiResponse)
	err = json.NewDecoder(resp.Body).Decode(respData)
	if err != nil {
		return nil, err
	}
	return respData, nil
}

// BuildGraphQLVariables converts URL query parameters into a GraphQL variables map.
func BuildGraphQLVariables(params url.Values) map[string]any {
	vars := make(map[string]any)

	// Helper to convert comma-separated strings into slices
	parseList := func(key string) []string {
		if val := params.Get(key); val != "" {
			return strings.Split(val, ",")
		}
		return nil
	}

	// Helper to convert integer parameters
	parseInt := func(key string) *int {
		if val := params.Get(key); val != "" {
			if i, err := strconv.Atoi(val); err == nil {
				return &i
			}
		}
		return nil
	}

	// Helper to convert boolean parameters
	parseBool := func(key string) *bool {
		if val := params.Get(key); val != "" {
			if b, err := strconv.ParseBool(val); err == nil {
				return &b
			}
		}
		return nil
	}

	// Basic int and bool params
	if v := parseInt("page"); v != nil {
		vars["page"] = *v
	}
	if v := parseInt("seasonYear"); v != nil {
		vars["seasonYear"] = *v
	}
	if v := parseInt("yearLesser"); v != nil {
		vars["yearLesser"] = *v
	}
	if v := parseInt("yearGreater"); v != nil {
		vars["yearGreater"] = *v
	}
	if v := parseInt("averageScoreGreater"); v != nil {
		vars["averageScoreGreater"] = *v
	}
	if v := parseInt("averageScoreLesser"); v != nil {
		vars["averageScoreLesser"] = *v
	}
	if v := parseInt("minimumTagRank"); v != nil {
		vars["minimumTagRank"] = *v
	}
	if v := parseBool("onList"); v != nil {
		vars["onList"] = *v
	}
	if v := parseBool("isAdult"); v != nil {
		vars["isAdult"] = *v
	}

	// Simple string params
	for _, key := range []string{"type", "search", "status", "countryOfOrigin", "season", "year"} {
		if val := params.Get(key); val != "" {
			vars[key] = val
		}
	}

	// List-type string params
	for _, key := range []string{"format", "genres", "excludedGenres", "tags", "excludedTags", "sort"} {
		if list := parseList(key); list != nil {
			vars[key] = list
		}
	}

	return vars
}
