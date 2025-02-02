// Copyright 2021-present The Atlas Authors. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package mysql

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"ariga.io/atlas/sql/internal/sqlx"
	"ariga.io/atlas/sql/schema"
)

// A diff provides a MySQL implementation for sqlx.DiffDriver.
type diff struct{ conn }

// SchemaAttrDiff returns a changeset for migrating schema attributes from one state to the other.
func (d *diff) SchemaAttrDiff(from, to *schema.Schema) []schema.Change {
	var changes []schema.Change
	// Charset change.
	if change := d.charsetChange(from.Attrs, from.Realm.Attrs, to.Attrs); change != noChange {
		changes = append(changes, change)
	}
	// Collation change.
	if change := d.collationChange(from.Attrs, from.Realm.Attrs, to.Attrs); change != noChange {
		changes = append(changes, change)
	}
	return changes
}

// TableAttrDiff returns a changeset for migrating table attributes from one state to the other.
func (d *diff) TableAttrDiff(from, to *schema.Table) []schema.Change {
	var changes []schema.Change
	// Charset change.
	if change := d.charsetChange(from.Attrs, from.Schema.Attrs, to.Attrs); change != noChange {
		changes = append(changes, change)
	}
	// Collation change.
	if change := d.collationChange(from.Attrs, from.Schema.Attrs, to.Attrs); change != noChange {
		changes = append(changes, change)
	}
	// Drop or modify checks.
	for _, c1 := range checks(from.Attrs) {
		switch c2, ok := checkByName(to.Attrs, c1.Name); {
		case !ok:
			changes = append(changes, &schema.DropAttr{
				A: c1,
			})
		case c1.Clause != c2.Clause || c1.Enforced != c2.Enforced:
			changes = append(changes, &schema.ModifyAttr{
				From: c1,
				To:   c2,
			})
		}
	}
	// Add checks.
	for _, c1 := range checks(to.Attrs) {
		if _, ok := checkByName(from.Attrs, c1.Name); !ok {
			changes = append(changes, &schema.AddAttr{
				A: c1,
			})
		}
	}
	return changes
}

// ColumnChange returns the schema changes (if any) for migrating one column to the other.
func (d *diff) ColumnChange(from, to *schema.Column) (schema.ChangeKind, error) {
	change := sqlx.CommentChange(from.Attrs, to.Attrs)
	if from.Type.Null != to.Type.Null {
		change |= schema.ChangeNull
	}
	changed, err := d.typeChanged(from, to)
	if err != nil {
		return schema.NoChange, err
	}
	if changed {
		change |= schema.ChangeType
	}
	changed, err = d.defaultChanged(from, to)
	if err != nil {
		return schema.NoChange, err
	}
	if changed {
		change |= schema.ChangeDefault
	}
	return change, nil
}

// IndexAttrChanged reports if the index attributes were changed.
func (*diff) IndexAttrChanged(from, to []schema.Attr) bool {
	return indexType(from).T != indexType(to).T
}

// IndexPartAttrChanged reports if the index-part attributes were changed.
func (*diff) IndexPartAttrChanged(from, to []schema.Attr) bool {
	return indexCollation(from).V != indexCollation(to).V
}

// ReferenceChanged reports if the foreign key referential action was changed.
func (*diff) ReferenceChanged(from, to schema.ReferenceOption) bool {
	// According to MySQL docs, foreign key constraints are checked
	// immediately, so NO ACTION is the same as RESTRICT. Specifying
	// RESTRICT (or NO ACTION) is the same as omitting the ON DELETE
	// or ON UPDATE clause.
	if from == "" || from == schema.Restrict {
		from = schema.NoAction
	}
	if to == "" || to == schema.Restrict {
		to = schema.NoAction
	}
	return from != to
}

// Normalize implements the sqlx.Normalizer interface.
func (d *diff) Normalize(from, to *schema.Table) {
	for i, idx := range from.Indexes {
		if _, ok := to.Index(idx.Name); ok {
			continue
		}
		// MySQL requires that foreign key columns be indexed; Therefore, if the child
		// table is defined on non-indexed columns, an index is automatically created
		// to satisfy the constraint.
		// Therefore, if no such key was defined on the desired state, the diff will
		// recommend to drop it on migration. Therefore, we fix it by dropping it from
		// the current state manually.
		if keySupportsFK(from, idx) {
			from.Indexes = append(from.Indexes[:i], from.Indexes[i+1:]...)
		}
	}
}

// collationChange returns the schema change for migrating the collation if
// it was changed and its not the default attribute inherited from its parent.
func (d *diff) collationChange(from, top, to []schema.Attr) schema.Change {
	var fromC, topC, toC schema.Collation
	switch fromHas, topHas, toHas := sqlx.Has(from, &fromC), sqlx.Has(top, &topC), sqlx.Has(to, &toC); {
	case !fromHas && !toHas:
	case !fromHas:
		return &schema.AddAttr{
			A: &toC,
		}
	case !toHas:
		if !topHas || fromC.V != topC.V {
			return &schema.DropAttr{
				A: &fromC,
			}
		}
	case fromC.V != toC.V:
		return &schema.ModifyAttr{
			From: &fromC,
			To:   &toC,
		}
	}
	return noChange
}

// charsetChange returns the schema change for migrating the collation if
// it was changed and its not the default attribute inherited from its parent.
func (d *diff) charsetChange(from, top, to []schema.Attr) schema.Change {
	var fromC, topC, toC schema.Charset
	switch fromHas, topHas, toHas := sqlx.Has(from, &fromC), sqlx.Has(top, &topC), sqlx.Has(to, &toC); {
	case !fromHas && !toHas:
	case !fromHas:
		return &schema.AddAttr{
			A: &toC,
		}
	case !toHas:
		if !topHas || fromC.V != topC.V {
			return &schema.DropAttr{
				A: &fromC,
			}
		}
	case fromC.V != toC.V:
		return &schema.ModifyAttr{
			From: &fromC,
			To:   &toC,
		}
	}
	return noChange
}

// indexCollation returns the index collation from its attribute.
// The default collation is ascending if no order was specified.
func indexCollation(attr []schema.Attr) *schema.Collation {
	c := &schema.Collation{V: "A"}
	if sqlx.Has(attr, c) {
		c.V = strings.ToUpper(c.V)
	}
	return c
}

// indexType returns the index type from its attribute.
// The default type is BTREE if no type was specified.
func indexType(attr []schema.Attr) *IndexType {
	t := &IndexType{T: "BTREE"}
	if sqlx.Has(attr, t) {
		t.T = strings.ToUpper(t.T)
	}
	return t
}

// noChange describes a zero change.
var noChange struct{ schema.Change }

func checks(attr []schema.Attr) (checks []*Check) {
	for i := range attr {
		if c, ok := attr[i].(*Check); ok {
			checks = append(checks, c)
		}
	}
	return checks
}

func (d *diff) typeChanged(from, to *schema.Column) (bool, error) {
	fromT, toT := from.Type.Type, to.Type.Type
	if fromT == nil || toT == nil {
		return false, fmt.Errorf("mysql: missing type infromation for column %q", from.Name)
	}
	if reflect.TypeOf(fromT) != reflect.TypeOf(toT) {
		return true, nil
	}
	var changed bool
	switch fromT := fromT.(type) {
	case *schema.BinaryType, *schema.BoolType, *schema.DecimalType, *schema.FloatType:
		changed = mustFormat(fromT) != mustFormat(toT)
	case *schema.EnumType:
		toT := toT.(*schema.EnumType)
		changed = !sqlx.ValuesEqual(fromT.Values, toT.Values)
	case *schema.IntegerType:
		toT := toT.(*schema.IntegerType)
		// MySQL v8.0.19 dropped the display width
		// information from the information schema.
		if d.supportsDisplayWidth() {
			ft, _, _, err := parseColumn(fromT.T)
			if err != nil {
				return false, err
			}
			tt, _, _, err := parseColumn(toT.T)
			if err != nil {
				return false, err
			}
			fromT.T, toT.T = ft[0], tt[0]
		}
		fromW, toW := displayWidth(fromT.Attrs), displayWidth(toT.Attrs)
		changed = fromT.T != toT.T || fromT.Unsigned != toT.Unsigned ||
			(fromW != nil) != (toW != nil) || (fromW != nil && fromW.N != toW.N)
	case *schema.JSONType:
		toT := toT.(*schema.JSONType)
		changed = fromT.T != toT.T
	case *schema.StringType:
		changed = mustFormat(fromT) != mustFormat(toT)
	case *schema.SpatialType:
		toT := toT.(*schema.SpatialType)
		changed = fromT.T != toT.T
	case *schema.TimeType:
		toT := toT.(*schema.TimeType)
		changed = fromT.T != toT.T
	case *BitType:
		toT := toT.(*BitType)
		changed = fromT.T != toT.T
	case *SetType:
		toT := toT.(*SetType)
		changed = !sqlx.ValuesEqual(fromT.Values, toT.Values)
	default:
		return false, &sqlx.UnsupportedTypeError{Type: fromT}
	}
	return changed, nil
}

// defaultChanged reports if the a default value of a column
// type was changed.
func (d *diff) defaultChanged(from, to *schema.Column) (bool, error) {
	d1, ok1 := from.Default.(*schema.RawExpr)
	d2, ok2 := to.Default.(*schema.RawExpr)
	if ok1 != ok2 {
		return true, nil
	}
	if d1 == nil || d1.X == d2.X {
		return false, nil
	}
	switch from.Type.Type.(type) {
	case *schema.BoolType:
		a, err1 := boolValue(d1.X)
		b, err2 := boolValue(d2.X)
		if err1 == nil && err2 == nil {
			return a != b, nil
		}
		return false, nil
	case *schema.IntegerType:
		return !d.equalsIntValues(d1.X, d2.X), nil
	case *schema.TimeType:
		x1 := strings.ToLower(strings.Trim(d1.X, "' ()"))
		x2 := strings.ToLower(strings.Trim(d2.X, "' ()"))
		return x1 != x2, nil
	default:
		x1 := strings.Trim(d1.X, "'")
		x2 := strings.Trim(d2.X, "'")
		return x1 != x2, nil
	}
}

// equalsIntValues report if the 2 int default values are ~equal.
// Note that default expression are not supported atm.
func (d *diff) equalsIntValues(x1, x2 string) bool {
	x1 = strings.ToLower(strings.Trim(x1, "' "))
	x2 = strings.ToLower(strings.Trim(x2, "' "))
	if x1 == x2 {
		return true
	}
	d1, err := strconv.ParseInt(x1, 10, 64)
	if err != nil {
		// Numbers are rounded down to their nearest integer.
		f, err := strconv.ParseFloat(x1, 64)
		if err != nil {
			return false
		}
		d1 = int64(f)
	}
	d2, err := strconv.ParseInt(x2, 10, 64)
	if err != nil {
		// Numbers are rounded down to their nearest integer.
		f, err := strconv.ParseFloat(x2, 64)
		if err != nil {
			return false
		}
		d2 = int64(f)
	}
	return d1 == d2
}

// boolValue returns the MySQL boolean value for the given string (if it is known).
func boolValue(x string) (bool, error) {
	switch x {
	case "1", "'1'", "TRUE", "true":
		return true, nil
	case "0", "'0'", "FALSE", "false":
		return false, nil
	default:
		return false, fmt.Errorf("mysql: unknown value: %q", x)
	}
}

func checkByName(attr []schema.Attr, name string) (*Check, bool) {
	for i := range attr {
		if c, ok := attr[i].(*Check); ok && c.Name == name {
			return c, true
		}
	}
	return nil, false
}

func displayWidth(attr []schema.Attr) *DisplayWidth {
	var (
		z *ZeroFill
		d *DisplayWidth
	)
	for i := range attr {
		switch at := attr[i].(type) {
		case *ZeroFill:
			z = at
		case *DisplayWidth:
			d = at
		}
	}
	// Accept the display width only if
	// the zerofill attribute is defined.
	if z == nil || d == nil {
		return nil
	}
	return d
}

// keySupportsFK reports if the index key was created automatically by MySQL
// to support the constraint. See sql/sql_table.cc#find_fk_supporting_key.
func keySupportsFK(t *schema.Table, idx *schema.Index) bool {
	if _, ok := t.ForeignKey(idx.Name); ok {
		return true
	}
search:
	for _, fk := range t.ForeignKeys {
		if len(fk.Columns) != len(idx.Parts) {
			continue
		}
		for i, c := range fk.Columns {
			if idx.Parts[i].C == nil || idx.Parts[i].C.Name != c.Name {
				continue search
			}
		}
		return true
	}
	return false
}
