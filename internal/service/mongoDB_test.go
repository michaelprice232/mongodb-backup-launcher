package service

import (
	"context"
	"github.com/pkg/errors"
	"testing"

	"github.com/michaelprice232/mongodb-backup-launcher/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockSingleResult struct {
	mock.Mock
}

func (m *mockSingleResult) Decode(v any) error {
	args := m.Called(v)
	return args.Error(0)
}

type mockMongoClient struct {
	mock.Mock
}

func (m *mockMongoClient) RunCommand(ctx context.Context, runCommand interface{}) config.SingleResult {
	args := m.Called(ctx, runCommand)
	return args.Get(0).(config.SingleResult)
}

func Test_mongoDBReadReplicaToTarget(t *testing.T) {
	tests := []struct {
		name           string
		ok             int // 1 == succeeded, 0 == failed
		members        []member
		excludeReplica string
		expectedTarget string
		expectedError  bool
		logLevel       string
		nilMembers     bool // for simulating nil slice
	}{
		{
			name: "GoodWithExcludeReplica", ok: 1, members: []member{
			{Name: "mongodb-0.mongodb.database.svc.cluster.local", Role: "PRIMARY"},
			{Name: "mongodb-1.mongodb.database.svc.cluster.local", Role: "SECONDARY"},
			{Name: "mongodb-2.mongodb.database.svc.cluster.local", Role: "SECONDARY"}},
			excludeReplica: "mongodb-1.mongodb.database.svc.cluster.local",
			expectedTarget: "mongodb-2.mongodb.database.svc.cluster.local",
			expectedError:  false,
		},
		{
			name: "GoodWithNoExcludeReplica", ok: 1, members: []member{
			{Name: "mongodb-0.mongodb.database.svc.cluster.local", Role: "SECONDARY"},
			{Name: "mongodb-1.mongodb.database.svc.cluster.local", Role: "PRIMARY"},
			{Name: "mongodb-2.mongodb.database.svc.cluster.local", Role: "SECONDARY"}},
			expectedTarget: "mongodb-0.mongodb.database.svc.cluster.local",
			expectedError:  false,
		},
		{
			name: "NotOK", ok: 0, members: []member{},
			expectedTarget: "",
			expectedError:  true,
		},
		{
			name: "NoReplicasButOKStatus", ok: 1, members: []member{},
			expectedTarget: "",
			expectedError:  true,
		},
		{
			name: "DebugLogging", ok: 1, members: []member{
			{Name: "mongodb-0.mongodb.database.svc.cluster.local", Role: "SECONDARY"},
			{Name: "mongodb-1.mongodb.database.svc.cluster.local", Role: "SECONDARY"},
			{Name: "mongodb-2.mongodb.database.svc.cluster.local", Role: "PRIMARY"}},
			expectedTarget: "mongodb-0.mongodb.database.svc.cluster.local",
			expectedError:  false,
			logLevel:       "debug",
		},
		{
			name: "FailedDecode", ok: 1, members: []member{
			{Name: "mongodb-0.mongodb.database.svc.cluster.local", Role: "PRIMARY"},
			{Name: "mongodb-1.mongodb.database.svc.cluster.local", Role: "SECONDARY"},
			{Name: "mongodb-2.mongodb.database.svc.cluster.local", Role: "SECONDARY"}},
			expectedTarget: "",
			expectedError:  true,
		},
		{
			name: "MultipleExcludedReplicas", ok: 1, members: []member{
			{Name: "mongodb-1", Role: "SECONDARY"},
			{Name: "mongodb-2", Role: "SECONDARY"},
			{Name: "mongodb-3", Role: "SECONDARY"},
		},
			excludeReplica: "mongodb-1",
			expectedTarget: "mongodb-2",
			expectedError:  false,
		},
		{
			name: "AllSecondariesExcluded", ok: 1, members: []member{
			{Name: "mongodb-1", Role: "SECONDARY"},
			{Name: "mongodb-2", Role: "SECONDARY"},
		},
			excludeReplica: "mongodb-1", // will exclude first, but not second
			expectedTarget: "mongodb-2",
			expectedError:  false,
		},
		{
			name: "MultipleExcludedReplicasPassed", ok: 1, members: []member{
			{Name: "mongodb-1", Role: "SECONDARY"},
			{Name: "mongodb-2", Role: "SECONDARY"},
		},
			excludeReplica: "mongodb-1,mongodb-2", // multiple excluded replicas are not supported by code, so there will not be a match
			expectedTarget: "mongodb-1",
			expectedError:  false,
		},
		{
			name: "NoSecondaries", ok: 1, members: []member{
			{Name: "mongodb-1", Role: "PRIMARY"},
			{Name: "mongodb-2", Role: "ARBITER"},
		},
			expectedTarget: "",
			expectedError:  true,
		},
		{
			name: "MalformedMemberEmptyName", ok: 1, members: []member{
			{Name: "", Role: "SECONDARY"},
			{Name: "mongodb-2", Role: "SECONDARY"},
		},
			expectedTarget: "mongodb-2",
			expectedError:  false,
		},
		{
			name: "MalformedMemberEmptyRole", ok: 1, members: []member{
			{Name: "mongodb-1", Role: ""},
			{Name: "mongodb-2", Role: "SECONDARY"},
		},
			expectedTarget: "mongodb-2",
			expectedError:  false,
		},
		{
			name: "NilMembersSlice", ok: 1, nilMembers: true,
			expectedTarget: "",
			expectedError:  true,
		},
	}

	for _, tc := range tests {
		mockClient := new(mockMongoClient)
		mockResult := new(mockSingleResult)

		var decodeError error
		if tc.name == "FailedDecode" {
			decodeError = errors.New("error whilst decoding the server response")
		}

		t.Run(tc.name, func(t *testing.T) {
			// setup Decode to write into the provided struct
			mockResult.On("Decode", mock.Anything).Run(func(args mock.Arguments) {
				ptr := args.Get(0).(*replicaSetMembers)
				ptr.OK = tc.ok
				if tc.nilMembers {
					ptr.Members = nil
				} else {
					ptr.Members = tc.members
				}
			}).Return(decodeError)

			mockClient.On("RunCommand", mock.Anything, mock.Anything).Return(mockResult)

			s := Service{
				conf: config.Config{
					MongoDBClient:  mockClient,
					ExcludeReplica: tc.excludeReplica,
					LogLevel:       tc.logLevel,
				},
			}

			target, err := s.mongoDBReadReplicaToTarget()

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedTarget, target)
		})
	}
}
