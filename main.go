package main

import (
	log "github.com/sirupsen/logrus"
	"os"
	// "github.com/robfig/cron/v3"
	"github.com/blevesearch/bleve"
	"net/http"
	"github.com/patrickmn/go-cache"
	"time"
	"io/ioutil"
	"encoding/json"
)

// The caching strategy used here has been discussed in
// https://carlosbecker.com/posts/golang-cache-interface

type SteamApp struct {
	AppID int64 `json:"appid"`
	Name string `json:"name"`
}

type AppListStruct struct {
	Apps []SteamApp `json:"apps"`
}

type ResponseStruct struct {
	Applist AppListStruct `json:"applist"`
}

type Client interface {
	GetSteamApps() ([]SteamApp, error)
}

func NewSteamClient() Client {
	return steamClient{}
}

type steamClient struct {}

func (steamClient) GetSteamApps() ([]SteamApp, error) {
	resp, _ := http.Get("https://api.steampowered.com/ISteamApps/GetAppList/v2/")
	responseData,err := ioutil.ReadAll(resp.Body)
	if err != nil {
    	log.Fatal(err)
	}
	var response ResponseStruct
	err = json.Unmarshal(responseData, &response)
	return response.Applist.Apps, nil
}


func NewCachedClient(client Client, cache *cache.Cache) Client {
	return cachedClient{
		client: client,
		cache:  cache,
	}
}

type cachedClient struct {
	client Client
	cache *cache.Cache
}

func (c cachedClient) GetSteamApps() ([]SteamApp, error) {
	cached, found := c.cache.Get("steam-apps")
	if found {
		return cached.([]SteamApp), nil
	}
	// call the underlying client
	live, err := c.client.GetSteamApps()
	c.cache.Set("steam-apps", live, cache.DefaultExpiration)
	return live, err
}

type cacheTestClient struct {
	result *[]SteamApp
}

func (f cacheTestClient) GetSteamApps() ([]SteamApp, error) {
	return *f.result, nil
}

func main() {
	// Set up file to log to

	file, err := os.OpenFile("info.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        log.Fatal(err)
    }

    defer file.Close()

	// Set logrus output to file
	// log.SetOutput(file)
	var cache = cache.New(cache.NoExpiration, 60*time.Minute)
	var cli = NewCachedClient(NewSteamClient(), cache)
	x, _ := cli.GetSteamApps()
	mapping := bleve.NewIndexMapping()
	index, _ := bleve.New("example.bleve", mapping)
	if err != nil {
		log.Error(err)
		return
	}
	index.Index("Name", x)

	// search for some text
	query := bleve.NewMatchQuery("swatch")
	search := bleve.NewSearchRequest(query)
	searchResults, err := index.Search(search)
	if err != nil {
		log.Error(err)
		return
	}
	log.Print(searchResults)
}