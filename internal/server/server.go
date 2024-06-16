package server

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/mapstructure"
	"github.com/umbracle/vesta/internal/backend/docker"
	"github.com/umbracle/vesta/internal/catalog"
	"github.com/umbracle/vesta/internal/jsonnet"
	"github.com/umbracle/vesta/internal/schema"
	"github.com/umbracle/vesta/internal/server/proto"
	"github.com/umbracle/vesta/internal/server/state"
	"github.com/umbracle/vesta/internal/uuid"
)

type Config struct {
	PersistentDB *bolt.DB
	Catalog      []string
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{}
}

type Catalog interface {
	GetFields(id string, input []byte) (*schema.FieldData, []proto.VolumeStub, error)
	Build(prev []byte, req *proto.ApplyRequest) (*schema.FieldData, *proto.Service, error)
	Build2(name string, data *schema.FieldData) *proto.Service
	ListPlugins() []string
	GetPlugin(name string) (*proto.Item, map[string]proto.VolumeStub, error)
	ValidateFn(plugin string, validationFn string, config, obj interface{}) bool
}

type Server struct {
	logger  hclog.Logger
	state   *state.StateStore
	catalog Catalog
	backend *docker.Docker
}

func NewServer(logger hclog.Logger, config *Config) (*Server, error) {
	var statedb *state.StateStore

	if config.PersistentDB != nil {
		s, err := state.NewStateStoreWithBoltDB(config.PersistentDB)
		if err != nil {
			return nil, err
		}
		statedb = s

	} else {
		s, err := state.NewStateStore("server.db")
		if err != nil {
			return nil, err
		}
		statedb = s
	}

	catalog, err := catalog.NewCatalog()
	if err != nil {
		return nil, err
	}

	catalog.SetLogger(logger)

	// load the custom catalogs
	for _, ctg := range config.Catalog {
		if err := catalog.Load(ctg); err != nil {
			return nil, fmt.Errorf("failed to load catalog '%s': %v", ctg, err)
		}
	}

	srv := &Server{
		logger:  logger,
		state:   statedb,
		catalog: catalog,
	}

	vestaPath, err := filepath.Abs("./data-vesta")
	if err != nil {
		return nil, err
	}
	srv.backend = docker.NewDocker(vestaPath, srv)

	return srv, nil
}

func (s *Server) UpdateEvent(event *proto.Event) {
	if err := s.state.InsertEvent(event); err != nil {
		s.logger.Error("failed to insert event", "err", err)
	}
}

func validateField(raw interface{}, sch schema.Type) (interface{}, bool, error) {
	switch t := sch; t {
	case schema.TypeString:
		var result string
		if err := mapstructure.WeakDecode(raw, &result); err != nil {
			return nil, false, err
		}
		return result, true, nil

	case schema.TypeBool:
		var result bool
		if err := mapstructure.WeakDecode(raw, &result); err != nil {
			return nil, false, err
		}
		return result, true, nil

	case schema.TypeInt:
		var result int
		if err := mapstructure.WeakDecode(raw, &result); err != nil {
			return nil, false, err
		}
		return result, true, nil

	default:
		panic(fmt.Sprintf("Unknown type: %s", sch))
	}
}

func (s *Server) Create(req *proto.ApplyRequest) (string, error) {
	catalog := jsonnet.Load()

	alias := req.AllocationId
	if alias == "" {
		// generate the alias from the chain name and the node type
		alias = fmt.Sprintf("%s-%s", strings.ToLower(req.Chain), strings.ToLower(req.Action))
	}

	// find the specific network
	spl := strings.Split(req.Chain, ".")
	network, chain := spl[0], spl[1]

	var entry *jsonnet.Network
	for _, testEntry := range catalog.Networks {
		if strings.ToLower(testEntry.Network) == network {
			entry = testEntry
		}
	}
	if entry == nil {
		return "", fmt.Errorf("network not found '%s'", network)
	}

	// validate if the network exists or not
	chainInfo, ok := entry.Chains[chain]
	if !ok {
		return "", fmt.Errorf("chain not found '%s'", chain)
	}

	fmt.Println("-- entry -- ")
	fmt.Println(entry.Nodes)

	// find the implementation of the node
	var node *jsonnet.Node
	for _, n := range entry.Nodes {
		if strings.EqualFold(n.Name, req.Action) {
			node = n
		}
	}
	if node == nil {
		return "", fmt.Errorf("node not found '%s'", req.Action)
	}

	// check if the node can implement the chain
	nodeChain, ok := node.Chains[chain]
	if !ok {
		return "", fmt.Errorf("node '%s' does not implement the chain '%s'", req.Action, chain)
	}

	fmt.Println(chainInfo)
	fmt.Println(entry)
	fmt.Println(node)
	fmt.Println("nodeChain", nodeChain)

	properties := map[string]*proto.Item_Field{}
	for _, vol := range node.Volumes {
		for k, v := range vol.Properties {
			properties[k] = v
		}
	}

	fmt.Println("XXXX")
	fmt.Println(properties)
	fmt.Println(req.Input)

	iinput := map[string]string{}
	for k, v := range req.Input {
		iinput[k] = v.(string)
	}
	input := validate(iinput, properties)
	input["chain"] = chain

	// generate the input for the jsonnet
	output := entry.Apply(strings.ToLower(req.Action), input)

	// create the volumesSpec and the task volumes
	taskVolumes := map[string]*proto.Task_Volume{}
	srvVolumes := []*proto.ServiceVolume{}

	for _, vol := range node.Volumes {
		id := uuid.Generate()

		srvVolumes = append(srvVolumes, &proto.ServiceVolume{
			ID:   id,
			Name: vol.Name,
		})

		taskVolumes[vol.Name] = &proto.Task_Volume{
			Name: vol.Name,
			Id:   id,
			Path: vol.Path,
		}
	}

	task := &proto.Task{
		Image:   node.Image,
		Tag:     nodeChain.MaxVersion,
		Args:    output.Args,
		Ports:   node.Ports,
		Volumes: taskVolumes,
		Data:    output.Files,
	}

	srv := &proto.Service{
		Name:      alias,
		Artifacts: output.Artifacts,
		Task:      task,
		Volumes:   srvVolumes,
	}

	for _, artifact := range srv.Artifacts {
		srv.InitContainers = append(srv.InitContainers, generateArtifactsTask(srv, artifact))
	}

	s.backend.RunTasks(srv)
	return "", nil

	alias = req.AllocationId
	var prevAlloc *proto.Service

	// get the reference to the plugin and download on the background if necessary
	backend, volumeSpec, err := s.catalog.GetPlugin(req.Action)
	if err != nil {
		return "", fmt.Errorf("failed to get plugin '%s': %v", req.Action, err)
	}

	// try to resolve the alias to check if we have some previous allocation
	if alias != "" {
		if prevAlloc, err = s.state.GetDeployment(alias); err == nil {
			return "", err
		}
	}

	if alias == "" {
		// generate the alias from the chain name and the node type
		alias = fmt.Sprintf("%s-%s", strings.ToLower(req.Chain), strings.ToLower(req.Action))
	}

	// validate that all the inputs are expected and the required items are present
	visited := map[string]struct{}{}
	for name, field := range backend.Fields {
		visited[name] = struct{}{}

		val, ok := req.Input[name]
		if !ok && field.Required {
			return "", fmt.Errorf("field '%s' is required", name)
		}
		if ok {
			// validate the type of the input
			if _, _, err := validateField(val, field.Type); err != nil {
				return "", fmt.Errorf("failed to validate field '%s': %v", name, err)
			}
		}
	}

	// check if there are extra fields
	for k := range req.Input {
		if _, ok := visited[k]; !ok {
			return "", fmt.Errorf("field '%s' is not part of the schema", k)
		}
	}

	// create the field data
	fieldData := schema.FieldData{
		Raw:    req.Input,
		Schema: backend.Fields,
	}

	// validate any reference input with the state.
	for name, field := range backend.Fields {
		if field.References == nil {
			continue
		}

		val, ok := fieldData.GetOk(name)
		if !ok {
			continue
		}
		refName := val.(string)

		// validate the reference
		var obj interface{}
		if field.References.Type == "node" {
			node, err := s.state.GetDeployment(refName)
			if err != nil {
				return "", fmt.Errorf("failed to get node deployment '%s': %v", refName, err)
			}
			obj = node.Task
		} else if field.References.Type == "volume" {
			panic("Not anymore like this")
			if obj, err = s.state.GetVolume(refName); err != nil {
				return "", fmt.Errorf("failed to get volume '%s': %v", refName, err)
			}
		} else {
			panic("unknown reference type")
		}

		if !s.catalog.ValidateFn(req.Action, field.References.FilterCriteriaFunc, refName, obj) {
			return "", fmt.Errorf("failed to validate reference '%s'", refName)
		}
	}

	service := s.catalog.Build2(req.Action, &fieldData)

	// generate the artifact tasks
	for _, artifact := range service.Artifacts {
		service.InitContainers = append(service.InitContainers, generateArtifactsTask(service, artifact))
	}
	for _, init := range service.Init {
		service.InitContainers = append(service.InitContainers, &proto.Task{
			Image: service.Task.Image,
			Tag:   service.Task.Tag,
			Args:  init.Cmd,
		})
	}

	fmt.Println("-- InitContainers --")
	fmt.Println(service.InitContainers)

	// Apply volumes
	attachedVolumesByName := map[string]*proto.ServiceVolume{}
	if prevAlloc != nil {
		for _, vol := range prevAlloc.Volumes {
			attachedVolumesByName[vol.Name] = vol
		}
	}

	// 1. Check if there is any new volume reference that does not have any volume attached
	newVolumesMap := map[string]string{}

	for name := range volumeSpec {
		if _, ok := attachedVolumesByName[name]; !ok {
			id := uuid.Generate()
			newVolumesMap[name] = id

			attachedVolumesByName[name] = &proto.ServiceVolume{
				ID:   id,
				Name: name,
			}
		}
	}

	// apply any new labels
	updateVolumes := []*proto.Volume{}

	fmt.Println(service)
	fmt.Println(service.Task)

	for name, taskVol := range service.Task.Volumes {
		if _, ok := volumeSpec[name]; !ok {
			return "", fmt.Errorf("volume '%s' is not described in the volumes section", name)
		}

		var volume *proto.Volume
		if _, ok := newVolumesMap[name]; ok {
			volume = &proto.Volume{
				Id:     newVolumesMap[name],
				Name:   name,
				Labels: map[string]string{},
			}
		} else {
			if volume, err = s.state.GetVolume(attachedVolumesByName[name].ID); err != nil {
				return "", fmt.Errorf("failed to get volume '%s': %v", name, err)
			}
			volume = volume.Copy()
		}

		for k, v := range taskVol.Labels {
			volume.Labels[k] = v
		}
		updateVolumes = append(updateVolumes, volume)
	}

	newService := &proto.Service{
		Name:           alias,
		Task:           service.Task,
		InitContainers: service.InitContainers,
	}
	for _, vol := range attachedVolumesByName {
		newService.Volumes = append(newService.Volumes, vol)
	}

	if err := s.state.PutDeployment(newService, updateVolumes); err != nil {
		return "", fmt.Errorf("failed to put deployment: %v", err)
	}
	s.backend.RunTasks(newService)

	/*
		return "", nil

		stateDiff, service, err := s.catalog.Build(prestate, req)
		if err != nil {
			return "", fmt.Errorf("failed to run plugin '%s': %v", req.Action, err)
		}

		// Validate if the external reference is valid or not
		for fieldName, field := range stateDiff.Schema {
			if field.References != nil {
				val, ok := stateDiff.Raw[fieldName]
				if !ok {
					continue
				}
				refName := val.(string)

				// validate the reference
				var obj interface{}
				if field.References.Type == "node" {
					node, err := s.state.GetDeployment(refName)
					if err != nil {
						return "", fmt.Errorf("failed to get node deployment '%s': %v", refName, err)
					}
					obj = node.Tasks["node"]
				} else if field.References.Type == "volume" {
					if obj, err = s.state.GetVolume(refName); err != nil {
						return "", fmt.Errorf("failed to get volume '%s': %v", refName, err)
					}
				}

				if !s.catalog.ValidateFn(req.Action, field.References.FilterCriteriaFunc, stateDiff.Raw, obj) {
					return "", fmt.Errorf("failed to validate reference '%s'", refName)
				}
			}
		}

		// 2. If there are new volumes, create them. This means creating a new id for the volume
		// The volumes are going to be in the service BUT they will have no id created.
		for _, volume := range service.Volumes {
			if volume.Id == "" {
				// this will force to create the new volume, now the volume is assigned to the service
				volume.Id = uuid.Generate()
			}
		}

		// Add basic label to each task
		for name, task := range service.Tasks {
			if task.Labels == nil {
				task.Labels = map[string]string{}
			}
			task.Labels["task"] = name
			task.Labels["service"] = alias
			task.Labels["chain"] = req.Chain
		}

		newState, err := json.Marshal(stateDiff.Raw)
		if err != nil {
			return "", err
		}

		newService := &proto.Service{
			Name:      alias,
			Tasks:     service.Tasks,
			PrevState: newState,
			Volumes:   service.Volumes,
		}

		fmt.Println("-- write deployment --")
		if err := s.state.PutDeployment(newService); err != nil {
			return "", err
		}

		s.backend.RunTasks(newService)
	*/

	return alias, nil
}

func (s *Server) Apply(ctx context.Context, req *proto.ApplyRequest) (string, error) {
	id, err := s.Create(req)
	if err != nil {
		return "", err
	}

	return id, nil
}

func (s *Server) VolumeList(ctx context.Context) ([]*proto.Volume, error) {
	return s.state.GetVolumes()
}

func (s *Server) DeploymentList(ctx context.Context) ([]*proto.Service, error) {
	return s.state.GetDeployments()
}

func (s *Server) Destroy(ctx context.Context, id string) error {
	s.backend.Destroy(id)
	return nil
}

func (s *Server) CatalogList(ctx context.Context) ([]string, error) {
	return s.catalog.ListPlugins(), nil
}

func (s *Server) CatalogInspect(ctx context.Context, name string) (*proto.Item, error) {
	panic("TODO")
}

/*
func (s *service) SubscribeEvents(req *proto.SubscribeEventsRequest, stream proto.VestaService_SubscribeEventsServer) error {
	for {
		ws := memdb.NewWatchSet()
		it := s.srv.state.SubscribeEvents(req.Service, ws)

		for obj := it.Next(); obj != nil; obj = it.Next() {
			event := obj.(*proto.Event)

			if err := stream.Send(event); err != nil {
				panic(err)
			}

			select {
			case <-stream.Context().Done():
				return nil
			default:
			}
		}

		// wait for the duties to change
		select {
		case <-ws.WatchCh(context.Background()):
		case <-stream.Context().Done():
			return nil
		}
	}
}
*/

func generateArtifactsTask(service *proto.Service, artifact *proto.Artifact) *proto.Task {
	artifactTask := &proto.Task{
		Image: "alpine/curl",
		Tag:   "latest",
		Args: []string{
			"curl",
			"-o",
			artifact.Dst,
			artifact.Src,
		},
		Volumes: service.Task.Volumes,
	}
	return artifactTask
}

func validate(input map[string]string, items map[string]*proto.Item_Field) map[string]interface{} {
	result := map[string]interface{}{}

	// first, validate the types of the input
	for name, val := range input {
		item, ok := items[name]

		if !ok {
			panic(fmt.Sprintf("field '%s' is not part of the schema", name))
		}

		val, err := getPrimitive(val, item.Type)
		if err != nil {
			panic(fmt.Sprintf("failed to validate field '%s': %v", name, err))
		}

		result[name] = val
	}

	// check the values that are not set
	// if the value is not set, we need to set the default value
	// if the value is not set and required, fail.
	for name, item := range items {
		if _, ok := input[name]; ok {
			continue
		}

		if item.Required {
			panic(fmt.Sprintf("field '%s' is required", name))
		}
		if item.Default != "" {
			val, err := getPrimitive(item.Default, item.Type)
			if err != nil {
				panic(fmt.Sprintf("failed to validate field '%s': %v", name, err))
			}
			result[name] = val
		}
	}

	return result
}

func getPrimitive(raw string, typ string) (interface{}, error) {
	switch typ {
	case "string":
		var result string
		if err := mapstructure.WeakDecode(raw, &result); err != nil {
			return nil, err
		}
		return result, nil

	case "bool":
		var result bool
		if err := mapstructure.WeakDecode(raw, &result); err != nil {
			return nil, err
		}
		return result, nil

	case "int":
		var result int
		if err := mapstructure.WeakDecode(raw, &result); err != nil {
			return nil, err
		}
		return result, nil

	default:
		panic(fmt.Sprintf("Unknown type: %s", typ))
	}
}
