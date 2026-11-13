package soql

import (
	"encoding/json"
	"testing"
)

// columnJSON compares the columnMetadata array by its JSON form.
func columnJSON(t *testing.T, got []map[string]any) string {
	t.Helper()
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestColumnMetadataShapes(t *testing.T) {
	f := newBindFixture()
	for _, tt := range []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "plain field",
			query: "select txt__c from item__c",
			want:  `[{"columnName":"Txt__c"}]`,
		},
		{
			name:  "parent path",
			query: "select parent__r.name from item__c",
			want:  `[{"columnName":"Parent__r","joinColumns":[{"columnName":"Name","joinColumns":[]}]}]`,
		},
		{
			name:  "nested parent path",
			query: "select self__r.self__r.name from loop__c",
			want:  `[{"columnName":"Self__r","joinColumns":[{"columnName":"Self__r","joinColumns":[{"columnName":"Name","joinColumns":[]}]}]}]`,
		},
		{
			name:  "child subquery",
			query: "select name, (select name from typeditems__r) from item__c",
			want:  `[{"columnName":"Name"},{"columnName":"Typeditems__r","joinColumns":[{"columnName":"Name","joinColumns":[]}]}]`,
		},
		{
			name:  "child plain and parent leaf sharing a name",
			query: "select name, (select name, item__r.name from typeditems__r) from item__c",
			want:  `[{"columnName":"Name"},{"columnName":"Typeditems__r","joinColumns":[{"columnName":"Name","joinColumns":[]},{"columnName":"Item__r","joinColumns":[{"columnName":"Name","joinColumns":[]}]}]}]`,
		},
		{
			name:  "distinct parent paths sharing a leaf",
			query: "select parent__r.name, group__r.name from item__c",
			want:  `[{"columnName":"Parent__r","joinColumns":[{"columnName":"Name","joinColumns":[]}]},{"columnName":"Group__r","joinColumns":[{"columnName":"Name","joinColumns":[]}]}]`,
		},
		{
			name:  "aggregate keys",
			query: "select count(id), sum(amount__c) total from item__c",
			want:  `[{"aggregate":true,"columnName":"expr0"},{"aggregate":true,"columnName":"total"}]`,
		},
		{
			name:  "count only",
			query: "select count() from item__c",
			want:  `[{"aggregate":true,"columnName":"expr0"}]`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b, err := f.bind(tt.query, f.admin)
			if err != nil {
				t.Fatal(err)
			}
			if got := columnJSON(t, ColumnMetadata(b)); got != tt.want {
				t.Fatalf("columnMetadata = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestColumnMetadataNilBound(t *testing.T) {
	if got := ColumnMetadata(nil); len(got) != 0 {
		t.Fatalf("nil bound = %#v", got)
	}
}
