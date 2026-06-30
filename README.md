# RSS feed for the Richmond upon Thames Borough Council

The council website has [a news section](https://richmond.gov.uk/news) without an RSS or Atom feed. This fixes that.

## Usage

SQLite is optional. If you don't need persistence, you can run the service in-memory. Just omit the db path.

### With Docker

```sh
docker run -p 8080:8080 -e RSS_DB_PATH=/data/news.db -v rcrss:/data ghcr.io/dcelasun/richmond-council-rss:latest
```

### Manually

```sh
make build
./richmond-council-rss -db=/data/news.db
```

Now point your RSS reader to `http://localhost:8080/feed.xml`.

## Development

Build and run:

```sh
make build
./richmond-council-rss               # in-memory, listens on :8080
./richmond-council-rss -db ./news.db # persist to SQLite
```

The feed is served at `/feed.xml` (aliases: `/` and `/rss`). A health check is at `/healthz`.

```sh
curl http://localhost:8080/feed.xml
curl 'http://localhost:8080/feed.xml?page=2'
```

Run vet and tests:

```sh
make vet
make test
```

## Configuration

Environment variables take priority over CLI flags.

| Env | Flag | Default | Description |
|---|---|---|---|
| `RSS_LISTEN_ADDR` | `-listen` | `:8080` | HTTP listen address |
| `RSS_SCRAPE_INTERVAL` | `-interval` | `30m` | Scrape interval (Go duration) |
| `RSS_DB_PATH` | `-db` | _(empty)_ | SQLite file path; empty = in-memory |
| `RSS_MAX_ITEMS` | `-max-items` | `50` | Posts to backfill on first run |
| `RSS_PAGE_SIZE` | `-page-size` | `50` | Items per feed page |
| `RSS_SITE_URL` | `-site-url` | `https://richmond.gov.uk` | Base site URL (used to resolve relative links) |
| `RSS_FEED_URL` | `-feed-url` | `http://localhost:8080/feed.xml` | Public feed URL (for `atom:self` and paging links) |
| `RSS_USER_AGENT` | `-user-agent` | `richmond-council-rss/1.0 (…)` | Scraper User-Agent |