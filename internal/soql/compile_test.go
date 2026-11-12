package soql

import (
	"strings"
	"testing"

	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
)

const marker = "XyZzY987654321MAGiC"

func cField(name string, k fieldtype.Kind) metadata.Field {
	f := metadata.Field{Name: name, Column: "col_" + name, Nillable: true, Createable: true, Updateable: true,
		Options: metadata.Options{Kind: k}}
	switch k {
	case fieldtype.Number, fieldtype.Currency, fieldtype.Percent:
		f.Precision, f.Scale = 18, 2
	case fieldtype.Text:
		f.Length = 80
	}
	return metadata.Complete(f)
}

func cObj(name, prefix string, fields ...metadata.Field) metadata.Object {
	return metadata.Object{Name: name, Label: name, PluralLabel: name + "s", Prefix: prefix, Fields: fields}
}

func TestCompileParameterized(t *testing.T) {
	item := cObj("Item__c", "a11",
		cField("Id", fieldtype.Id),
		cField("Name", fieldtype.Text),
		cField("Txt__c", fieldtype.Text),
		cField("Num__c", fieldtype.Number),
		cField("Flag__c", fieldtype.Checkbox),
		cField("Day__c", fieldtype.Date),
		cField("At__c", fieldtype.DateTime),
		cField("Stage__c", fieldtype.Picklist),
		cField("Multi__c", fieldtype.MultiselectPicklist),
	)
	parent := cObj("Parent__c", "a12",
		cField("Id", fieldtype.Id),
		cField("Name", fieldtype.Text),
	)
	relName := "Parent__r"
	parentRef := metadata.Complete(metadata.Field{
		Name: "Parent__c", Column: "col_parent", Nillable: true,
		ReferenceTo: []string{"Parent__c"}, RelationshipName: &relName,
		Options: metadata.Options{Kind: fieldtype.Lookup},
	})
	item.Fields = append(item.Fields, parentRef)

	textVal := cField("Txt__c", fieldtype.Text)
	numVal := cField("Num__c", fieldtype.Number)
	flagVal := cField("Flag__c", fieldtype.Checkbox)
	dayVal := cField("Day__c", fieldtype.Date)
	stageVal := cField("Stage__c", fieldtype.Picklist)
	multiVal := cField("Multi__c", fieldtype.MultiselectPicklist)
	nameVal := cField("Name", fieldtype.Text)

	cases := []struct {
		name string
		b    *Bound
	}{
		{
			name: "text equality",
			b: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where:   &BoundCmp{Left: BoundValue{Field: textVal}, Op: "=", Right: Param{Value: marker, Cast: "text"}},
			},
		},
		{
			name: "number comparison",
			b: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where:   &BoundCmp{Left: BoundValue{Field: numVal}, Op: ">", Right: Param{Value: marker, Cast: "numeric"}},
			},
		},
		{
			name: "null test",
			b: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where:   &BoundCmp{Left: BoundValue{Field: textVal}, Op: "=", Right: Param{Value: nil}},
			},
		},
		{
			name: "bool equality",
			b: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where:   &BoundCmp{Left: BoundValue{Field: flagVal}, Op: "=", Right: Param{Value: true, Cast: "bool"}},
			},
		},
		{
			name: "date comparison",
			b: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where:   &BoundCmp{Left: BoundValue{Field: dayVal}, Op: ">=", Right: Param{Value: "2026-" + marker, Cast: "date"}},
			},
		},
		{
			name: "like",
			b: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where:   &BoundCmp{Left: BoundValue{Field: textVal}, Op: "LIKE", Right: Param{Value: marker + "%", Cast: "text"}},
			},
		},
		{
			name: "in list",
			b: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where:   &BoundIn{Left: BoundValue{Field: stageVal}, Values: []Param{{Value: marker, Cast: "text"}, {Value: marker + "2", Cast: "text"}}},
			},
		},
		{
			name: "not in nullable",
			b: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where:   &BoundIn{Left: BoundValue{Field: textVal}, Not: true, Values: []Param{{Value: marker, Cast: "text"}}},
			},
		},
		{
			name: "includes",
			b: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where:   &BoundIncludes{Left: BoundValue{Field: multiVal}, Values: []Param{{Value: marker + ";" + marker + "B", Cast: "text"}}},
			},
		},
		{
			name: "excludes",
			b: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where:   &BoundIncludes{Left: BoundValue{Field: multiVal}, Exclude: true, Values: []Param{{Value: marker, Cast: "text"}}},
			},
		},
		{
			name: "and/or/not",
			b: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where: &BoundAnd{Items: []BoundExpr{
					&BoundOr{Items: []BoundExpr{
						&BoundCmp{Left: BoundValue{Field: textVal}, Op: "=", Right: Param{Value: marker, Cast: "text"}},
						&BoundNot{Expr: &BoundCmp{Left: BoundValue{Field: nameVal}, Op: "!=", Right: Param{Value: marker + "N", Cast: "text"}}},
					}},
					&BoundCmp{Left: BoundValue{Field: numVal}, Op: "<=", Right: Param{Value: marker, Cast: "numeric"}},
				}},
			},
		},
		{
			name: "parent path",
			b: &Bound{
				Object: item,
				Columns: []BoundColumn{
					{Key: "Id", Field: cField("Id", fieldtype.Id)},
					{Key: "Name", Path: []PathStep{{Object: parent, Via: parentRef}}, Field: nameVal},
				},
				Where: &BoundCmp{
					Left:  BoundValue{Path: []PathStep{{Object: parent, Via: parentRef}}, Field: nameVal},
					Op:    "=",
					Right: Param{Value: marker, Cast: "text"},
				},
			},
		},
		{
			name: "order by parent",
			b: &Bound{
				Object: item,
				Columns: []BoundColumn{
					{Key: "Id", Field: cField("Id", fieldtype.Id)},
				},
				Order: []BoundOrder{
					{Value: BoundValue{Path: []PathStep{{Object: parent, Via: parentRef}}, Field: nameVal}},
				},
			},
		},
		{
			name: "limit offset",
			b: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where:   &BoundCmp{Left: BoundValue{Field: textVal}, Op: "=", Right: Param{Value: marker, Cast: "text"}},
				Limit:   ptr(10),
				Offset:  ptr(5),
			},
		},
		{
			name: "semi-join",
			b: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where: &BoundIn{
					Left: BoundValue{Field: cField("Id", fieldtype.Id)},
					Semi: &Bound{
						Object:  parent,
						Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
						Where:   &BoundCmp{Left: BoundValue{Field: nameVal}, Op: "=", Right: Param{Value: marker, Cast: "text"}},
					},
				},
			},
		},
		{
			name: "anti-join",
			b: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where: &BoundIn{
					Left: BoundValue{Field: cField("Id", fieldtype.Id)},
					Not:  true,
					Semi: &Bound{
						Object:  parent,
						Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
						Where:   &BoundCmp{Left: BoundValue{Field: nameVal}, Op: "=", Right: Param{Value: marker, Cast: "text"}},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := Compile(tc.b)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(plan.Stmt.SQL, marker) {
				t.Fatalf("marker leaked into SQL: %s", plan.Stmt.SQL)
			}
			lowerMarker := strings.ToLower(marker)
			found := false
			for _, a := range plan.Stmt.Args {
				switch v := a.(type) {
				case string:
					if strings.Contains(v, marker) || strings.Contains(v, lowerMarker) {
						found = true
					}
				case []string:
					for _, s := range v {
						if strings.Contains(s, marker) || strings.Contains(s, lowerMarker) {
							found = true
						}
					}
				}
			}
			skipCheck := tc.name == "null test" || tc.name == "bool equality" || tc.name == "order by parent"
			if !found && !skipCheck {
				t.Fatalf("marker not found in args: %v", plan.Stmt.Args)
			}
		})
	}
}

// TestSemiJoinArgNumbering verifies that the semi-join sub-compiler continues
// the outer arg sequence so placeholder positions match the combined args slice.
func TestSemiJoinArgNumbering(t *testing.T) {
	item := cObj("Item__c", "a11",
		cField("Id", fieldtype.Id),
		cField("Txt__c", fieldtype.Text),
	)
	parent := cObj("Parent__c", "a12",
		cField("Id", fieldtype.Id),
		cField("Name", fieldtype.Text),
	)

	outerVal := cField("Txt__c", fieldtype.Text)
	subVal := cField("Name", fieldtype.Text)

	b := &Bound{
		Object:  item,
		Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
		Where: &BoundAnd{Items: []BoundExpr{
			&BoundCmp{Left: BoundValue{Field: outerVal}, Op: "=", Right: Param{Value: "outer-" + marker, Cast: "text"}},
			&BoundIn{
				Left: BoundValue{Field: cField("Id", fieldtype.Id)},
				Semi: &Bound{
					Object:  parent,
					Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
					Where:   &BoundCmp{Left: BoundValue{Field: subVal}, Op: "=", Right: Param{Value: "sub-" + marker, Cast: "text"}},
				},
			},
		}},
	}

	plan, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}

	// Find the positions of both marker args.
	outerIdx, subIdx := -1, -1
	for i, a := range plan.Stmt.Args {
		s, ok := a.(string)
		if !ok {
			continue
		}
		if s == "outer-"+marker {
			outerIdx = i
		}
		if s == "sub-"+marker {
			subIdx = i
		}
	}
	if outerIdx == -1 {
		t.Fatal("outer marker not found in args:", plan.Stmt.Args)
	}
	if subIdx == -1 {
		t.Fatal("sub marker not found in args:", plan.Stmt.Args)
	}
	// The sub-join arg must come after the outer WHERE arg (sequential numbering).
	if subIdx <= outerIdx {
		t.Fatalf("semi-join arg numbering broken: outer at %d, sub at %d (sub must be after outer)", outerIdx, subIdx)
	}
	// Verify SQL placeholders are sequential: $n for outer, $n+k for sub.
	sql := plan.Stmt.SQL
	outerPh := "$" + itoa(outerIdx+1)
	subPh := "$" + itoa(subIdx+1)
	if !strings.Contains(sql, outerPh) {
		t.Fatalf("outer placeholder %s not in SQL: %s", outerPh, sql)
	}
	if !strings.Contains(sql, subPh) {
		t.Fatalf("sub placeholder %s not in SQL: %s", subPh, sql)
	}
}

// TestParentPathArgsInOrder verifies that parent-path WHERE params land in args.
func TestParentPathArgsInOrder(t *testing.T) {
	item := cObj("Item__c", "a11",
		cField("Id", fieldtype.Id),
		cField("Txt__c", fieldtype.Text),
	)
	parent := cObj("Parent__c", "a12",
		cField("Id", fieldtype.Id),
		cField("Name", fieldtype.Text),
	)
	relName := "Parent__r"
	parentRef := metadata.Complete(metadata.Field{
		Name: "Parent__c", Column: "col_parent", Nillable: true,
		ReferenceTo: []string{"Parent__c"}, RelationshipName: &relName,
		Options: metadata.Options{Kind: fieldtype.Lookup},
	})
	item.Fields = append(item.Fields, parentRef)
	nameVal := cField("Name", fieldtype.Text)

	b := &Bound{
		Object: item,
		Columns: []BoundColumn{
			{Key: "Id", Field: cField("Id", fieldtype.Id)},
			{Key: "Name", Path: []PathStep{{Object: parent, Via: parentRef}}, Field: nameVal},
		},
		Where: &BoundCmp{
			Left:  BoundValue{Path: []PathStep{{Object: parent, Via: parentRef}}, Field: nameVal},
			Op:    "=",
			Right: Param{Value: marker, Cast: "text"},
		},
	}

	plan, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plan.Stmt.SQL, marker) {
		t.Fatalf("marker leaked into SQL: %s", plan.Stmt.SQL)
	}
	found := false
	for _, a := range plan.Stmt.Args {
		if s, ok := a.(string); ok && strings.Contains(s, marker) {
			found = true
		}
	}
	if !found {
		t.Fatalf("parent-path WHERE marker not found in args: %v", plan.Stmt.Args)
	}
}

// TestFilterAndSortParentPath verifies parent-path WHERE/ORDER BY produce LEFT JOINs
// even when no parent columns are selected.
func TestFilterAndSortParentPath(t *testing.T) {
	item := cObj("Item__c", "a11", cField("Id", fieldtype.Id), cField("Txt__c", fieldtype.Text))
	parent := cObj("Parent__c", "a12", cField("Id", fieldtype.Id), cField("Name", fieldtype.Text))
	relName := "Parent__r"
	parentRef := metadata.Complete(metadata.Field{Name: "Parent__c", Column: "col_parent", Nillable: true,
		ReferenceTo: []string{"Parent__c"}, RelationshipName: &relName, Options: metadata.Options{Kind: fieldtype.Lookup}})
	item.Fields = append(item.Fields, parentRef)
	nameVal := cField("Name", fieldtype.Text)
	path := []PathStep{{Object: parent, Via: parentRef}}
	idCol := []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}}

	t.Run("filter only", func(t *testing.T) {
		plan, err := Compile(&Bound{Object: item, Columns: idCol,
			Where: &BoundCmp{Left: BoundValue{Path: path, Field: nameVal}, Op: "=", Right: Param{Value: marker, Cast: "text"}}})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(plan.Stmt.SQL, "LEFT JOIN") {
			t.Fatalf("filter-only parent path missing LEFT JOIN: %s", plan.Stmt.SQL)
		}
	})
	t.Run("sort only", func(t *testing.T) {
		plan, err := Compile(&Bound{Object: item, Columns: idCol,
			Order: []BoundOrder{{Value: BoundValue{Path: path, Field: nameVal}}}})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(plan.Stmt.SQL, "LEFT JOIN") {
			t.Fatalf("sort-only parent path missing LEFT JOIN: %s", plan.Stmt.SQL)
		}
	})
}

// TestAntiJoinNotExists verifies anti-joins compile to NOT EXISTS (NULL-safe).
func TestAntiJoinNotExists(t *testing.T) {
	item := cObj("Item__c", "a11", cField("Id", fieldtype.Id))
	parent := cObj("Parent__c", "a12", cField("Id", fieldtype.Id), cField("Name", fieldtype.Text))
	b := &Bound{Object: item, Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
		Where: &BoundIn{Left: BoundValue{Field: cField("Id", fieldtype.Id)}, Not: true,
			Semi: &Bound{Object: parent, Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where: &BoundCmp{Left: BoundValue{Field: cField("Name", fieldtype.Text)}, Op: "=", Right: Param{Value: marker, Cast: "text"}}}}}
	plan, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	sql := plan.Stmt.SQL
	if !strings.Contains(sql, "NOT EXISTS") {
		t.Fatalf("anti-join must use NOT EXISTS: %s", sql)
	}
	if strings.Contains(sql, "NOT IN") {
		t.Fatalf("anti-join must not use NOT IN: %s", sql)
	}
}

// TestInNilRejection verifies nil values in IN/NOT IN lists are rejected.
func TestInNilRejection(t *testing.T) {
	item := cObj("Item__c", "a11", cField("Id", fieldtype.Id))
	txtVal := cField("Txt__c", fieldtype.Text)
	for name, in := range map[string]*BoundIn{
		"IN nil":     {Left: BoundValue{Field: txtVal}, Values: []Param{{Value: "a", Cast: "text"}, {Value: nil}}},
		"NOT IN nil": {Left: BoundValue{Field: txtVal}, Not: true, Values: []Param{{Value: nil}, {Value: "a", Cast: "text"}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Compile(&Bound{Object: item, Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}}, Where: in}); err == nil {
				t.Fatal("nil in IN list should be rejected")
			}
		})
	}
}

// TestSemiJoinSubRootON verifies the semi-join ON clause uses s.tenant_id (sub-root),
// not r.tenant_id (outer root).
func TestSemiJoinSubRootON(t *testing.T) {
	item := cObj("Item__c", "a11", cField("Id", fieldtype.Id))
	parent := cObj("Parent__c", "a12", cField("Id", fieldtype.Id), cField("Name", fieldtype.Text))
	relName := "Parent__r"
	parentRef := metadata.Complete(metadata.Field{Name: "Parent__c", Column: "col_parent", Nillable: true,
		ReferenceTo: []string{"Parent__c"}, RelationshipName: &relName, Options: metadata.Options{Kind: fieldtype.Lookup}})
	item.Fields = append(item.Fields, parentRef)
	nameVal := cField("Name", fieldtype.Text)
	b := &Bound{Object: item, Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
		Where: &BoundIn{Left: BoundValue{Field: cField("Id", fieldtype.Id)},
			Semi: &Bound{Object: item, Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where: &BoundCmp{Left: BoundValue{Path: []PathStep{{Object: parent, Via: parentRef}}, Field: nameVal},
					Op: "=", Right: Param{Value: marker, Cast: "text"}}}}}
	plan, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	sql := plan.Stmt.SQL
	if !strings.Contains(sql, "s.tenant_id") {
		t.Fatalf("semi-join ON must use sub-root s.tenant_id: %s", sql)
	}
	if strings.Contains(sql, "r.tenant_id = s.tenant_id") {
		t.Fatalf("semi-join ON must not correlate to outer root: %s", sql)
	}
}

// TestSemiJoinParentPathFilter verifies that a semi-join subquery with a
// parent-path WHERE filter carries the sub-compiler's joins into the subquery FROM.
func TestSemiJoinParentPathFilter(t *testing.T) {
	item := cObj("Item__c", "a11",
		cField("Id", fieldtype.Id),
		cField("Txt__c", fieldtype.Text),
	)
	parent := cObj("Parent__c", "a12",
		cField("Id", fieldtype.Id),
		cField("Name", fieldtype.Text),
	)
	relName := "Parent__r"
	parentRef := metadata.Complete(metadata.Field{
		Name: "Parent__c", Column: "col_parent", Nillable: true,
		ReferenceTo: []string{"Parent__c"}, RelationshipName: &relName,
		Options: metadata.Options{Kind: fieldtype.Lookup},
	})
	item.Fields = append(item.Fields, parentRef)
	nameVal := cField("Name", fieldtype.Text)

	b := &Bound{
		Object:  item,
		Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
		Where: &BoundIn{
			Left: BoundValue{Field: cField("Id", fieldtype.Id)},
			Semi: &Bound{
				Object:  item,
				Columns: []BoundColumn{{Key: "Id", Field: cField("Id", fieldtype.Id)}},
				Where: &BoundCmp{
					Left:  BoundValue{Path: []PathStep{{Object: parent, Via: parentRef}}, Field: nameVal},
					Op:    "=",
					Right: Param{Value: marker, Cast: "text"},
				},
			},
		},
	}

	plan, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	sql := plan.Stmt.SQL
	// The subquery must include a LEFT JOIN for the parent-path filter.
	if !strings.Contains(sql, "LEFT JOIN") {
		t.Fatalf("semi-join subquery missing parent-path JOIN: %s", sql)
	}
	if !strings.Contains(sql, "IN (SELECT") {
		t.Fatalf("semi-join missing IN (SELECT: %s", sql)
	}
	// Marker must be in args, not SQL.
	if strings.Contains(sql, marker) {
		t.Fatalf("marker leaked into SQL: %s", sql)
	}
	found := false
	for _, a := range plan.Stmt.Args {
		if s, ok := a.(string); ok && strings.Contains(s, marker) {
			found = true
		}
	}
	if !found {
		t.Fatalf("semi-join parent-path marker not found in args: %v", plan.Stmt.Args)
	}
}
