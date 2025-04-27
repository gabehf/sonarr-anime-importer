package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
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

func handleAniListAnimeSearch(idMap *ConcurrentMap, permaSkipIds []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		search, err := getAniListAnimeSearch(idMap, permaSkipIds, r)
		if err != nil {
			w.WriteHeader(500)
			if _, writeErr := w.Write([]byte(err.Error())); writeErr != nil {
				log.Printf("Error writing error response: %v", writeErr)
			}
		} else {
			if _, writeErr := w.Write(search); writeErr != nil {
				log.Printf("Error writing response: %v", writeErr)
			}
		}
	}
}

func getAniListAnimeSearch(idMap *ConcurrentMap, permaSkipAniListIds []string, r *http.Request) ([]byte, error) {
	q := r.URL.Query()

	// set default params
	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil {
		return nil, errors.New(" Required parameter \"limit\" not specified")
	}
	q.Set("type", "ANIME")

	// dont include limit in the AniList api call as its already hard coded at 20 per page
	q.Del("limit")

	skipDedup := parseBoolParam(q, "allowDuplicates")

	hasNextPage := true
	page := 0
	resp := []ResponseItem{}
	count := 0
	usedIds := make(map[int]bool, 0)
	for hasNextPage {
		page++
		q.Set("page", strconv.Itoa(page))
		result, err := makeAniListApiCall(q)
		if err != nil {
			log.Println("Error sending request to AniList: ", err)
			return nil, err
		}

		// map the data
		for _, item := range result.Data.Page.Media {
			if idMap.GetByMalId(item.IdMal) == 0 {
				log.Printf("AniList ID %d (%s) has no associated TVDB ID, skipping...\n", item.Id, FullAnimeTitle(item.Title.Romaji, item.Title.English))
				continue
			}
			if usedIds[item.Id] && !skipDedup {
				log.Printf("AniList ID %d (%s) is a duplicate, skipping...\n", item.Id, FullAnimeTitle(item.Title.Romaji, item.Title.English))
				continue
			}
			if slices.Contains(permaSkipAniListIds, strconv.Itoa(item.Id)) {
				log.Printf("AniList ID %d (%s) is set to always skip, skipping...\n", item.Id, FullAnimeTitle(item.Title.Romaji, item.Title.English))
				continue
			}
			count++
			if count > limit {
				break
			}
			resp = append(resp,
				ResponseItem{
					item.Title.Romaji,
					item.Title.English,
					item.IdMal,
					item.Id,
					idMap.GetByMalId(item.IdMal),
				})
			usedIds[item.Id] = true
		}
		hasNextPage = result.Data.Page.PageInfo.HasNextPage
		if count > limit {
			break
		}
		if hasNextPage {
			time.Sleep(500 * time.Millisecond) // sleep between requests for new page to try and avoid rate limits
		}
	}

	respJson, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		log.Println("Error marshalling response: ", err)
		return nil, err
	}
	return respJson, nil
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
