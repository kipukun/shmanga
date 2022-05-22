package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	c = &http.Client{
		Timeout: 10 * time.Second,
	}
)

func get[T any](url string) (T, error) {
	var zero T
	resp, err := c.Get(url)
	if err != nil {
		return zero, fmt.Errorf("error getting: %w", err)
	}
	defer resp.Body.Close()

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("error reading all from response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("did not get HTTP 200, got HTTP %d with body %s", resp.StatusCode, string(bs))
	}

	var ret T
	err = json.Unmarshal(bs, &ret)
	if err != nil {
		return zero, fmt.Errorf("error unmarshalling JSON HTTP response: %w", err)
	}

	return ret, nil
}

func post[T any](url string, v url.Values) (T, error) {
	var zero T
	resp, err := c.PostForm(url, v)
	if err != nil {
		return zero, fmt.Errorf("error posting: %w", err)
	}
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("error reading all from response: %w", err)
	}

	var ret T
	err = json.Unmarshal(bs, &ret)
	if err != nil {
		return zero, fmt.Errorf("error unmarshalling JSON HTTP response: %w", err)
	}

	return ret, nil
}

func path(ss ...string) string {
	var b strings.Builder
	for _, s := range ss {
		b.WriteString(s)
		b.WriteByte('/')
	}
	return strings.TrimSuffix(b.String(), "/")
}
