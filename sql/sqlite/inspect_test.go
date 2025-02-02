// Copyright 2021-present The Atlas Authors. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package sqlite

import (
	"context"
	"fmt"
	"testing"

	"ariga.io/atlas/sql/internal/sqltest"
	"ariga.io/atlas/sql/schema"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestDriver_InspectTable(t *testing.T) {
	tests := []struct {
		name   string
		opts   *schema.InspectTableOptions
		before func(mock)
		expect func(*require.Assertions, *schema.Table, error)
	}{
		{
			name: "table does not exist",
			before: func(m mock) {
				m.systemVars("3.36.0")
				m.tableExists("users", false)
			},
			expect: func(require *require.Assertions, t *schema.Table, err error) {
				require.Nil(t)
				require.Error(err)
				require.True(schema.IsNotExistError(err), "expect not exists error: %v", err)
			},
		},
		{
			name: "table columns",
			before: func(m mock) {
				m.systemVars("3.36.0")
				m.tableExists("users", true, "CREATE TABLE users(id INTEGER PRIMARY KEY AUTOINCREMENT)")
				m.ExpectQuery(sqltest.Escape(fmt.Sprintf(columnsQuery, "users"))).
					WillReturnRows(sqltest.Rows(`
 name |   type       | nullable | dflt_value  | primary 
------+--------------+----------+ ------------+----------
 c1   | int           |  1      |             |  0
 c2   | integer       |  0      |             |  0
 c3   | varchar(100)  |  1      |             |  0
 c4   | boolean       |  0      |             |  0
 c5   | json          |  0      |             |  0
 c6   | datetime      |  0      |             |  0
 c7   | blob          |  0      |             |  0
 c8   | text          |  0      |             |  0
 c9   | numeric(10,2) |  0      |             |  0
 c10  | real          |  0      |             |  0
 id   | integer       |  0      |             |  1
`))
				m.noIndexes("users")
				m.noFKs("users")
			},
			expect: func(require *require.Assertions, t *schema.Table, err error) {
				require.NoError(err)
				columns := []*schema.Column{
					{Name: "c1", Type: &schema.ColumnType{Null: true, Type: &schema.IntegerType{T: "int"}, Raw: "int"}},
					{Name: "c2", Type: &schema.ColumnType{Type: &schema.IntegerType{T: "integer"}, Raw: "integer"}},
					{Name: "c3", Type: &schema.ColumnType{Null: true, Type: &schema.StringType{T: "varchar", Size: 100}, Raw: "varchar(100)"}},
					{Name: "c4", Type: &schema.ColumnType{Type: &schema.BoolType{T: "boolean"}, Raw: "boolean"}},
					{Name: "c5", Type: &schema.ColumnType{Type: &schema.JSONType{T: "json"}, Raw: "json"}},
					{Name: "c6", Type: &schema.ColumnType{Type: &schema.TimeType{T: "datetime"}, Raw: "datetime"}},
					{Name: "c7", Type: &schema.ColumnType{Type: &schema.BinaryType{T: "blob"}, Raw: "blob"}},
					{Name: "c8", Type: &schema.ColumnType{Type: &schema.StringType{T: "text"}, Raw: "text"}},
					{Name: "c9", Type: &schema.ColumnType{Type: &schema.DecimalType{T: "numeric", Precision: 10, Scale: 2}, Raw: "numeric(10,2)"}},
					{Name: "c10", Type: &schema.ColumnType{Type: &schema.FloatType{T: "real"}, Raw: "real"}},
					{Name: "id", Type: &schema.ColumnType{Type: &schema.IntegerType{T: "integer"}, Raw: "integer"}, Attrs: []schema.Attr{AutoIncrement{}}},
				}
				require.Equal(t.Columns, columns)
				require.EqualValues(&schema.Index{
					Name:   "PRIMARY",
					Unique: true,
					Table:  t,
					Parts:  []*schema.IndexPart{{SeqNo: 1, C: columns[len(columns)-1]}},
					Attrs:  []schema.Attr{AutoIncrement{}},
				}, t.PrimaryKey)
			},
		},
		{
			name: "table indexes",
			before: func(m mock) {
				m.systemVars("3.36.0")
				m.tableExists("users", true, "CREATE TABLE users(id INTEGER PRIMARY KEY)")
				m.ExpectQuery(sqltest.Escape(fmt.Sprintf(columnsQuery, "users"))).
					WillReturnRows(sqltest.Rows(`
 name |   type       | nullable | dflt_value  | primary 
------+--------------+----------+ ------------+----------
 c1   | int           |  1      |             |  0
 c2   | integer       |  0      |             |  0
`))
				m.ExpectQuery(sqltest.Escape(fmt.Sprintf(indexesQuery, "users"))).
					WillReturnRows(sqltest.Rows(`
 name  |   unique     | origin | partial  |                      sql 
-------+--------------+--------+----------+-------------------------------------------------------
 c1u   |  1           |  c     |  0       | CREATE UNIQUE INDEX c1u on users(c1, c2)
 c1_c2 |  0           |  c     |  1       | CREATE INDEX c1_c2 on users(c1, c2*2) WHERE c1 <> NULL
`))
				m.ExpectQuery(sqltest.Escape(fmt.Sprintf(indexColumnsQuery, "c1u"))).
					WillReturnRows(sqltest.Rows(`
 name 
------
 c1   
 c2
`))
				m.ExpectQuery(sqltest.Escape(fmt.Sprintf(indexColumnsQuery, "c1_c2"))).
					WillReturnRows(sqltest.Rows(`
 name 
------
 c1   
 nil
`))
				m.noFKs("users")
			},
			expect: func(require *require.Assertions, t *schema.Table, err error) {
				require.NoError(err)
				columns := []*schema.Column{
					{Name: "c1", Type: &schema.ColumnType{Null: true, Type: &schema.IntegerType{T: "int"}, Raw: "int"}},
					{Name: "c2", Type: &schema.ColumnType{Type: &schema.IntegerType{T: "integer"}, Raw: "integer"}},
				}
				indexes := []*schema.Index{
					{
						Name:   "c1u",
						Unique: true,
						Table:  t,
						Parts: []*schema.IndexPart{
							{SeqNo: 1, C: columns[0]},
							{SeqNo: 2, C: columns[1]},
						},
						Attrs: []schema.Attr{
							&CreateStmt{S: "CREATE UNIQUE INDEX c1u on users(c1, c2)"},
							&IndexOrigin{O: "c"},
						},
					},
					{
						Name:  "c1_c2",
						Table: t,
						Parts: []*schema.IndexPart{
							{SeqNo: 1, C: columns[0]},
							{SeqNo: 2, X: &schema.RawExpr{X: "<unsupported>"}},
						},
						Attrs: []schema.Attr{
							&CreateStmt{S: "CREATE INDEX c1_c2 on users(c1, c2*2) WHERE c1 <> NULL"},
							&IndexOrigin{O: "c"},
							&IndexPredicate{P: "c1 <> NULL"},
						},
					},
				}
				require.Equal(t.Columns, columns)
				require.Equal(t.Indexes, indexes)
			},
		},
		{
			name: "table fks",
			before: func(m mock) {
				m.systemVars("3.36.0")
				m.tableExists("users", true, `
CREATE TABLE users(
	id INTEGER PRIMARY KEY,
	c1 int,
	c2 integer NOT NULL CONSTRAINT c2_fk REFERENCES users (c1) ON DELETE SET NULL,
	c3 integer NOT NULL REFERENCES users (c1) ON DELETE SET NULL,
	CONSTRAINT "c1_c2_fk" FOREIGN KEY (c1, c2) REFERENCES t2 (id, c1)
)
`)
				m.ExpectQuery(sqltest.Escape(fmt.Sprintf(columnsQuery, "users"))).
					WillReturnRows(sqltest.Rows(`
 name |   type       | nullable | dflt_value  | primary 
------+--------------+----------+ ------------+----------
 c1   | int           |  1      |             |  0
 c2   | integer       |  0      |             |  0
 c3   | integer       |  0      |             |  0
`))
				m.noIndexes("users")
				m.ExpectQuery(sqltest.Escape(fmt.Sprintf(fksQuery, "users"))).
					WillReturnRows(sqltest.Rows(`
 id |   from    | to | table  | on_update   | on_delete   
----+-----------+-------------+-------------+-----------
 0  | c1        | id | t2     |  NO ACTION  | CASCADE
 0  | c2        | c1 | t2     |  NO ACTION  | CASCADE
 1  | c2        | c1 | users  |  NO ACTION  | CASCADE
`))
			},
			expect: func(require *require.Assertions, t *schema.Table, err error) {
				require.NoError(err)
				fks := []*schema.ForeignKey{
					{Symbol: "c1_c2_fk", Table: t, OnUpdate: schema.NoAction, OnDelete: schema.Cascade, RefTable: &schema.Table{Name: "t2", Schema: &schema.Schema{Name: "main"}}, RefColumns: []*schema.Column{{Name: "id"}, {Name: "c1"}}},
					{Symbol: "c2_fk", Table: t, OnUpdate: schema.NoAction, OnDelete: schema.Cascade, RefTable: t},
				}
				columns := []*schema.Column{
					{Name: "c1", Type: &schema.ColumnType{Null: true, Type: &schema.IntegerType{T: "int"}, Raw: "int"}, ForeignKeys: fks[:1]},
					{Name: "c2", Type: &schema.ColumnType{Type: &schema.IntegerType{T: "integer"}, Raw: "integer"}, ForeignKeys: fks},
					{Name: "c3", Type: &schema.ColumnType{Type: &schema.IntegerType{T: "integer"}, Raw: "integer"}},
				}
				fks[0].Columns = columns[:2]
				fks[1].Columns = columns[1:2]
				fks[1].RefColumns = columns[:1]
				require.Equal(t.Columns, columns)
				require.Equal(t.ForeignKeys, fks)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, m, err := sqlmock.New()
			require.NoError(t, err)
			tt.before(mock{m})
			drv, err := Open(db)
			require.NoError(t, err)
			table, err := drv.InspectTable(context.Background(), "users", tt.opts)
			tt.expect(require.New(t), table, err)
		})
	}
}

func TestRegex_TableConstraint(t *testing.T) {
	tests := []struct {
		input   string
		matches []string
	}{
		{
			input:   `CREATE TABLE pets (id int NOT NULL, owner_id int, CONSTRAINT "owner_fk" FOREIGN KEY(owner_id) REFERENCES users(id))`,
			matches: []string{"owner_fk", "owner_id", "users", "id"},
		},
		{
			input:   `CREATE TABLE pets (id int NOT NULL, owner_id int, CONSTRAINT "owner_fk" FOREIGN KEY (owner_id) REFERENCES users(id))`,
			matches: []string{"owner_fk", "owner_id", "users", "id"},
		},
		{
			input: `
CREATE TABLE pets (
id int NOT NULL,
owner_id int,
CONSTRAINT owner_fk
	FOREIGN KEY ("owner_id") REFERENCES "users" (id)
)`,
			matches: []string{"owner_fk", `"owner_id"`, "users", "id"},
		},
		{
			input: `
CREATE TABLE pets (
id int NOT NULL,
c int,
d int,
CONSTRAINT "c_d_fk" FOREIGN KEY (c, d) REFERENCES "users" (a, b)
)`,
			matches: []string{"c_d_fk", "c, d", "users", "a, b"},
		},
		{
			input:   `CREATE TABLE pets (id int NOT NULL,c int,d int,CONSTRAINT "c_d_fk" FOREIGN KEY (c, "d") REFERENCES "users" (a, "b"))`,
			matches: []string{"c_d_fk", `c, "d"`, "users", `a, "b"`},
		},
		{
			input: `CREATE TABLE pets (id int NOT NULL,c int,d int,CONSTRAINT FOREIGN KEY (c, "d") REFERENCES "users" (a, "b"))`,
		},
		{
			input: `CREATE TABLE pets (id int NOT NULL,c int,d int,CONSTRAINT name FOREIGN KEY c REFERENCES "users" (a, "b"))`,
		},
		{
			input: `CREATE TABLE pets (id int NOT NULL,c int,d int,CONSTRAINT name FOREIGN KEY c REFERENCES (a, "b"))`,
		},
	}
	for _, tt := range tests {
		m := reConstT.FindStringSubmatch(tt.input)
		require.Equal(t, len(m) != 0, len(tt.matches) != 0)
		if len(m) > 0 {
			require.Equal(t, tt.matches, m[1:])
		}
	}
}

func TestRegex_ColumnConstraint(t *testing.T) {
	tests := []struct {
		input   string
		matches []string
	}{
		{
			input:   `CREATE TABLE pets (id int NOT NULL, owner_id int CONSTRAINT "owner_fk" REFERENCES users(id))`,
			matches: []string{"owner_id", "owner_fk", "users", "id"},
		},
		{
			input:   `CREATE TABLE pets (id int NOT NULL, owner_id int CONSTRAINT "owner_fk" REFERENCES users(id))`,
			matches: []string{"owner_id", "owner_fk", "users", "id"},
		},
		{
			input: `
CREATE TABLE pets (
	id int NOT NULL,
	c int REFERENCES users(id),
	d int CONSTRAINT "dfk" REFERENCES users(id)
)`,
			matches: []string{"d", "dfk", "users", "id"},
		},
		{
			input: `
CREATE TABLE t1 (
	c int REFERENCES users(id),
	d text CONSTRAINT "dfk" CHECK (d <> '') REFERENCES t2(d)
)`,
		},
	}
	for _, tt := range tests {
		m := reConstC.FindStringSubmatch(tt.input)
		require.Equal(t, len(m) != 0, len(tt.matches) != 0)
		if len(m) > 0 {
			require.Equal(t, tt.matches, m[1:])
		}
	}
}

type mock struct {
	sqlmock.Sqlmock
}

func (m mock) systemVars(version string) {
	m.ExpectQuery(sqltest.Escape("SELECT sqlite_version()")).
		WillReturnRows(sqltest.Rows(`
  version   
------------
 ` + version + `
`))
	m.ExpectQuery(sqltest.Escape("PRAGMA foreign_keys")).
		WillReturnRows(sqltest.Rows(`
  foreign_keys   
------------
    1
`))
	m.ExpectQuery(sqltest.Escape("SELECT name FROM pragma_collation_list()")).
		WillReturnRows(sqltest.Rows(`
  pragma_collation_list   
------------------------
      decimal
      uint
      RTRIM
      NOCASE
      BINARY
`))
	m.ExpectQuery(sqltest.Escape(databasesQuery + " WHERE name IN (?)")).
		WithArgs("main").
		WillReturnRows(sqltest.Rows(`
 name |   file    
------+-----------
 main |   
`))
}

func (m mock) tableExists(table string, exists bool, stmt ...string) {
	rows := sqlmock.NewRows([]string{"name", "sql"})
	if exists {
		rows.AddRow(table, stmt[0])
	}
	m.ExpectQuery(sqltest.Escape(tablesQuery + " AND name IN (?)")).
		WithArgs(table).
		WillReturnRows(rows)
}

func (m mock) noIndexes(table string) {
	m.ExpectQuery(sqltest.Escape(fmt.Sprintf(indexesQuery, table))).
		WillReturnRows(sqlmock.NewRows([]string{"name", "unique", "origin", "partial", "sql"}))
}

func (m mock) noFKs(table string) {
	m.ExpectQuery(sqltest.Escape(fmt.Sprintf(fksQuery, table))).
		WillReturnRows(sqlmock.NewRows([]string{"id", "from", "to", "table", "on_update", "on_delete"}))
}
