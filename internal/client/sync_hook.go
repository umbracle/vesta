package client

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/hashicorp/go-hclog"
	babel "github.com/umbracle/babel/sdk"
	"github.com/umbracle/vesta/internal/client/runner/hooks"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
	"google.golang.org/grpc"
)

type syncStateUpdater interface {
	UpdateSyncState(alloc, task string, status *babel.SyncStatus)
}

var _ hooks.TaskHook = &syncHook{}
var _ hooks.TaskPoststartHook = &syncHook{}
var _ hooks.TaskStopHook = &metricsHook{}

type syncHook struct {
	logger           hclog.Logger
	task             *proto.Task
	alloc            string
	closeCh          chan struct{}
	ip               string
	syncStateUpdater syncStateUpdater
}

func newSyncHook(logger hclog.Logger, alloc string, task *proto.Task, syncStateUpdater syncStateUpdater) *syncHook {
	h := &syncHook{
		closeCh:          make(chan struct{}),
		task:             task,
		alloc:            alloc,
		syncStateUpdater: syncStateUpdater,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (m *syncHook) Name() string {
	return "sync-hook"
}

func (m *syncHook) Poststart(ctx context.Context, req *hooks.TaskPoststartHookRequest) error {
	if req.Spec.Ip == "" {
		return nil
	}

	// only track if the name of the task is babel
	if m.task.Name != "babel" {
		return nil
	}

	m.ip = req.Spec.Ip
	go m.collectMetrics()

	return nil
}

func (m *syncHook) collectMetrics() {
	conn, err := grpc.Dial(fmt.Sprintf("%s:2020", m.ip), grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	clt := babel.NewBabelServiceClient(conn)

	for {
		resp, err := clt.GetSyncStatus(context.Background(), &empty.Empty{})
		if err != nil {
			m.logger.Error("failed to get sync status", "err", err)
		} else {
			m.syncStateUpdater.UpdateSyncState(m.alloc, m.task.Name, resp)
		}

		select {
		case <-m.closeCh:
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (m *syncHook) Stop(ctx context.Context, req *hooks.TaskStopRequest) error {
	close(m.closeCh)
	return nil
}
