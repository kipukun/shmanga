package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
)

func createIO(in, out string) (io.Reader, io.WriteCloser, error) {
	var r io.Reader
	if in == "" {
		r = os.Stdin
	} else {
		f, err := os.Open(in)
		if err != nil {
			return nil, nil, fmt.Errorf("error opening file %q: %w", in, err)
		}
		r = f
	}
	var w io.WriteCloser
	if out == "" {
		w = os.Stdout
	} else {
		wf, err := os.Create(out)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating file %q: %w", out, err)
		}
		w = wf
	}
	return r, w, nil
}

func main() {
	publisherCmd := flag.NewFlagSet("publishers", flag.ExitOnError)
	publisherFile := publisherCmd.String("f", "", "CSV list of manga titles to search for, leave empty for stdin")
	publisherOutput := publisherCmd.String("o", "", "location of output file, leave empty for stdout")

	coversCmd := flag.NewFlagSet("covers", flag.ExitOnError)
	coversFile := coversCmd.String("f", "", "CSV list of manga titles to search for, leave empty for stdin")
	coversOutput := coversCmd.String("o", "", "location of not found list, leave empty for stdout")
	coversID := coversCmd.String("id", "", "download covers for this specific manga id")
	coversDir := coversCmd.String("dir", "", "location to output directories of zip files of covers")

	if len(os.Args) < 2 {
		fmt.Println("expected publishers or covers command")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	switch os.Args[1] {
	case "publishers":
		publisherCmd.Parse(os.Args[2:])

		r, w, err := createIO(*publisherFile, *publisherOutput)
		if err != nil {
			log.Fatalln(err)
			return
		}

		err = searchList(ctx, r, w)
		if err != nil {
			log.Fatalln(err)
			return
		}

	case "covers":
		coversCmd.Parse(os.Args[2:])

		if *coversID != "" {
			fmt.Println(*coversID)
			return
		}

		r, w, err := createIO(*coversFile, *coversOutput)
		if err != nil {
			log.Fatalln(err)
			return
		}

		err = createCoverZips(ctx, r, w, *coversDir)
		if err != nil {
			log.Fatalln(err)
			return
		}
	default:
		fmt.Println("expected publishers or covers command")
		return
	}
}
