package main

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"regexp"
	"sync"
	"time"
)

const (
	coverEndpointFmt  = "https://api.mangadex.org/cover?limit=100&order[volume]=asc&manga[]=%s"
	searchEndpointFmt = "https://api.mangadex.org/manga?title=%s"
	coversImgFmt      = "https://uploads.mangadex.org/covers/%s/%s"
)

var (
	invalidChars = regexp.MustCompile(`<>:"/\|\?*`)
)

type coversSearchResp struct {
	Result   string `json:"result"`
	Response string `json:"response"`
	Data     []struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Title struct {
				En string `json:"en"`
			} `json:"title"`
		} `json:"attributes"`
	} `json:"data"`
}

type coversResp struct {
	Result   string `json:"result"`
	Response string `json:"response"`
	Data     []struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Description string    `json:"description"`
			Volume      string    `json:"volume"`
			FileName    string    `json:"fileName"`
			Locale      string    `json:"locale"`
			CreatedAt   time.Time `json:"createdAt"`
			UpdatedAt   time.Time `json:"updatedAt"`
			Version     int       `json:"version"`
		} `json:"attributes"`
		Relationships []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"relationships"`
	} `json:"data"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

func searchManga(name string) (title string, uuid string, err error) {
	enc := url.QueryEscape(name)
	searchResp, err := get[coversSearchResp](fmt.Sprintf(searchEndpointFmt, enc))
	if err != nil {
		return "", "", fmt.Errorf("error searching manga on mangadex: %w", err)
	}

	if len(searchResp.Data) < 1 {
		return "", "", errNotEnoughResults
	}

	return searchResp.Data[0].Attributes.Title.En, searchResp.Data[0].ID, nil
}

func getCovers(uuid string) (map[string]string, error) {
	enc := url.QueryEscape(uuid)
	cr, err := get[coversResp](fmt.Sprintf(coverEndpointFmt, enc))
	if err != nil {
		return nil, err
	}

	covers := make(map[string]string)

	for _, link := range cr.Data {
		covers[link.Attributes.Volume] = link.Attributes.FileName
	}

	return covers, nil
}

func createCoverZips(ctx context.Context, r io.Reader, w io.WriteCloser, dir string) error {
	csvr := csv.NewReader(r)
	csvw := csv.NewWriter(w)

	recs, err := csvr.ReadAll()
	if err != nil {
		return fmt.Errorf("error reading csv input: %w", err)
	}

	dir = invalidChars.ReplaceAllString(dir, "_")

	ticker := time.NewTicker(time.Second)

	err = os.Mkdir(dir, 0750)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("error creating output dir: %w", err)
	}

	log.Println("created output directory", dir)

	var wg sync.WaitGroup
	errs := make(chan error)
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, rec := range recs {

		select {
		case <-wctx.Done():
			return errors.New("context cancelled")
		case <-ticker.C:
		}

		if len(rec) != 1 {
			return fmt.Errorf("expected row of length 1, got %d", len(rec))
		}

		title, uuid, err := searchManga(rec[0])
		if err != nil {
			return fmt.Errorf("error searching manga on mangadex: %w", err)
		}

		if title != rec[0] {
			log.Printf("%q != %q, continuing", title, rec[0])
			err = csvw.Write([]string{rec[0], ""})
			if err != nil {
				return fmt.Errorf("error writing csv: %w", err)
			}
			csvw.Flush()
			continue
		}

		covers, err := getCovers(uuid)
		if err != nil {
			return fmt.Errorf("error getting covers from mangadex: %w", err)
		}

		cleaned := invalidChars.ReplaceAllString(rec[0], "_")

		err = os.Mkdir(path(dir, cleaned), 0750)
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("error creating output dir: %w", err)
		}

		sem := make(chan struct{}, 5)

		log.Printf("getting covers for: %q\n", title)

		for volume, cover := range covers {

			if volume == "" {
				volume = "No Volume"
			} else {
				volume = fmt.Sprintf("Volume %s", volume)
			}

			fname := fmt.Sprintf("%s - %s.zip", title, volume)
			p := path(dir, cleaned, fname)

			if _, err := os.Stat(p); err == nil {
				continue
			}

			wg.Add(1)

			go func(s string) {
				defer wg.Done()

				select {
				case <-wctx.Done():
					return
				case sem <- struct{}{}:
				}

				u := fmt.Sprintf(coversImgFmt, uuid, s)
				log.Println("getting", u)

				resp, err := c.Get(u)
				if err != nil {
					errs <- err
					return
				}
				defer resp.Body.Close()

				of, err := os.Create(p)
				if err != nil {
					errs <- err
					return
				}
				defer of.Close()

				zw := zip.NewWriter(bufio.NewWriter(of))
				zf, err := zw.Create("cover")
				if err != nil {
					errs <- err
					return
				}

				_, err = io.Copy(zf, resp.Body)
				if err != nil {
					errs <- err
					return
				}

				err = zw.Close()
				if err != nil {
					errs <- err
					return
				}

				log.Println("created", p)

				<-sem
			}(cover)
		}

	}

	wait := make(chan struct{})

	go func() {
		wg.Wait()
		wait <- struct{}{}
	}()

	select {
	case err = <-errs:
		cancel()
		return fmt.Errorf("error creating cover zip: %w", err)
	case <-wait:
	}

	csvw.Flush()
	err = csvw.Error()
	if err != nil {
		return fmt.Errorf("error flushing csv writer: %w", err)
	}

	return nil
}
