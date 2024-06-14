package repo

import (
	"io"
	"net/http"
	"net/url"

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
	}
	return &MikanClient{
		client: client,
	}
}

func (c *MikanClient) Call(rssUrl string) ([]byte, error) {
	req, _ := http.NewRequest("GET", rssUrl, nil)
	resp, err := c.client.Do(req)
	if err != nil {
		log.Errorf("[mikan] mikan client call err: %v", err)
		return []byte{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("[mikan] read from mikan resp err: %v", err)
		return []byte{}, err
	}
	return data, nil
}
