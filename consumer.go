package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/url"
	"slices"
	"strconv"
	"time"

	"fmt"

	"github.com/darenliang/jikan-go"
	"github.com/patrickmn/go-cache"
)

type SupportedAPI int

const (
	AniList SupportedAPI = iota
	MyAnimeList
)

type ResponseItem struct {
	Title     string `json:"title"`
	TitleEng  string `json:"titleEnglish,omitempty"`
	MalId     int    `json:"malId,omitempty"`
	AniListId int    `json:"anilistId,omitempty"`
	TvdbId    int    `json:"tvdbId"`
}

type SearchOpts struct {
	AllowDuplicates bool
	MergeSeasons    bool
	Query           url.Values
	Limit           int
}

func ResponseItemFromAPI(target SupportedAPI, item any) *ResponseItem {
	switch target {
	case AniList:
		if aniListItem, ok := item.(AniListMediaItem); !ok {
			return nil
		} else {
			return &ResponseItem{
				Title:     aniListItem.Title.Romaji,
				TitleEng:  aniListItem.Title.English,
				MalId:     aniListItem.IdMal,
				AniListId: aniListItem.Id,
			}
		}
	case MyAnimeList:
		if malItem, ok := item.(jikan.AnimeBase); !ok {
			return nil
		} else {
			return &ResponseItem{
				Title:    malItem.Title,
				TitleEng: malItem.TitleEnglish,
				MalId:    malItem.MalId,
			}
		}
	default:
		return nil
	}
}

func makeApiRequest(idMap *ConcurrentMap, target SupportedAPI, opts *SearchOpts) ([]byte, error) {

	hasNextPage := true
	page := 0
	resp := []ResponseItem{}
	apiItems := []*ResponseItem{}
	count := 0
	usedIds := make(map[int]bool, 0)
	usedTvdbIds := make(map[int]bool, 0)

	for hasNextPage {

		page++
		opts.Query.Set("page", strconv.Itoa(page))
		if target == MyAnimeList {
			var result *jikan.AnimeSearch
			if cachedResult, found := Cache.Get(fmt.Sprint(MyAnimeList) + opts.Query.Encode()); found {
				result = cachedResult.(*jikan.AnimeSearch)
				log.Println("Jikan cache hit!")
			} else {
				log.Println(opts.Query.Encode())
				newResult, err := jikan.GetAnimeSearch(opts.Query)
				if err != nil {
					log.Println("Error sending request to Jikan: ", err)
					return nil, err
				}
				result = newResult
				Cache.Set(fmt.Sprint(MyAnimeList)+opts.Query.Encode(), newResult, cache.DefaultExpiration)
			}
			for _, item := range result.Data {
				respItem := ResponseItemFromAPI(MyAnimeList, item)
				if respItem == nil {
					return nil, errors.New("failed to parse item from mal api")
				}
				apiItems = append(apiItems, respItem)
			}
			hasNextPage = result.Pagination.HasNextPage
		} else if target == AniList {
			result, err := makeAniListApiCall(opts.Query)
			if err != nil {
				log.Println("Error sending request to AniList: ", err)
				return nil, err
			}
			for _, item := range result.Data.Page.Media {
				respItem := ResponseItemFromAPI(AniList, item)
				if respItem == nil {
					return nil, errors.New("failed to parse item from anilist api")
				}
				apiItems = append(apiItems, respItem)
			}
			hasNextPage = result.Data.Page.PageInfo.HasNextPage
		} else {
			return nil, errors.New("unsupported api")
		}

		// map the data
		for _, item := range apiItems {
			item.TvdbId = idMap.GetByMalId(item.MalId)
			if item.TvdbId == 0 {
				log.Printf("MyAnimeList ID %d (%s) has no associated TVDB ID, skipping...\n", item.MalId, FullAnimeTitle(item.Title, item.TitleEng))
				continue
			}
			if usedTvdbIds[item.TvdbId] && opts.MergeSeasons {
				log.Printf("MyAnimeList ID %d (%s) is season of an already included anime, skipping...\n", item.MalId, FullAnimeTitle(item.Title, item.TitleEng))
				continue
			}
			if usedIds[item.MalId] && !opts.AllowDuplicates {
				log.Printf("MyAnimeList ID %d (%s) is a duplicate, skipping...\n", item.MalId, FullAnimeTitle(item.Title, item.TitleEng))
				continue
			}
			if slices.Contains(PermaSkipIds, strconv.Itoa(idMap.GetByMalId(item.MalId))) {
				log.Printf("MyAnimeList ID %d (%s) is set to always skip, skipping...\n", item.MalId, FullAnimeTitle(item.Title, item.TitleEng))
				continue
			}
			count++
			if count > opts.Limit {
				break
			}
			resp = append(resp, *item)
			usedIds[item.MalId] = true
			usedTvdbIds[item.TvdbId] = true
		}
		if count > opts.Limit {
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
