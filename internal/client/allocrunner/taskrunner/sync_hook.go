package taskrunner

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/hashicorp/go-hclog"
	babel "github.com/umbracle/babel/sdk"
	"github.com/umbracle/vesta/internal/server/proto"
	"google.golang.org/grpc"
)

type syncHook struct {
	logger  hclog.Logger
	closeCh chan struct{}
}

func newSyncHook(logger hclog.Logger, task *proto.Task) *syncHook {
	h := &syncHook{
		closeCh: make(chan struct{}),
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (m *syncHook) Name() string {
	return "sync-hook"
}

func (m *syncHook) PostStart() {
	go m.collectMetrics()
}

func (m *syncHook) collectMetrics() {
	conn, err := grpc.Dial("localhost:2020", grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	clt := babel.NewBabelServiceClient(conn)

	for {
		resp, err := clt.GetSyncStatus(context.Background(), &empty.Empty{})
		if err != nil {
			m.logger.Error("failed to get sync status", "err", err)
		} else {
			fmt.Println("-- resp --")
			fmt.Println(resp)
		}

		select {
		case <-m.closeCh:
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (m *syncHook) Stop() {
	close(m.closeCh)
}
