package state

import "github.com/hashicorp/go-memdb"

var schema = &memdb.DBSchema{
	Tables: map[string]*memdb.TableSchema{
		"deployments": {
			Name: "deployments",
			Indexes: map[string]*memdb.IndexSchema{
				"id": {
					Name:    "id",
					Unique:  true,
					Indexer: &memdb.StringFieldIndex{Field: "Id"},
				},
			},
		},
		"allocations": {
			Name: "allocations",
			Indexes: map[string]*memdb.IndexSchema{
				"id": {
					Name:    "id",
					Unique:  true,
					Indexer: &memdb.StringFieldIndex{Field: "Id"},
				},
				"nodeId": {
					Name:    "nodeId",
					Indexer: &memdb.StringFieldIndex{Field: "NodeId"},
				},
			},
		},
		"data": {
			Name: "data",
			Indexes: map[string]*memdb.IndexSchema{
				"id": {
					Name:    "id",
					Unique:  true,
					Indexer: &memdb.StringFieldIndex{Field: "Id"},
				},
			},
		},
	},
}
