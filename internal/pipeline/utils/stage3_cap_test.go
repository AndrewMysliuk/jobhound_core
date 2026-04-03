package utils

import "testing"

func TestMaxStage3JobsPerPipelineRunExecution(t *testing.T) {
	if MaxStage3JobsPerPipelineRunExecution != 20 {
		t.Fatalf("cap N must be 20 per 008 default, got %d", MaxStage3JobsPerPipelineRunExecution)
	}
}

func TestSelectStage3JobIDs(t *testing.T) {
	t.Run("nil and empty", func(t *testing.T) {
		if got := SelectStage3JobIDs(nil, nil, 0); got != nil {
			t.Fatalf("nil candidates: got %#v want nil", got)
		}
		if got := SelectStage3JobIDs([]string{}, nil, 0); got != nil {
			t.Fatalf("empty candidates: got %#v want nil", got)
		}
	})

	t.Run("explicit cap two", func(t *testing.T) {
		in := []string{"a", "b", "c", "d"}
		got := SelectStage3JobIDs(in, nil, 2)
		want := []string{"a", "b"}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	})

	t.Run("dedup and order", func(t *testing.T) {
		in := []string{"a", "b", "a", "c", "d", "e", "f"}
		got := SelectStage3JobIDs(in, nil, 0)
		want := []string{"a", "b", "c", "d", "e", "f"}
		if len(got) != len(want) {
			t.Fatalf("len %d want %d: %#v", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("idx %d: got %q want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("skip empty strings", func(t *testing.T) {
		in := []string{"", "x", "", "y"}
		got := SelectStage3JobIDs(in, nil, 0)
		if len(got) != 2 || got[0] != "x" || got[1] != "y" {
			t.Fatalf("got %#v want [x y]", got)
		}
	})

	t.Run("exclude", func(t *testing.T) {
		in := []string{"a", "b", "c", "d", "e"}
		ex := map[string]struct{}{"b": {}, "d": {}}
		got := SelectStage3JobIDs(in, ex, 0)
		want := []string{"a", "c", "e"}
		if len(got) != len(want) {
			t.Fatalf("got %#v want %#v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("idx %d: got %q want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("exclude shrinks below cap", func(t *testing.T) {
		in := []string{"a", "b", "c", "d", "e", "f"}
		ex := map[string]struct{}{"a": {}, "b": {}, "c": {}, "d": {}, "e": {}}
		got := SelectStage3JobIDs(in, ex, 0)
		if len(got) != 1 || got[0] != "f" {
			t.Fatalf("got %#v want [f]", got)
		}
	})

	t.Run("idempotent exclude map", func(t *testing.T) {
		in := []string{"a", "b", "c"}
		first := SelectStage3JobIDs(in, nil, 0)
		ex := map[string]struct{}{}
		for _, id := range first {
			ex[id] = struct{}{}
		}
		second := SelectStage3JobIDs(in, ex, 0)
		if len(second) != 0 {
			t.Fatalf("after excluding first batch, want empty, got %#v", second)
		}
	})

	t.Run("nil vs empty exclude", func(t *testing.T) {
		in := []string{"a", "b", "c"}
		a := SelectStage3JobIDs(in, nil, 0)
		b := SelectStage3JobIDs(in, map[string]struct{}{}, 0)
		if len(a) != len(b) {
			t.Fatalf("len mismatch %d vs %d", len(a), len(b))
		}
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("idx %d: %q vs %q", i, a[i], b[i])
			}
		}
	})
}
