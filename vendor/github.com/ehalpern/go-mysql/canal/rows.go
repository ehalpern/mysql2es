package canal

import (
	"fmt"

	"github.com/ehalpern/go-mysql/schema"
	"github.com/juju/errors"
)

const (
	UpdateAction = "update"
	InsertAction = "insert"
	DeleteAction = "delete"
)

type RowsEvent struct {
	Table  *schema.Table
	Action string
	// changed row list
	// binlog has three update event version, v0, v1 and v2.
	// for v1 and v2, the rows number must be even.
	// Two rows for one event, format is [before update row, after update row]
	// for update v0, only one row for a event, and we don't support this version.
	Rows [][]interface{}
}

func newRowsEvent(table *schema.Table, action string, rows [][]interface{}) *RowsEvent {
	e := new(RowsEvent)

	e.Table = table
	e.Action = action
	e.Rows = rows

	return e
}

// Get primary keys in one row for a table, a table may use multi fields as the PK
func GetPKValues(table *schema.Table, row []interface{}) ([]interface{}, error) {
	indexes := table.PKColumns
	if len(indexes) == 0 {
		return nil, errors.Errorf("table %s has no PK", table)
	} else if len(table.Columns) < len(row) {   // Ok if schema has added a column
		return nil, errors.Errorf("table %s has %d columns, but row data %v len is %d", table,
			len(table.Columns), row, len(row))
	}

	values := make([]interface{}, 0, len(indexes))

	for _, index := range indexes {
		keyPart := fmt.Sprintf("%v", row[index])
		if keyPart == "" {
			return nil, errors.Errorf("row in %s has no PK: %v", table, row)
		}
		values = append(values, row[index])
	}

	return values, nil
}
