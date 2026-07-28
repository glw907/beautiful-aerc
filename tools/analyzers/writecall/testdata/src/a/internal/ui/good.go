package ui

import "a/internal/store"

func good() {
	_ = store.Apply(store.Intent{Kind: "send"})
	_ = store.Dispatch(store.Intent{Kind: "delete"})
}
