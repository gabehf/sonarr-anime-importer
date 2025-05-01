# sonarr-anime-importer
Easily create import lists in sonarr with MyAnimeList or AniList queries!

This is basically a wrapper for [Jikan](https://jikan.moe) and the AniList API that maps IDs to a list with TVDB IDs so that Sonarr can import the results.

**Until v1.0.0, breaking changes can happen at any time. Multiple have happened already! Be wary updating.**

**The "limit" parameter is required for all requests!**

Pulls MyAnimeList, AniList, and TVDB ID associations from https://raw.githubusercontent.com/Kometa-Team/Anime-IDs/master/anime_ids.json.

## Supported Requests
### GET /v1/mal/anime
Searches anime from MyAnimeList

See https://docs.api.jikan.moe/#tag/anime/operation/getAnimeSearch for parameters.

Additional parameters supported:
- `allow_duplicates`: skips de-duplication of results
- `merge_seasons`: skips seasons of already included shows

Example request:
```bash
# fetches the top 10 most popular currently airing tv anime
curl "http://localhost:3333/v1/mal/anime?type=tv&status=airing&order_by=popularity&sort=asc&limit=10"
```
### GET /v1/anilist/anime
Searches anime from AniList

Parameters:
- isAdult: Boolean
- search: String
- format: [[MediaFormat]](https://studio.apollographql.com/sandbox/schema/reference/enums/MediaFormat)
- status: [MediaStatus](https://studio.apollographql.com/sandbox/schema/reference/enums/MediaStatus)
- countryOfOrigin: [CountryCode](https://studio.apollographql.com/sandbox/schema/reference/scalars/CountryCode)
- season: [MediaSeason](https://studio.apollographql.com/sandbox/schema/reference/enums/MediaSeason)
- seasonYear: Int
- year: String
- onList: Boolean
- yearLesser: [FuzzyDateInt](https://studio.apollographql.com/sandbox/schema/reference/scalars/FuzzyDateInt)
- yearGreater: [FuzzyDateInt](https://studio.apollographql.com/sandbox/schema/reference/scalars/FuzzyDateInt)
- averageScoreGreater: Int
- averageScoreLesser: Int
- genres: [String]
- excludedGenres: [String]
- tags: [String]
- excludedTags: [String]
- minimumTagRank: Int
- sort: [[MediaSort]](https://studio.apollographql.com/sandbox/schema/reference/enums/MediaSort)
- limit: Int
- allowDuplicates: Boolean
- mergeSeasons: Boolean

Example request:
```bash
# fetch the top 20, non-adult trending anime that are either TV or ONA and are made in Japan after 2020
curl "http://localhost:3333/v1/anilist/anime?format=TV,ONA&sort=TRENDING_DESC&isAdult=false&countryOfOrigin=JP&yearGreater=20200000&limit=20"
```

## Environment
One configuration environment variable is supported:
- `ALWAYS_SKIP_TVDB_IDS`: Comma-separated list of TVDB IDs to always skip. These do not count towards the return limit.

## Docker Compose
```yaml
services:
  sonarr-anime-importer:
    image: gabehf/sonarr-anime-importer:latest
    container_name: sonarr-anime-importer
    ports:
      - 3333:3333
    environment:
      - ALWAYS_SKIP_TVDB_IDS=12345,67890 # Comma-separated
    restart: unless-stopped

```

# TODO
- [x] Only do "a.k.a." when logging if the anime has different romanized and english titles
- [ ] Prevent spamming calls when few/no IDs are mapped to TVDB

# Albums that fueled development
| Album                   | Artist                          |
|-------------------------|---------------------------------|
| ZOO!!                   | Necry Talkie (ネクライトーキー)    |
| FREAK                   | Necry Talkie (ネクライトーキー)    |
| Expert In A Dying Field | The Beths                       |
| Vivid                   | ADOY                            |
| CHUU                    | Strawberry Rush                 |
| MIMI                    | Hug (feat. HATSUNE MIKU & KAFU) |
