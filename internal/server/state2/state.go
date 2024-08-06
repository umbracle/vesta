package state2

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	"github.com/umbracle/vesta/internal/server/proto"
)

type State struct {
	db *sql.DB
}

func NewState(path string) (*State, error) {
	s := &State{}

	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	s.db = db

	if err := s.applyMigrations(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *State) Close() error {
	return s.db.Close()
}

func (s *State) ListDeployments() ([]*proto.Deployment2, error) {

	// get the deployments
	rows, err := s.db.Query("SELECT id, name, spec FROM deployments")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []*proto.Deployment2
	for rows.Next() {
		var id, name, spec string
		if err := rows.Scan(&id, &name, &spec); err != nil {
			return nil, err
		}
		deployments = append(deployments, &proto.Deployment2{
			Id:   id,
			Name: name,
			Spec: []byte(spec),
		})
	}

	return deployments, nil
}

func (s *State) CreateDeployment(dep *proto.Deployment2) error {

	// create the deployment
	_, err := s.db.Exec("INSERT INTO deployments (id, name, spec) VALUES (?, ?, ?)", dep.Id, dep.Name, dep.Spec)
	if err != nil {
		return err
	}

	return nil
}

func (s *State) UpdateDeployment(dep *proto.Deployment2) error {

	// update the deployment
	_, err := s.db.Exec("UPDATE deployments SET name=?, spec=? WHERE id=?", dep.Name, dep.Spec, dep.Id)
	if err != nil {
		return err
	}

	return nil
}

func (s *State) GetDeploymentById(id string) (*proto.Deployment2, error) {

	// get the deployment
	row := s.db.QueryRow("SELECT id, name, spec FROM deployments WHERE id=?", id)

	var name, spec string
	if err := row.Scan(&id, &name, &spec); err != nil {
		return nil, err
	}

	return &proto.Deployment2{
		Id:   id,
		Name: name,
		Spec: []byte(spec),
	}, nil
}

func (s *State) CreateEvent(event *proto.Event2) error {

	// create the event
	_, err := s.db.Exec("INSERT INTO events (id, deployment_id, task, type) VALUES (?, ?, ?, ?)", event.Id, event.Deployment, event.Task, event.Type)
	if err != nil {
		return err
	}

	return nil
}

func (s *State) GetEventsByDeployment(id string) ([]*proto.Event2, error) {
	// get the events
	rows, err := s.db.Query("SELECT id, deployment_id, task, type FROM events WHERE deployment_id=?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*proto.Event2
	for rows.Next() {
		var id, deployment, task, typ string
		if err := rows.Scan(&id, &deployment, &task, &typ); err != nil {
			return nil, err
		}
		events = append(events, &proto.Event2{
			Id:         id,
			Deployment: deployment,
			Task:       task,
			Type:       typ,
		})
	}

	return events, nil
}
