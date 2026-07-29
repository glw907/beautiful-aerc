// Package main stands in for poplar's cmd/poplar, the startup path
// that opens the store and starts the writer.
package main

import "a/internal/store"

func main() {
	w, err := store.Open("poplar.db", store.DefaultWriterConfig())
	if err != nil {
		return
	}
	defer w.Close()
}
