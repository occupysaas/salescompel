package soql

import "testing"

// TestColumnMetadataAggregatePaths keeps a relationship path in aggregate
// column metadata: the parent chain nests joinColumns and the aggregate leaf
// carries aggregate=true with the explicit empty joinColumns array the pinned
// CLI requires. spec: soql-08
func TestColumnMetadataAggregatePaths(t *testing.T) {
	f := newBindFixture()
	for _, tt := range []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "grouped parent field",
			query: "select parent__r.name, count(id) from item__c group by parent__r.name",
			want:  `[{"columnName":"Parent__r","joinColumns":[{"columnName":"Name","joinColumns":[]}]},{"aggregate":true,"columnName":"expr0"}]`,
		},
		{
			name:  "aggregate over parent path",
			query: "select min(parent__r.name) from item__c",
			want:  `[{"columnName":"Parent__r","joinColumns":[{"aggregate":true,"columnName":"expr0","joinColumns":[]}]}]`,
		},
		{
			name:  "five hop aggregate path",
			query: "select min(self__r.self__r.self__r.self__r.self__r.name) from loop__c",
			want:  `[{"columnName":"Self__r","joinColumns":[{"columnName":"Self__r","joinColumns":[{"columnName":"Self__r","joinColumns":[{"columnName":"Self__r","joinColumns":[{"columnName":"Self__r","joinColumns":[{"aggregate":true,"columnName":"expr0","joinColumns":[]}]}]}]}]}]}]`,
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
