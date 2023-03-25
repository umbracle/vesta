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

func Task1() *proto.Task1 {
	return &proto.Task1{
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

func ServiceAlloc() *proto.Allocation1 {
	return &proto.Allocation1{
		Deployment: &proto.Deployment1{
			Tasks: []*proto.Task1{
				{
					Name:  "a",
					Image: "busybox",
					Tag:   "1.29.3",
					Args:  []string{"sleep", "30"},
				},
				{
					Name:  "b",
					Image: "busybox",
					Tag:   "1.29.3",
					Args:  []string{"sleep", "30"},
				},
			},
		},
	}
}
