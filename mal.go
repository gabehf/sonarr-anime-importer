package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/darenliang/jikan-go"
)

func handleMalAnimeSearch(idMap *ConcurrentMap, permaSkipMalIds []string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		search, err := getJikanAnimeSearch(idMap, permaSkipMalIds, r)
		if err != nil {
			w.WriteHeader(500)
			if _, writeErr := w.Write([]byte(err.Error())); writeErr != nil {
				log.Printf("Error writing error response: %v", writeErr)
			}
		} else {
			if _, writeErr := w.Write([]byte(search)); writeErr != nil {
				log.Printf("Error writing response: %v", writeErr)
			}
		}
	})
}

func getJikanAnimeSearch(idMap *ConcurrentMap, permaSkipMalIds []string, r *http.Request) (string, error) {
	q := r.URL.Query()

	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil {
		return "", errors.New(" Required parameter \"limit\" not specified")
	}

	skipDedup := parseBoolParam(q, "allow_duplicates")

	// for some reason Jikan responds with 400 Bad Request for any limit >25
	// so instead, we just limit when mapping the data and remove the limit from the Jikan request
	q.Del("limit")

	hasNextPage := true
	page := 0
	resp := []ResponseItem{}
	count := 0
	usedIds := make(map[int]bool, 0)
	for hasNextPage {
		page++
		q.Set("page", strconv.Itoa(page))
		result, err := jikan.GetAnimeSearch(q)
		if err != nil {
			log.Println("Error sending request to Jikan: ", err)
			return "", err
		}

		// map the data
		for _, item := range result.Data {
			if idMap.GetByMalId(item.MalId) == 0 {
				log.Printf("MyAnimeList ID %d (%s) has no associated TVDB ID, skipping...\n", item.MalId, FullAnimeTitle(item.Title, item.TitleEnglish))
				continue
			}
			if usedIds[item.MalId] && !skipDedup {
				log.Printf("MyAnimeList ID %d (%s) is a duplicate, skipping...\n", item.MalId, FullAnimeTitle(item.Title, item.TitleEnglish))
				continue
			}
			if slices.Contains(permaSkipMalIds, strconv.Itoa(item.MalId)) {
				log.Printf("MyAnimeList ID %d (%s) is set to always skip, skipping...\n", item.MalId, FullAnimeTitle(item.Title, item.TitleEnglish))
				continue
			}
			count++
			if count > limit {
				break
			}
			resp = append(resp,
				ResponseItem{
					item.Title,
					item.TitleEnglish,
					item.MalId,
					0,
					idMap.GetByMalId(item.MalId),
				})
			usedIds[item.MalId] = true
		}
		hasNextPage = result.Pagination.HasNextPage
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
		return "", err
	}
	return string(respJson), nil
}
