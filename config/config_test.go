package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"k8s.io/client-go/kubernetes"
)

var (
	origMongoDBClient = mongoDBClient
	origK8sClient     = k8sClient
)

func stubMongoDBClient() {
	mongoDBClient = func() (*mongo.Database, error) {
		return nil, nil
	}
}

func stubK8sClient() {
	k8sClient = func() (*kubernetes.Clientset, error) {
		return nil, nil
	}
}

func restoreClients() {
	mongoDBClient = origMongoDBClient
	k8sClient = origK8sClient
}

func TestNewConfig_EnvVars(t *testing.T) {
	stubMongoDBClient()
	stubK8sClient()
	t.Cleanup(restoreClients)

	t.Run("all env vars set, valid", func(t *testing.T) {
		_ = os.Setenv("DOCKER_IMAGE_URI", "test-image")
		_ = os.Setenv("BACKUP_TYPE", "hourly")
		_ = os.Setenv("BACKUP_JOB_TIMEOUT_SEC", "1234")
		_ = os.Setenv("HOSTNAME", "test-host")
		_ = os.Setenv("EXCLUDE_REPLICA", "replica-1")
		defer os.Clearenv()

		conf, err := NewConfig()
		assert.NoError(t, err)
		assert.Equal(t, "test-image", conf.DockerImageURI)
		assert.Equal(t, "hourly", conf.BackupType)
		assert.Equal(t, int64(1234), conf.BackupTimeout)
		assert.Equal(t, "test-host", conf.Hostname)
		assert.Equal(t, "replica-1", conf.ExcludeReplica)
	})

	t.Run("hostname unset", func(t *testing.T) {
		_ = os.Setenv("DOCKER_IMAGE_URI", "test-image")
		_ = os.Setenv("BACKUP_TYPE", "hourly")
		_ = os.Setenv("BACKUP_JOB_TIMEOUT_SEC", "1234")
		_ = os.Unsetenv("HOSTNAME")
		defer os.Clearenv()

		conf, err := NewConfig()
		assert.NoError(t, err)
		assert.Equal(t, "unknown", conf.Hostname)
	})

	t.Run("timeout unset uses default", func(t *testing.T) {
		_ = os.Setenv("DOCKER_IMAGE_URI", "test-image")
		_ = os.Setenv("BACKUP_TYPE", "hourly")
		_ = os.Unsetenv("BACKUP_JOB_TIMEOUT_SEC")
		defer os.Clearenv()

		conf, err := NewConfig()
		assert.NoError(t, err)
		assert.Equal(t, int64(2700), conf.BackupTimeout)
	})

	t.Run("timeout invalid returns error", func(t *testing.T) {
		_ = os.Setenv("DOCKER_IMAGE_URI", "test-image")
		_ = os.Setenv("BACKUP_TYPE", "hourly")
		_ = os.Setenv("BACKUP_JOB_TIMEOUT_SEC", "notanint")
		defer os.Clearenv()

		_, err := NewConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parsing BACKUP_JOB_TIMEOUT_SEC")
	})

	t.Run("missing docker image returns error", func(t *testing.T) {
		_ = os.Unsetenv("DOCKER_IMAGE_URI")
		_ = os.Setenv("BACKUP_TYPE", "hourly")
		defer os.Clearenv()

		_, err := NewConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "docker image URI")
	})

	t.Run("invalid backup type returns error", func(t *testing.T) {
		_ = os.Setenv("DOCKER_IMAGE_URI", "test-image")
		_ = os.Setenv("BACKUP_TYPE", "weekly")
		defer os.Clearenv()

		_, err := NewConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "BACKUP_TYPE must be 'hourly' or 'daily'")
	})
}
