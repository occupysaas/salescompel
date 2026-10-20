package externalid

import (
	"fmt"
	"reflect"
	"testing"
)

func keyPointer(value string) *string { return &value }

func TestFindCollisions(t *testing.T) {
	records := []Record{
		{"a", "2", keyPointer("p-AbC")}, {"a", "1", keyPointer("p-abc")},
		{"a", "4", keyPointer("p-é")}, {"a", "3", keyPointer("p-e\u0301")},
		{"a", "6", keyPointer("p-ß")}, {"a", "5", keyPointer("p-ss")},
		{"b", "2", keyPointer("p-AbC")}, {"b", "1", keyPointer("p-abc")},
		{"a", "7", keyPointer("p-I")}, {"a", "8", keyPointer("p-ı")},
		{"a", "9", keyPointer("")}, {"a", "10", keyPointer("")},
		{"a", "11", nil}, {"a", "12", nil},
	}
	want := []Collision{
		{"a", "", []string{"10", "9"}},
		{"a", "p-abc", []string{"1", "2"}},
		{"a", "p-ss", []string{"5", "6"}},
		{"a", "p-é", []string{"3", "4"}},
		{"b", "p-abc", []string{"1", "2"}},
	}
	got, err := FindCollisions(records)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, %v; want %#v", got, err, want)
	}
	t.Run("no collisions returns empty slice", func(t *testing.T) {
		for _, records := range [][]Record{nil, {}, {{"a", "1", nil}}, {{"a", "1", keyPointer("same")}, {"b", "1", keyPointer("same")}}} {
			got, err := FindCollisions(records)
			if err != nil || got == nil || len(got) != 0 {
				t.Fatalf("got %#v, %v", got, err)
			}
		}
	})
	t.Run("structural tuples", func(t *testing.T) {
		// Concatenation with a unit-separator would merge these distinct groups.
		records := []Record{
			{"a\x1fb", "1", keyPointer("c")}, {"a", "1", keyPointer("b\x1fc")},
			{"a\x1fb", "2", keyPointer("C")}, {"a", "2", keyPointer("B\x1fC")},
			// These identities also collide under delimiter concatenation.
			{"x\x1fy", "z", nil}, {"x", "y\x1fz", nil},
		}
		want := []Collision{{"a", "b\x1fc", []string{"1", "2"}}, {"a\x1fb", "c", []string{"1", "2"}}}
		got, err := FindCollisions(records)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, %v; want %#v", got, err, want)
		}
	})
}

func TestFindCollisionsInvalidInput(t *testing.T) {
	cases := []struct {
		name    string
		records []Record
	}{
		{"empty tenant", []Record{{"", "3", nil}}},
		{"empty ID", []Record{{"a", "", nil}}},
		{"duplicate ID", []Record{{"a", "3", keyPointer("x")}, {"a", "3", keyPointer("y")}}},
		{"duplicate null ID", []Record{{"a", "3", nil}, {"a", "3", nil}}},
		{"duplicate mixed null ID", []Record{{"a", "3", nil}, {"a", "3", keyPointer("x")}}},
		{"invalid UTF-8", []Record{{"a", "3", keyPointer(string([]byte{0xff}))}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Start with a real collision to ensure errors never leak partial results.
			records := append([]Record{{"a", "1", keyPointer("x")}, {"a", "2", keyPointer("X")}}, tc.records...)
			got, err := FindCollisions(records)
			if err == nil || got != nil {
				t.Fatalf("got %#v, %v; want nil result and error", got, err)
			}
		})
	}
}

func TestFindCollisionsDeterministic(t *testing.T) {
	records := []Record{{"b", "2", keyPointer("X")}, {"a", "2", keyPointer("Y")}, {"b", "1", keyPointer("x")}, {"a", "1", keyPointer("y")}}
	want := []Collision{{"a", "y", []string{"1", "2"}}, {"b", "x", []string{"1", "2"}}}
	count := 0
	var permute func(int)
	permute = func(start int) {
		if start == len(records) {
			count++
			t.Run(fmt.Sprint(count), func(t *testing.T) {
				got, err := FindCollisions(records)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("got %#v, %v; want %#v", got, err, want)
				}
			})
			return
		}
		for i := start; i < len(records); i++ {
			records[start], records[i] = records[i], records[start]
			permute(start + 1)
			records[start], records[i] = records[i], records[start]
		}
	}
	permute(0)
	if count != 24 {
		t.Fatalf("tested %d permutations", count)
	}
}

func TestFindCollisionsPreservesInput(t *testing.T) {
	records := []Record{{"b", "2", keyPointer("E\u0301")}, {"b", "1", keyPointer("é")}, {"a", "3", nil}, {"a", "4", keyPointer("")}}
	snapshot := make([]Record, len(records))
	for i, record := range records {
		snapshot[i] = record
		if record.Key != nil {
			snapshot[i].Key = keyPointer(*record.Key)
		}
	}
	pointers := []*string{records[0].Key, records[1].Key, records[2].Key, records[3].Key}
	got, err := FindCollisions(records)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(records, snapshot) {
		t.Fatalf("input mutated: %#v", records)
	}
	for i := range records {
		if records[i].Key != pointers[i] {
			t.Fatalf("key pointer %d replaced", i)
		}
	}
	got[0].IDs[0] = "changed"
	again, err := FindCollisions(records)
	want := []Collision{{"b", "é", []string{"1", "2"}}}
	if err != nil || !reflect.DeepEqual(again, want) || !reflect.DeepEqual(records, snapshot) {
		t.Fatalf("result aliases input: %#v, %v", again, err)
	}
}
