package fieldtype

import (
	"reflect"
	"testing"
)

func TestMultiPicklist(t *testing.T) {
	got, values := MultiPicklist(" B ;A;;B;Z", []string{"A", "B", "C"})
	if got != "A;B;Z" || !reflect.DeepEqual(values, []string{"A", "B", "Z"}) {
		t.Fatalf("got %q %v", got, values)
	}
	if got, values := MultiPicklist(" ; ", []string{"A"}); got != "" || len(values) != 0 {
		t.Fatalf("empty got %q %v", got, values)
	}
	if got, _ := MultiPicklist("Y;X;A", []string{"A"}); got != "A;Y;X" {
		t.Fatalf("unlisted order got %q", got)
	}
}
