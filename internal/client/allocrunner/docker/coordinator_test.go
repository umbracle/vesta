package docker

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/umbracle/vesta/internal/uuid"
)

type mockClient struct {
	idMap map[string]string
	delay time.Duration
	pulls int
}

func (m *mockClient) ImagePull(ctx context.Context, refStr string, options types.ImagePullOptions) (io.ReadCloser, error) {
	time.Sleep(m.delay)
	m.pulls++

	reader := io.NopCloser(strings.NewReader(""))
	return reader, nil
}

func (m *mockClient) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
	id, ok := m.idMap[imageID]
	if !ok {
		return types.ImageInspect{}, nil, fmt.Errorf("id not found %s", imageID)
	}
	return types.ImageInspect{ID: id}, nil, nil
}

func (m *mockClient) setID(image string) string {
	id := uuid.Generate()
	m.idMap[image] = id
	return id
}

func newMockClient(delay time.Duration) *mockClient {
	return &mockClient{idMap: map[string]string{}, delay: delay}
}

func TestCoordinator_ConcurrentPulls(t *testing.T) {
	image := "image"

	client := newMockClient(2 * time.Second)
	id := client.setID(image)

	c := newDockerImageCoordinator(client)

	num := 10
	doneCh := make(chan string, num)

	for i := 0; i < num; i++ {
		go func(t *testing.T) {
			res, err := c.PullImage(image)
			assert.NoError(t, err)

			doneCh <- res
		}(t)
	}

	for i := 0; i < num; i++ {
		res := <-doneCh
		assert.Equal(t, res, id)
	}

	assert.Equal(t, client.pulls, 1)
}
