package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

type AnimeEntry struct {
	TvdbId    int `json:"tvdb_id"`
	MalId     any `json:"mal_id"`
	AniListId int `json:"anilist_id"`
}

type ConcurrentMap struct {
	mal map[int]int
	mut sync.RWMutex
}

func (m *ConcurrentMap) GetByMalId(i int) int {
	m.mut.RLock()
	defer m.mut.RUnlock()
	return m.mal[i]
}

var PermaSkipIds []string

var Cache = cache.New(10*time.Minute, 15*time.Minute)

var lastBuiltAnimeIdList time.Time

const Version = "v0.2.2"

func main() {
	log.Printf("sonarr-anime-importer %s", Version)
	log.Println("Building Anime ID Associations...")
	var idMap = new(ConcurrentMap)
	buildIdMap(idMap)
	permaSkipStr := os.Getenv("ALWAYS_SKIP_TVDB_IDS")
	PermaSkipIds = strings.Split(permaSkipStr, ",")
	if permaSkipStr != "" {
		log.Printf("Always skipping TVDB IDs: %v\n", PermaSkipIds)
	}
	middleware := []Middleware{
		loggerMiddleware,
		cacheMiddleware,
		newRebuildStaleIdMapMiddleware(idMap),
	}
	http.HandleFunc("/v1/mal/anime", ChainMiddleware(handleMalAnimeSearch(idMap), middleware...))
	http.HandleFunc("/v1/anilist/anime", ChainMiddleware(handleAniListAnimeSearch(idMap), middleware...))
	log.Println("Listening on :3333")

	srv := &http.Server{
		Addr:         ":3333",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

func buildIdMap(idMap *ConcurrentMap) {
	// build/re-build the mal -> tvdb association table
	idMap.mut.Lock()
	defer idMap.mut.Unlock()
	var idListBytes []byte
	resp, err := http.Get("https://raw.githubusercontent.com/Kometa-Team/Anime-IDs/master/anime_ids.json")
	if err != nil {
		log.Fatal("Error fetching anime_ids.json: ", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("Error closing response body: %v", closeErr)
		}
	}()
	idListBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading anime_ids.json: ", err)
	}

	var animeMap map[string]AnimeEntry
	err = json.Unmarshal(idListBytes, &animeMap)
	if err != nil {
		log.Fatal("Error unmarshalling anime_ids.json: ", err)
	}
	idMap.mal = make(map[int]int, 0)
	for _, entry := range animeMap {
		if entry.MalId == nil {
			continue
		}
		var malIdList []int
		switch t := reflect.TypeOf(entry.MalId); t.Kind() {
		case reflect.String:
			s := strings.Split(entry.MalId.(string), ",")
			for _, ss := range s {
				id, err := strconv.Atoi(ss)
				if err != nil {
					log.Fatal("Error building anime id associations: ", err)
				}
				malIdList = append(malIdList, id)
			}
		case reflect.Float64:
			malIdList = append(malIdList, int(entry.MalId.(float64)))
		}
		for _, val := range malIdList {
			idMap.mal[val] = entry.TvdbId
		}
		if entry.AniListId == 0 {
			continue
		}
	}
	lastBuiltAnimeIdList = time.Now()
}
