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
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kipukun/shmanga/group"
)

const (
	coverEndpointFmt  = "https://api.mangadex.org/cover?limit=100&order[volume]=asc&manga[]=%s"
	searchEndpointFmt = "https://api.mangadex.org/manga?title=%s"
	mangaEndpoint     = "https://api.mangadex.org/manga/%s"
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

type mangaResp struct {
	Result   string `json:"result"`
	Response string `json:"response"`
	Data     struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Title struct {
				En string `json:"en"`
			} `json:"title"`
			AltTitles []struct {
				JaRo string `json:"ja-ro,omitempty"`
				En   string `json:"en,omitempty"`
				Ru   string `json:"ru,omitempty"`
				Th   string `json:"th,omitempty"`
				Ja   string `json:"ja,omitempty"`
				Zh   string `json:"zh,omitempty"`
				Ko   string `json:"ko,omitempty"`
				Ar   string `json:"ar,omitempty"`
			} `json:"altTitles"`
			Description struct {
				En   string `json:"en"`
				Pl   string `json:"pl"`
				PtBr string `json:"pt-br"`
			} `json:"description"`
			IsLocked bool `json:"isLocked"`
			Links    struct {
				Al    string `json:"al"`
				Ap    string `json:"ap"`
				Bw    string `json:"bw"`
				Kt    string `json:"kt"`
				Mu    string `json:"mu"`
				Amz   string `json:"amz"`
				Cdj   string `json:"cdj"`
				Ebj   string `json:"ebj"`
				Mal   string `json:"mal"`
				Raw   string `json:"raw"`
				Engtl string `json:"engtl"`
			} `json:"links"`
			OriginalLanguage       string `json:"originalLanguage"`
			LastVolume             string `json:"lastVolume"`
			LastChapter            string `json:"lastChapter"`
			PublicationDemographic string `json:"publicationDemographic"`
			Status                 string `json:"status"`
			Year                   int    `json:"year"`
			ContentRating          string `json:"contentRating"`
			Tags                   []struct {
				ID         string `json:"id"`
				Type       string `json:"type"`
				Attributes struct {
					Name struct {
						En string `json:"en"`
					} `json:"name"`
					Description []interface{} `json:"description"`
					Group       string        `json:"group"`
					Version     int           `json:"version"`
				} `json:"attributes"`
				Relationships []interface{} `json:"relationships"`
			} `json:"tags"`
			State                          string        `json:"state"`
			ChapterNumbersResetOnNewVolume bool          `json:"chapterNumbersResetOnNewVolume"`
			CreatedAt                      time.Time     `json:"createdAt"`
			UpdatedAt                      time.Time     `json:"updatedAt"`
			Version                        int           `json:"version"`
			AvailableTranslatedLanguages   []interface{} `json:"availableTranslatedLanguages"`
		} `json:"attributes"`
		Relationships []struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Related string `json:"related,omitempty"`
		} `json:"relationships"`
	} `json:"data"`
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

func getMangaTitleById(uuid string) (string, error) {
	resp, err := get[mangaResp](fmt.Sprintf(mangaEndpoint, uuid))
	if err != nil {
		return "", err
	}

	return resp.Data.Attributes.Title.En, nil
}

func createFile(u, p string) error {
	split := strings.Split(u, ".")
	if len(split) != 4 {
		return fmt.Errorf("malformed url: %q", u)
	}

	ext := split[3]

	resp, err := c.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	of, err := os.Create(p)
	if err != nil {
		return err
	}
	defer of.Close()

	zw := zip.NewWriter(bufio.NewWriter(of))
	zf, err := zw.Create("cover." + ext)
	if err != nil {
		return err
	}

	_, err = io.Copy(zf, resp.Body)
	if err != nil {
		return err
	}

	err = zw.Close()
	if err != nil {
		return err
	}

	return nil
}

type job struct {
	dir, uuid, title string
}

func createFileFromJob(ctx context.Context, j job) error {
	covers, err := getCovers(j.uuid)
	if err != nil {
		return fmt.Errorf("error getting covers from mangadex: %w", err)
	}

	err = os.Mkdir(j.dir, 0750)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("error creating output dir: %w", err)
	}

	g, ctx := group.WithContext(ctx)
	g.Limit(5)

	for volume, cover := range covers {

		if volume == "" {
			volume = "No Volume"
		} else {
			volume = fmt.Sprintf("Volume %s", volume)
		}

		fname := fmt.Sprintf("%s - %s.zip", j.title, volume)
		p := filepath.Join(j.dir, fname)

		if _, err := os.Stat(p); err == nil {
			continue
		}

		u := fmt.Sprintf(coversImgFmt, j.uuid, cover)

		g.Do(ctx, func() error {
			err := createFile(u, p)
			if err != nil {
				return err
			}
			log.Println("created", p)
			return nil
		})

	}

	err = g.Wait(ctx)
	if err != nil {
		return err
	}

	return nil
}

func createCoversFromIds(ctx context.Context, s string, dir string) error {
	uuids := strings.Split(s, ",")
	dir = invalidChars.ReplaceAllString(dir, "_")

	err := os.Mkdir(dir, 0750)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("error creating output dir: %w", err)
	}
	log.Println("created output directory", dir)

	g, ctx := group.WithContext(ctx)

	for _, uuid := range uuids {
		title, err := getMangaTitleById(uuid)
		if err != nil {
			return err
		}

		j := job{
			title: title,
			uuid:  uuid,
			dir:   filepath.Join(dir, title),
		}

		g.Do(ctx, func() error {
			err := createFileFromJob(ctx, j)
			if err != nil {
				return err
			}
			return nil
		})
	}

	err = g.Wait(ctx)
	if err != nil {
		return err
	}

	return nil
}

func createCoverZips(ctx context.Context, r io.Reader, w io.WriteCloser, dir string) error {
	csvr := csv.NewReader(r)
	csvw := csv.NewWriter(w)

	recs, err := csvr.ReadAll()
	if err != nil {
		return fmt.Errorf("error reading csv input: %w", err)
	}

	dir = invalidChars.ReplaceAllString(dir, "_")

	err = os.Mkdir(dir, 0750)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("error creating output dir: %w", err)
	}

	log.Println("created output directory", dir)

	g, ctx := group.WithContext(ctx)

	for _, rec := range recs {

		if len(rec) != 1 {
			return fmt.Errorf("expected row of length 1, got %d", len(rec))
		}

		title, uuid, err := searchManga(rec[0])
		if err != nil {
			if errors.Is(err, errNotEnoughResults) {
				err = csvw.Write([]string{rec[0], ""})
				if err != nil {
					return fmt.Errorf("error writing csv: %w", err)
				}
				csvw.Flush()
				continue
			}
			return fmt.Errorf("error searching manga on mangadex: %w", err)
		}

		if title != rec[0] {
			log.Printf("%q != %q, continuing", title, rec[0])
			err = csvw.Write([]string{rec[0], ""})
			if err != nil {
				return fmt.Errorf("error writing csv: %w", err)
			}
			continue
		}

		log.Printf("getting covers for: %q\n", title)

		cleanedTitle := invalidChars.ReplaceAllString(rec[0], "_")

		err = os.Mkdir(dir, 0750)
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("error creating output dir: %w", err)
		}

		j := job{
			title: cleanedTitle,
			dir:   filepath.Join(dir, cleanedTitle),
			uuid:  uuid,
		}

		g.Do(ctx, func() error {
			err := createFileFromJob(ctx, j)
			if err != nil {
				return err
			}
			return nil
		})
	}

	err = g.Wait(ctx)
	if err != nil {
		return err
	}

	csvw.Flush()
	err = csvw.Error()
	if err != nil {
		return fmt.Errorf("error flushing csv writer: %w", err)
	}

	return nil
}
