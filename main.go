package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
)

type bufWriteCloser struct {
	*bufio.Writer
}

func (bwc *bufWriteCloser) Close() error {
	return nil
}

func main() {
	publisherCmd := flag.NewFlagSet("publishers", flag.ExitOnError)
	publisherFile := publisherCmd.String("f", "", "CSV list of manga titles to search for, leave empty for stdin")
	publisherOutput := publisherCmd.String("o", "", "location of output file, leave empty for stdout")

	if len(os.Args) < 2 {
		fmt.Println("expected publishers or covers command")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "publishers":
		publisherCmd.Parse(os.Args[2:])
		var r io.Reader
		if *publisherFile == "" {
			r = os.Stdin
		} else {
			f, err := os.Open(*publisherFile)
			if err != nil {
				log.Fatalf("error opening file: %q", *publisherFile)
			}
			r = f
		}
		var w io.WriteCloser
		if *publisherOutput == "" {
			w = os.Stdout
		} else {
			wf, err := os.Create(*publisherOutput)
			if err != nil {
				log.Fatalf("error opening file: %q", *publisherFile)
			}
			w = wf
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		err := searchList(ctx, r, w)
		if err != nil {
			log.Fatalln(err)
		}
	default:
		fmt.Println("expected publishers or covers command")
		os.Exit(1)
	}
}
