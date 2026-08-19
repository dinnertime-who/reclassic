package gutenberg

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	http     *http.Client
	ua       string
	interval time.Duration
	last     time.Time
}

func NewClient(ua string, interval time.Duration) *Client {
	return &Client{
		http:     &http.Client{Timeout: 2 * time.Minute},
		ua:       ua,
		interval: interval,
	}
}

func (c *Client) Get(url string) (body []byte, status int, err error) {
	c.wait()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read body: %w", err)
	}
	return body, resp.StatusCode, nil
}

func (c *Client) wait() {
	if c.last.IsZero() {
		c.last = time.Now()
		return
	}
	elapsed := time.Since(c.last)
	if elapsed < c.interval {
		time.Sleep(c.interval - elapsed)
	}
	c.last = time.Now()
}

func (c *Client) GetRetry5xx(url string, maxAttempts int) ([]byte, int, error) {
	var lastErr error
	var lastStatus int
	backoff := c.interval
	if backoff < time.Second {
		backoff = time.Second
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		body, status, err := c.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		lastStatus = status
		if status >= 500 && status <= 599 && attempt < maxAttempts {
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		return body, status, nil
	}
	if lastErr != nil {
		return nil, lastStatus, lastErr
	}
	return nil, lastStatus, fmt.Errorf("status %d after retries", lastStatus)
}
