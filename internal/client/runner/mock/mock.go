package mock

import proto "github.com/umbracle/vesta/internal/client/runner/structs"

func Task1() *proto.Task {
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

func ServiceAlloc() *proto.Allocation {
	return &proto.Allocation{
		Deployment: &proto.Deployment{
			Tasks: []*proto.Task{
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
