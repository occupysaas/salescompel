package soql

import "testing"

// TestBindOrderByExpressions refuses ORDER BY toLabel (directly or through a
// select alias) with MALFORMED_QUERY while legal field, path, date-function
// and aggregate keys continue to bind. spec: soql-05
func TestBindOrderByExpressions(t *testing.T) {
	f := newBindFixture()
	for _, text := range []string{
		"select Id from Item__c order by toLabel(Stage__c)",
		"select toLabel(Stage__c) s from Item__c order by s",
	} {
		wantCode(t, mustBind(t, f, text, f.admin), "MALFORMED_QUERY")
	}
	for _, text := range []string{
		"select Id from Item__c order by Name",
		"select Id from Item__c order by Parent__r.Name",
		"select Id from Item__c order by CALENDAR_YEAR(Day__c)",
		"select CALENDAR_YEAR(Day__c) y from Item__c order by y",
		"select COUNT(Id) total from Item__c order by total",
	} {
		if _, err := f.bind(text, f.admin); err != nil {
			t.Errorf("Bind(%s) = %v, want ok", text, err)
		}
	}
}
