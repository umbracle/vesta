package mock

import "github.com/umbracle/vesta/internal/server/proto"

func Task() *proto.Task {
	return &proto.Task{
		Image: "vesta",
		Tag:   "latest",
		Args: []string{
			"a",
		},
		Env: map[string]string{
			"b": "c",
		},
	}
}
