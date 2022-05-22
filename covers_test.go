package main

import (
	"fmt"
	"testing"
)

func TestCovers(t *testing.T) {
	test := "Komi-san wa Komyushou Desu."
	title, uuid, err := searchManga(test)
	if err != nil {
		t.Fatal(err)
	}

	if title != test {
		t.Fatal("inexact title")
	}

	covers, err := getCovers(uuid)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(covers)
}
