package docker

import (
	"context"
	"io"
	"io/ioutil"
	"sync"

	"github.com/docker/docker/api/types"
)

type pullFuture struct {
	waitCh chan struct{}

	err     error
	imageID string
}

func newPullFuture() *pullFuture {
	return &pullFuture{
		waitCh: make(chan struct{}),
	}
}

func (p *pullFuture) wait() *pullFuture {
	<-p.waitCh
	return p
}

func (p *pullFuture) result() (imageID string, err error) {
	return p.imageID, p.err
}

func (p *pullFuture) set(imageID string, err error) {
	p.imageID = imageID
	p.err = err
	close(p.waitCh)
}

type dockerImageCoordinator struct {
	client      dockerClient
	imageLock   sync.Mutex
	pullFutures map[string]*pullFuture
}

type dockerClient interface {
	ImagePull(ctx context.Context, refStr string, options types.ImagePullOptions) (io.ReadCloser, error)
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
}

func newDockerImageCoordinator(client dockerClient) *dockerImageCoordinator {
	return &dockerImageCoordinator{
		client:      client,
		pullFutures: map[string]*pullFuture{},
	}
}

func (d *dockerImageCoordinator) PullImage(image string) (string, error) {
	d.imageLock.Lock()
	future, ok := d.pullFutures[image]
	if !ok {
		future = newPullFuture()
		d.pullFutures[image] = future

		go d.pullImageImpl(image, future)
	}
	d.imageLock.Unlock()

	id, err := future.wait().result()

	d.imageLock.Lock()
	defer d.imageLock.Unlock()

	delete(d.pullFutures, image)

	return id, err
}

func (d *dockerImageCoordinator) pullImageImpl(image string, future *pullFuture) {
	reader, err := d.client.ImagePull(context.Background(), image, types.ImagePullOptions{})
	if err != nil {
		future.set("", err)
		return
	}
	if _, err = io.Copy(ioutil.Discard, reader); err != nil {
		future.set("", err)
		return
	}

	dockerImage, _, err := d.client.ImageInspectWithRaw(context.Background(), image)
	if err != nil {
		future.set("", err)
		return
	}

	future.set(dockerImage.ID, nil)
}
