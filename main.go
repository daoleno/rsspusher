package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/spf13/viper"
)

type config struct {
	WebHookURL string   `mapstructure:"webhook_url"`
	FeedURLs   []string `mapstructure:"feed_urls"`
	DateSince  string   `mapstructure:"date_since"`
}

func main() {
	// load config
	conf := loadConf()

	// fetch, sort and filter feeds
	items := fetchFeedItems(conf.FeedURLs)
	sortItems(items)
	filteredItems := filterItems(items, conf.DateSince)

	// send webhooks
	for _, item := range filteredItems {
		if err := webHookSend(conf.WebHookURL, item.Title, item.Link); err != nil {
			fmt.Println(err)
		}
	}

	// refresh date_since
	if len(filteredItems) > 0 {
		conf.DateSince = filteredItems[0].PublishedParsed.Format(time.RFC3339)
		viper.Set("date_since", conf.DateSince)
		if err := viper.WriteConfig(); err != nil {
			fmt.Println(err)
		}
	}

}

func loadConf() *config {
	var conf config
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.SetConfigType("toml")
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}
	if err := viper.Unmarshal(&conf); err != nil {
		panic(err)
	}
	return &conf

}

func fetchFeedItems(urls []string) []*gofeed.Item {
	fp := gofeed.NewParser()
	items := make(chan []*gofeed.Item)
	for _, url := range urls {
		go func(url string) {
			feed, err := fp.ParseURL(url)
			if err != nil {
				fmt.Println(err)
				items <- nil
				return
			}
			items <- feed.Items
		}(url)
	}

	var allItems []*gofeed.Item
	for range urls {
		allItems = append(allItems, <-items...)
	}

	return allItems
}

func sortItems(items []*gofeed.Item) {
	// sort items by PublishedParsed
	sort.SliceStable(items, func(i int, j int) bool {
		if items[i].PublishedParsed == nil {
			return false
		}
		return items[i].PublishedParsed.After(*items[j].PublishedParsed)
	})
}

func filterItems(items []*gofeed.Item, dateSince string) []*gofeed.Item {
	// parse date_since
	dateSinceParsed, err := time.Parse(time.RFC3339, dateSince)
	if err != nil {
		panic(err)
	}
	var filteredItems []*gofeed.Item
	for _, item := range items {
		if item.PublishedParsed == nil || !item.PublishedParsed.After(dateSinceParsed) {
			continue
		}
		filteredItems = append(filteredItems, item)
	}
	return filteredItems

}

func webHookSend(url string, title string, link string) error {
	_, err := http.Post(url, "application/json", strings.NewReader(fmt.Sprintf(`{"content":"%s\n%s"}`, title, link)))
	if err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}
