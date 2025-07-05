package service

import (
	"fmt"

	"github.com/michaelprice232/mongodb-backup-launcher/config"
)

// Service is the main service struct for the backup launcher.
type Service struct {
	conf config.Config
}

// NewService creates a new Service instance.
func NewService(conf config.Config) (*Service, error) {
	return &Service{conf: conf}, nil
}

// Run executes the backup workflow.
func (s *Service) Run() error {
	targetHost, err := s.mongoDBReadReplicaToTarget()
	if err != nil {
		return fmt.Errorf("finding which secondary MongoDB replica to target: %w", err)
	}

	targetAZ, targetNamespace, err := s.availabilityZoneToTarget(targetHost)
	if err != nil {
		return fmt.Errorf("finding which availabilty zone to target: %w", err)
	}

	_, err = s.createJob(targetHost, targetAZ, targetNamespace)
	if err != nil {
		return fmt.Errorf("creating job: %w", err)
	}

	return nil
}
