package main

import (
	"fmt"
	"testing"
)

func TestPostSearch(t *testing.T) {
	name := "New Game!"
	name, id, err := postMuSearch(name)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(id)
}
