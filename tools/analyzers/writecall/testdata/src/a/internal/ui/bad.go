package ui

import "a/internal/store"

func bad() {
	w := store.NewWriter()                  // want `store\.NewWriter reaches a store-write entry point outside internal/store; go through internal/store's intent gateway \(Apply, Dispatch\)`
	_ = store.WriteMessage(w, "1")           // want `store\.WriteMessage reaches a store-write entry point outside internal/store; go through internal/store's intent gateway \(Apply, Dispatch\)`
	_ = store.Writer{}                       // want `store\.Writer constructs a store-write entry point outside internal/store; go through internal/store's intent gateway \(Apply, Dispatch\)`
}
