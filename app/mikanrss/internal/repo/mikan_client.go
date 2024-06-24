package repo

import (
	"a1in-bot/app/mikanrss/internal/model"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type MikanClient struct {
	client *http.Client
}

func NewMikanClient() *MikanClient {
	proxyURL, _ := url.Parse("http://127.0.0.1:7890")
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 5 * time.Second,
	}
	return &MikanClient{
		client: client,
	}
}

func (c *MikanClient) GetRSSFeed(rssUrl string) (*model.MikanRSSFeed, error) {
	req, _ := http.NewRequest("GET", rssUrl, nil)
	resp, err := c.client.Do(req)
	if err != nil {
		log.Errorf("[mikan] mikan client call err: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("[mikan] read from mikan resp err: %v", err)
		return nil, err
	}
	feed := &model.MikanRSSFeed{}
	err = xml.Unmarshal(data, feed)
	if err != nil {
		log.Errorf("[mikan] unmarshal rss feed err: %v", err)
		return nil, err
	}
	return feed, nil
}
