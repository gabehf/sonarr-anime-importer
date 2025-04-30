package main

import (
	"errors"
	"log"
	"net/http"
	"strconv"
)

func handleMalAnimeSearch(idMap *ConcurrentMap) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		opts, err := SearchOptsFromMalRequest(r)
		if err != nil {
			log.Printf("Error creating search options: %v", err)
			return
		}
		search, err := makeApiRequest(idMap, MyAnimeList, opts)
		if err != nil {
			w.WriteHeader(500)
			if _, writeErr := w.Write([]byte(err.Error())); writeErr != nil {
				log.Printf("Error writing error response: %v", writeErr)
			}
		} else {
			w.WriteHeader(http.StatusOK)
			if _, writeErr := w.Write([]byte(search)); writeErr != nil {
				log.Printf("Error writing response: %v", writeErr)
			}
		}
	})
}

func SearchOptsFromMalRequest(r *http.Request) (*SearchOpts, error) {
	q := r.URL.Query()

	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil {
		return nil, errors.New(" Required parameter \"limit\" not specified")
	}

	skipDedup := parseBoolParam(q, "allow_duplicates")
	mergeSeasons := parseBoolParam(q, "merge_seasons")

	// for some reason Jikan responds with 400 Bad Request for any limit >25
	// so instead, we just limit when mapping the data and remove the limit from the Jikan request
	q.Del("limit")

	return &SearchOpts{
		AllowDuplicates: skipDedup,
		MergeSeasons:    mergeSeasons,
		Query:           q,
		Limit:           limit,
	}, nil
}
