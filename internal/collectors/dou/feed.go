package dou

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
)

// feedItem is one RSS <item> before detail enrichment.
type feedItem struct {
	Title       string
	Link        string
	GUID        string
	PubDate     time.Time // zero if missing/unparsed
	Description string    // RSS teaser; fallback if detail fails
}

type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

func fetchFeedBytes(ctx context.Context, client *http.Client, feedURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	utils.SetCollectorUserAgent(req)
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		return nil, fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(snippet))
	}
	return body, nil
}

func feedItemsFromRSS(body []byte) ([]feedItem, error) {
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("decode RSS: %w", err)
	}
	out := make([]feedItem, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		fi := newFeedItem(item.Title, item.Link, item.GUID, item.PubDate, item.Description)
		if fi.Link == "" && fi.GUID == "" {
			continue
		}
		out = append(out, fi)
	}
	return out, nil
}

func newFeedItem(title, link, guid, pubDate, description string) feedItem {
	return feedItem{
		Title:       strings.TrimSpace(title),
		Link:        strings.TrimSpace(link),
		GUID:        strings.TrimSpace(guid),
		PubDate:     parseRSSPubDate(pubDate),
		Description: strings.TrimSpace(description),
	}
}

func parseRSSPubDate(raw string) time.Time {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
