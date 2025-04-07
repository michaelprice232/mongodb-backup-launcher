package service

import (
	"fmt"

	"github.com/michaelprice232/mongodb-backup-launcher/config"
)

type Service struct {
	conf config.Config
}

func NewService(conf config.Config) (*Service, error) {
	return &Service{conf: conf}, nil
}

func (s *Service) Run() error {
	targetHost, err := s.mongoDBReadReplicaToTarget()
	if err != nil {
		return fmt.Errorf("finding which secondary MongoDB replica to target: %w", err)
	}

	targetAZ, targetNamespace, err := s.availabilityZoneToTarget(targetHost)
	if err != nil {
		return fmt.Errorf("finding which availabilty zone to target: %w", err)
	}

	err = s.createJob(targetHost, targetAZ, targetNamespace)
	if err != nil {
		return fmt.Errorf("creating job: %w", err)
	}

	return nil
}
