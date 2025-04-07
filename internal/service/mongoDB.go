package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type member struct {
	Name string `bson:"name"`
	Role string `bson:"stateStr"`
}

type replicaSetMembers struct {
	OK      int      `bson:"ok"`
	Members []member `bson:"members"`
}

func (s *Service) mongoDBReadReplicaToTarget() (string, error) {
	db := s.conf.MongoDBClient.Database("admin")

	rsMembers := replicaSetMembers{
		Members: make([]member, 3),
	}

	// https://www.mongodb.com/docs/drivers/go/current/fundamentals/run-command/
	err := db.RunCommand(context.Background(), bson.D{bson.E{Key: "replSetGetStatus", Value: 1}}).Decode(&rsMembers)
	if err != nil {
		return "", fmt.Errorf("getting replica set status: %v", err)
	}

	if rsMembers.OK != 1 {
		return "", fmt.Errorf("database operation did not complete succesfully")
	}

	if s.conf.LogLevel == "debug" {
		members, err := json.MarshalIndent(rsMembers, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshalling replica set payload: %w", err)
		}
		fmt.Printf("Replica set members:\n%s\n", string(members))
	}

	var target string
	for _, m := range rsMembers.Members {
		if m.Role == "SECONDARY" {
			if s.conf.ExcludeReplica != "" && s.conf.ExcludeReplica == m.Name {
				continue
			}
			target = m.Name
			break
		}
	}

	if target == "" {
		return "", fmt.Errorf("not found a SECONDARY replica set member which is not in the EXCLUDE_REPLICA env var. EXCLUDE_REPLICA = %s", s.conf.ExcludeReplica)
	}

	slog.Debug("Target Host", "host", target)

	return target, nil
}
