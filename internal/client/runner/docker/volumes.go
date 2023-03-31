package docker

import (
	"context"

	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

func (d *Docker) CreateVolume(volumeID string) (bool, error) {
	// check if the volume exists already
	_, err := d.client.VolumeInspect(context.Background(), volumeID)
	if err == nil {
		// object exists, do not create it
		return false, nil
	}
	if !client.IsErrNotFound(err) {
		// any other error
		return false, err
	}

	opts := volume.VolumeCreateBody{
		Driver: "local",
		Name:   volumeID,
	}
	if _, err := d.client.VolumeCreate(context.Background(), opts); err != nil {
		return false, err
	}
	return true, nil
}

func (d *Docker) DeleteVolume(volumeID string) error {
	return d.client.VolumeRemove(context.Background(), volumeID, true)
}
