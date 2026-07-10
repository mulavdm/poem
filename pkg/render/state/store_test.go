package state

import "testing"

type interactionState struct {
	Index int
	Open  bool
}

func TestStoreTypedReplacementAndRevision(t *testing.T) {
	store := NewStore()
	first := StoreValue(store, "search", interactionState{Index: 2, Open: true})
	got, ok := Load[interactionState](store, "search")
	if !ok || got.Index != 2 || !got.Open {
		t.Fatalf("unexpected typed state: %+v %v", got, ok)
	}
	second := StoreValue(store, "search", interactionState{Index: 3})
	if second <= first || store.Revision() != second {
		t.Fatalf("revision did not advance: %d -> %d", first, second)
	}
	if _, ok := Load[string](store, "search"); ok {
		t.Fatal("mismatched type should not load")
	}
}

func TestStoreDeletePrefix(t *testing.T) {
	store := NewStore()
	store.Set("dialog/field", 1)
	store.Set("dialog/list", 2)
	store.Set("other", 3)
	if removed := store.DeletePrefix("dialog/"); removed != 2 || store.Len() != 1 {
		t.Fatalf("removed=%d len=%d", removed, store.Len())
	}
}

func BenchmarkStoreTypedRead(b *testing.B) {
	store := NewStore()
	StoreValue(store, "tree", interactionState{Index: 4, Open: true})
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if value, ok := Load[interactionState](store, "tree"); !ok || value.Index != 4 {
			b.Fatal("missing value")
		}
	}
}
