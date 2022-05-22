package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"time"
)

const (
	mupdatesSearchEndpoint = "https://api.mangaupdates.com/v1/series/search"
	muSeriesEndpoint       = "https://api.mangaupdates.com/v1/series/%d"
)

var (
	errNotEnoughResults = errors.New("not enough results")
)

type mupdatesSearch struct {
	TotalHits int `json:"total_hits"`
	Page      int `json:"page"`
	PerPage   int `json:"per_page"`
	Results   []struct {
		Record struct {
			SeriesID    int64  `json:"series_id"`
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			Image       struct {
				URL struct {
					Original string `json:"original"`
					Thumb    string `json:"thumb"`
				} `json:"url"`
				Height int `json:"height"`
				Width  int `json:"width"`
			} `json:"image"`
			Type           string  `json:"type"`
			Year           string  `json:"year"`
			BayesianRating float64 `json:"bayesian_rating"`
			RatingVotes    int     `json:"rating_votes"`
			Genres         []struct {
				Genre string `json:"genre"`
			} `json:"genres"`
			LastUpdated struct {
				Timestamp int    `json:"timestamp"`
				AsRfc3339 string `json:"as_rfc3339"`
				AsString  string `json:"as_string"`
			} `json:"last_updated"`
		} `json:"record"`
		HitTitle string `json:"hit_title"`
		Metadata struct {
			UserList struct {
				ListType interface{} `json:"list_type"`
				ListIcon interface{} `json:"list_icon"`
				Status   struct {
					Volume  interface{} `json:"volume"`
					Chapter interface{} `json:"chapter"`
				} `json:"status"`
			} `json:"user_list"`
			UserGenreHighlights []interface{} `json:"user_genre_highlights"`
		} `json:"metadata"`
	} `json:"results"`
}

type getSeriesResp struct {
	SeriesID   int64  `json:"series_id"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Associated []struct {
		Title string `json:"title"`
	} `json:"associated"`
	Description string `json:"description"`
	Image       struct {
		URL struct {
			Original string `json:"original"`
			Thumb    string `json:"thumb"`
		} `json:"url"`
		Height int `json:"height"`
		Width  int `json:"width"`
	} `json:"image"`
	Type           string  `json:"type"`
	Year           string  `json:"year"`
	BayesianRating float64 `json:"bayesian_rating"`
	RatingVotes    int     `json:"rating_votes"`
	Genres         []struct {
		Genre string `json:"genre"`
	} `json:"genres"`
	Categories []struct {
		SeriesID   int64  `json:"series_id"`
		Category   string `json:"category"`
		Votes      int    `json:"votes"`
		VotesPlus  int    `json:"votes_plus"`
		VotesMinus int    `json:"votes_minus"`
		AddedBy    int64  `json:"added_by"`
	} `json:"categories"`
	LatestChapter int    `json:"latest_chapter"`
	ForumID       int64  `json:"forum_id"`
	Status        string `json:"status"`
	Licensed      bool   `json:"licensed"`
	Completed     bool   `json:"completed"`
	Anime         struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"anime"`
	RelatedSeries []struct {
		RelationID            int    `json:"relation_id"`
		RelationType          string `json:"relation_type"`
		RelatedSeriesID       int64  `json:"related_series_id"`
		RelatedSeriesName     string `json:"related_series_name"`
		TriggeredByRelationID int    `json:"triggered_by_relation_id"`
	} `json:"related_series"`
	Authors []struct {
		Name     string `json:"name"`
		AuthorID int64  `json:"author_id"`
		Type     string `json:"type"`
	} `json:"authors"`
	Publishers []struct {
		PublisherName string `json:"publisher_name"`
		PublisherID   int64  `json:"publisher_id"`
		Type          string `json:"type"`
		Notes         string `json:"notes"`
	} `json:"publishers"`
	Publications []struct {
		PublicationName string `json:"publication_name"`
		PublisherName   string `json:"publisher_name"`
		PublisherID     int64  `json:"publisher_id"`
	} `json:"publications"`
	Recommendations []struct {
		SeriesName string `json:"series_name"`
		SeriesID   int64  `json:"series_id"`
		Weight     int    `json:"weight"`
	} `json:"recommendations"`
	CategoryRecommendations []struct {
		SeriesName string `json:"series_name"`
		SeriesID   int64  `json:"series_id"`
		Weight     int    `json:"weight"`
	} `json:"category_recommendations"`
	Rank struct {
		Position struct {
			Week        int `json:"week"`
			Month       int `json:"month"`
			ThreeMonths int `json:"three_months"`
			SixMonths   int `json:"six_months"`
			Year        int `json:"year"`
		} `json:"position"`
		OldPosition struct {
			Week        int `json:"week"`
			Month       int `json:"month"`
			ThreeMonths int `json:"three_months"`
			SixMonths   int `json:"six_months"`
			Year        int `json:"year"`
		} `json:"old_position"`
		Lists struct {
			Reading    int `json:"reading"`
			Wish       int `json:"wish"`
			Complete   int `json:"complete"`
			Unfinished int `json:"unfinished"`
			Custom     int `json:"custom"`
		} `json:"lists"`
	} `json:"rank"`
	LastUpdated struct {
		Timestamp int    `json:"timestamp"`
		AsRfc3339 string `json:"as_rfc3339"`
		AsString  string `json:"as_string"`
	} `json:"last_updated"`
}

func postMuSearch(name string) (string, int64, error) {
	v := url.Values{}
	v.Add("search", name)

	mus, err := post[mupdatesSearch](mupdatesSearchEndpoint, v)
	if err != nil {
		return "", -1, fmt.Errorf("error getting Manga Updates search endpoint: %w", err)
	}

	if len(mus.Results) < 1 {
		return "", -1, errNotEnoughResults
	}

	return mus.Results[0].Record.Title, mus.Results[0].Record.SeriesID, nil
}

func getmuSeries(id int64) ([]string, error) {
	resp, err := get[getSeriesResp](fmt.Sprintf(muSeriesEndpoint, id))
	if err != nil {
		return nil, fmt.Errorf("error retrieving series: %w", err)
	}

	var ret []string
	for _, publisher := range resp.Publishers {
		if publisher.Type == "English" {
			ret = append(ret, publisher.PublisherName)
		}
	}

	return ret, nil
}

func searchList(ctx context.Context, r io.Reader, w io.WriteCloser) error {
	defer w.Close()

	csvr := csv.NewReader(r)
	csvw := csv.NewWriter(w)
	records, err := csvr.ReadAll()
	if err != nil {
		return fmt.Errorf("error reading records from CSV: %w", err)
	}

	err = csvw.Write([]string{"title", "publishers"})
	if err != nil {
		return fmt.Errorf("error writing header: %w", err)
	}

	ticker := time.NewTicker(5 * time.Second)

	for _, record := range records {
		select {
		case <-ctx.Done():
			return errors.New("context cancelled")
		case <-ticker.C:
		}

		if len(record) != 1 {
			return fmt.Errorf("wanted one column, got %d", len(record))
		}
		log.Printf("searching for %q... ", record[0])
		name, id, err := postMuSearch(record[0])
		if err != nil {
			if errors.Is(err, errNotEnoughResults) || name != record[0] {
				csvw.Write([]string{record[0], "exact match not found"})
				continue
			}
			return fmt.Errorf("error searching manga %q: %w", record[0], err)
		}
		log.Printf("found! id: %d\n", id)
		pubs, err := getmuSeries(id)
		if err != nil {
			return fmt.Errorf("error getting manga %q with id %d: %w", record[0], id, err)
		}
		log.Print("\tpublishers: ")

		if len(pubs) < 1 {
			log.Print("none")
			err = csvw.Write([]string{record[0], ""})
			if err != nil {
				return fmt.Errorf("error writing CSV row: %w", err)
			}
			continue
		}

		log.Printf("%v", pubs)

		err = csvw.Write([]string{record[0], fmt.Sprintf("%v", pubs)})
		if err != nil {
			return fmt.Errorf("error writing CSV row: %w", err)
		}
	}

	csvw.Flush()
	err = csvw.Error()
	if err != nil {
		return fmt.Errorf("error flushing csv writer: %w", err)
	}

	return nil
}
