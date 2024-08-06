package state2

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/server/proto"
)

func TestState(t *testing.T) {
	s := newTestState(t)
	fmt.Println(s)

	dep := &proto.Deployment2{
		Id:   "1",
		Name: "test",
		Spec: []byte("spec"),
	}
	err := s.CreateDeployment(dep)
	require.NoError(t, err)

	deployments, err := s.ListDeployments()
	require.NoError(t, err)
	require.Len(t, deployments, 1)

	event := &proto.Event2{
		Id:         "1",
		Task:       "a",
		Deployment: "2",
		Type:       "create",
	}
	require.Error(t, s.CreateEvent(event))

	event.Deployment = "1"
	require.NoError(t, s.CreateEvent(event))
}

func newTestState(t *testing.T) *State {
	s, err := NewState(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return s
}
