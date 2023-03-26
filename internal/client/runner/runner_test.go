package runner

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/vesta/internal/client/runner/state"
	proto "github.com/umbracle/vesta/internal/client/runner/structs"
	"github.com/umbracle/vesta/internal/uuid"
)

func randomInt(min, max int) int {
	return min + rand.Intn(max-min)
}

var fuzzTypeActions = []string{
	"deploy",
	"alloc_task_add",
	"alloc_task_del",
	"alloc_destroy",
	"deattach",
}

func longRunningTask(name string) *proto.Task {
	return &proto.Task{
		Name:  name,
		Image: "busybox",
		Tag:   "1.29.3",
		Args:  []string{"sleep", "3000"},
	}
}

type fuzzUpdater struct {
}

func (f *fuzzUpdater) AllocStateUpdated(alloc *proto.Allocation) {
}

func TestRunner_Fuzz(t *testing.T) {
	t.Skip("")

	// rand.Seed(time.Now().UTC().UnixNano())

	// use the same state to represent all the execution
	tmpDir, err := ioutil.TempDir("/tmp", "fuzz-task-runner-")
	assert.NoError(t, err)

	boltDbPath := filepath.Join(tmpDir, "my.db")

	// runner might be empty to test the 'restore' action
	var runner *Runner

	maxNumberOfAllocs := 5

	timer := time.NewTimer(2 * time.Second)

	for i := 0; i < 7; i++ {
		fmt.Println("=> ITER", i)

		// if the runner is not declared, do it first
		if runner == nil {
			state, err := state.NewBoltdbStore(boltDbPath)
			assert.NoError(t, err)

			config := &Config{
				State:             state,
				AllocStateUpdated: &fuzzUpdater{},
			}
			r, err := NewRunner(config)
			require.NoError(t, err)

			runner = r
		} else {
			allocs := runner.allocs
			retryCount := 0

			timer.Reset(2 * time.Second)
			go func() {
				<-timer.C
				panic("cxx")
			}()

		RETRY:
			retryCount++
			if retryCount == 10 {
				t.Fatal("too many retries")
			}

			// otherwise, perform any of the next actions
			actTyp := fuzzTypeActions[rand.Intn(len(fuzzTypeActions))]

			fmt.Println("=> run action", actTyp)

			if actTyp == "deploy" {
				// deploy a new task, up to 'maxNumberOfAllocs' tasks
				if len(allocs) >= maxNumberOfAllocs {
					goto RETRY
				}

				runner.UpsertDeployment(&proto.Deployment{
					Name: uuid.Generate(),
					Tasks: []*proto.Task{
						longRunningTask(uuid.Generate()),
					},
				})

			} else if actTyp == "deattach" {
				// de-attach the runner
				runner.Shutdown()
				runner = nil

			} else if strings.HasPrefix(actTyp, "alloc_") {

				// alloc specific actions, there has to be at least
				// one alloc available
				if len(allocs) == 0 {
					goto RETRY
				}

				alloc := allocs[randomKeyMap(allocs)]
				dep := alloc.Deployment()

				if actTyp == "alloc_task_add" {
					// add a new task to the allocation
					tt := longRunningTask(uuid.Generate())

					dep.Tasks = append(dep.Tasks, tt)
					dep.Sequence++

					runner.UpsertDeployment(dep)

				} else if actTyp == "alloc_task_del" {
					// remove a task from the allocation. Only available if
					// two or more tasks exists
					if len(dep.Tasks) == 1 {
						goto RETRY
					}

					indx := rand.Intn(len(dep.Tasks))
					dep.Tasks = append(dep.Tasks[:indx], dep.Tasks[indx+1:]...)

					runner.UpsertDeployment(dep)

				} else if actTyp == "alloc_destroy" {
					// destroy the allocation
					alloc.Destroy()

				} else {
					panic(fmt.Sprintf("BUG: action '%s' not found", actTyp))
				}
			} else {
				panic(fmt.Sprintf("BUG: action '%s' not found", actTyp))
			}

			// time.Sleep(2 * time.Second)
		}
	}
}

func randomKeyMap(mapI interface{}) string {
	keys := reflect.ValueOf(mapI).MapKeys()
	return keys[rand.Intn(len(keys))].Interface().(string)
}
