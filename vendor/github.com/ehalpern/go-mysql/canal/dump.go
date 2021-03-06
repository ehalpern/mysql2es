package canal

import (
	"strconv"
	"time"

	"github.com/juju/errors"
	"github.com/ehalpern/go-mysql/dump"
	"github.com/ehalpern/go-mysql/schema"
	"github.com/siddontang/go/log"
)

type dumpParseHandler struct {
	c    *Canal
	name string
	pos  uint64
}

func (h *dumpParseHandler) BinLog(name string, pos uint64) error {
	h.name = name
	h.pos = pos
	return nil
}

func (h *dumpParseHandler) Data(db string, table string, values []string) error {
	if h.c.isClosed() {
		return errCanalClosed
	}

	tableInfo, err := h.c.GetTable(db, table)
	if err != nil {
		log.Errorf("get %s.%s information err: %v", db, table, err)
		return errors.Trace(err)
	}

	vs := make([]interface{}, len(values))
	log.Debugf("Handling Data: %v", values)
	for i, v := range values {
		if v == "NULL" {
			vs[i] = nil
		} else if firstChar := v[0]; firstChar == '\'' || firstChar == '"' {
			vs[i] = v[1 : len(v) - 1]
		} else {
			if tableInfo.Columns[i].Type == schema.TYPE_NUMBER {
				n, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					log.Errorf("parse row %v at %d error %v, skip", values, i, err)
					return dump.ErrSkip
				}
				vs[i] = n
			} else if tableInfo.Columns[i].Type == schema.TYPE_FLOAT {
				f, err := strconv.ParseFloat(v, 64)
				if err != nil {
					log.Errorf("parse row %v at %d error %v, skip", values, i, err)
					return dump.ErrSkip
				}
				vs[i] = f
			} else {
				log.Errorf("parse row %v at %d err: invalid type %v for value %v, skip", values, i, tableInfo.Columns[i].Type, v)
				return dump.ErrSkip
			}
		}
	}

	events := newRowsEvent(tableInfo, InsertAction, [][]interface{}{vs})
	return h.c.travelRowsEventHandler(events)
}

func (h *dumpParseHandler) Complete() error {
	for _, handler := range h.c.rsHandlers {
		if err := handler.Complete(); err != nil {
			return err
		}
	}
	return nil
}


func (c *Canal) AddDumpDatabases(dbs ...string) {
	if c.dumper == nil {
		return
	}

	c.dumper.AddDatabases(dbs...)
}

func (c *Canal) AddDumpTables(db string, tables ...string) {
	if c.dumper == nil {
		return
	}

	c.dumper.AddTables(db, tables...)
}

func (c *Canal) AddDumpIgnoreTables(db string, tables ...string) {
	if c.dumper == nil {
		return
	}

	c.dumper.AddIgnoreTables(db, tables...)
}

func (c *Canal) tryDump() error {
	if len(c.master.Name) > 0 {
		// we will sync with binlog name and position
		log.Infof("Skip dump, use last binlog replication pos (%s, %d)", c.master.Name, c.master.Position)
		return nil
	}
	if c.dumper == nil {
		log.Errorf("Skip dump, no dumper provided")
		return nil
	}

	h := &dumpParseHandler{c: c}
	start := time.Now()
	log.Info("Start dump")
	if err := c.dumper.DumpAndParse(h); err != nil {
		return errors.Trace(err)
	}

	log.Infof("Dump completed in %0.2f seconds", time.Now().Sub(start).Seconds())

	c.master.Update(h.name, uint32(h.pos))
	c.master.Save(true)
	return nil
}
