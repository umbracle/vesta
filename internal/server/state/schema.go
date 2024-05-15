package state

import "github.com/hashicorp/go-memdb"

var schema = &memdb.DBSchema{
	Tables: map[string]*memdb.TableSchema{
		"events": {
			Name: "events",
			Indexes: map[string]*memdb.IndexSchema{
				"id": {
					Name:    "id",
					Unique:  true,
					Indexer: &memdb.StringFieldIndex{Field: "Id"},
				},
				"service": {
					Name:    "service",
					Indexer: &memdb.StringFieldIndex{Field: "Service"},
				},
			},
		},
	},
}
