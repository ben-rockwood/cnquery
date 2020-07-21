package snapshot

import (
	"context"
	"os"

	"github.com/rs/zerolog/log"
	"go.mondoo.io/mondoo/motor/runtime"
	"go.mondoo.io/mondoo/motor/transports"
	"go.mondoo.io/mondoo/motor/transports/docker/cache"
	"go.mondoo.io/mondoo/motor/transports/docker/docker_engine"
	"go.mondoo.io/mondoo/motor/transports/tar"
	"go.mondoo.io/mondoo/nexus/assets"
)

type DockerSnapshotTransport struct {
	tar.Transport
}

func (t *DockerSnapshotTransport) Kind() assets.Kind {
	return assets.Kind_KIND_CONTAINER
}

func (t *DockerSnapshotTransport) Runtime() string {
	return runtime.RUNTIME_DOCKER_CONTAINER
}

func new(endpoint *transports.Endpoint) (*DockerSnapshotTransport, error) {
	return newWithClose(endpoint, nil)
}

func newWithClose(endpoint *transports.Endpoint, close func()) (*DockerSnapshotTransport, error) {
	t := &DockerSnapshotTransport{
		Transport: tar.Transport{
			Fs:      tar.NewFs(endpoint.Path),
			CloseFN: close,
		},
	}

	var err error
	if endpoint != nil && len(endpoint.Path) > 0 {
		err := t.LoadFile(endpoint.Path)
		if err != nil {
			log.Error().Err(err).Str("tar", endpoint.Path).Msg("tar> could not load tar file")
			return nil, err
		}
	}
	return t, err
}

func NewFromDockerEngine(containerid string) (*DockerSnapshotTransport, error) {
	// cache container on local disk
	f, err := cache.RandomFile()
	if err != nil {
		return nil, err
	}

	err = Export(containerid, f)
	if err != nil {
		return nil, err
	}

	return newWithClose(&transports.Endpoint{Path: f.Name()}, func() {
		// remove temporary file on stream close
		os.Remove(f.Name())
	})
}

func NewFromFile(filename string) (*DockerSnapshotTransport, error) {
	return new(&transports.Endpoint{Path: filename})
}

// exports a given container from docker engine to a tar file
func Export(containerid string, f *os.File) error {
	dc, err := docker_engine.GetDockerClient()
	if err != nil {
		return err
	}

	rc, err := dc.ContainerExport(context.Background(), containerid)
	if err != nil {
		return err
	}

	return cache.StreamToTmpFile(rc, f)
}