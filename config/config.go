package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type SingleResult interface {
	Decode(v any) error
}

type MongoDBClient interface {
	RunCommand(ctx context.Context, runCommand interface{}) SingleResult
}

type Config struct {
	MongoDBClient  MongoDBClient
	K8sClient      kubernetes.Interface
	ExcludeReplica string
	LogLevel       string
	DockerImageURI string
}

// realMongoClient wraps the MongoDB Database struct to work around the fact that mongo.SingleResult has no exported fields we can mock.
type realMongoClient struct {
	db *mongo.Database
}

func (r *realMongoClient) RunCommand(ctx context.Context, runCommand interface{}) SingleResult {
	return r.db.RunCommand(ctx, runCommand)
}

func NewConfig() (Config, error) {
	conf := Config{}

	// Logger
	logLevelStr := strings.ToLower(os.Getenv("LOG_LEVEL"))
	conf.LogLevel = logLevelStr
	var level slog.Level

	switch logLevelStr {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	slog.SetDefault(slog.New(handler))

	// A MongoDB replica which you do NOT want to use as a target. It might have another special role, and you don't want to add performance overhead
	conf.ExcludeReplica = os.Getenv("EXCLUDE_REPLICA")

	// Docker image to use when creating new K8s backup jobs
	dockerImageURI := os.Getenv("DOCKER_IMAGE_URI")
	if dockerImageURI == "" {
		return conf, fmt.Errorf("docker image URI - DOCKER_IMAGE_URI - has not been set")
	}
	conf.DockerImageURI = dockerImageURI

	// MongoDB Client
	mongoDBc, err := mongoDBClient()
	if err != nil {
		return conf, fmt.Errorf("creating MongoDB client: %w", err)
	}
	conf.MongoDBClient = &realMongoClient{
		db: mongoDBc,
	}

	// K8s Client
	k8sc, err := k8sClient()
	if err != nil {
		return conf, fmt.Errorf("creating K8s client: %w", err)
	}
	conf.K8sClient = k8sc

	return conf, nil
}

func k8sClient() (*kubernetes.Clientset, error) {
	var client *kubernetes.Clientset
	var config *rest.Config
	var err error

	runningLocally := os.Getenv("RUNNING_LOCALLY")
	if runningLocally == "true" {
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), ".kube", "config"))
		if err != nil {
			return nil, fmt.Errorf("building K8s client config from the local host: %w", err)
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("building K8s client config from the cluster: %w", err)
		}
	}

	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating K8s client: %w", err)
	}

	return client, nil
}

func mongoDBClient() (*mongo.Database, error) {
	mongoUsername := os.Getenv("MONGODB_USERNAME")
	if mongoUsername == "" {
		return nil, fmt.Errorf("mongoDB username - MONGODB_USERNAME - has not been set")
	}

	mongoPassword := os.Getenv("MONGODB_PASSWORD")
	if mongoPassword == "" {
		return nil, fmt.Errorf("mongoDB password - MONGODB_PASSWORD - has not been set")
	}

	mongoURI := os.Getenv("MONGODB_URI")
	if !strings.HasPrefix(mongoURI, "mongodb://") {
		return nil, fmt.Errorf("set your 'MONGODB_URI' environment variable. Must start with 'mongodb://'. See: https://www.mongodb.com/docs/drivers/go/current/fundamentals/connections/")
	}

	mongoDBCredential := options.Credential{
		AuthSource: "admin",
		Username:   mongoUsername,
		Password:   mongoPassword,
	}

	clientOpts := options.Client().ApplyURI(mongoURI).SetAuth(mongoDBCredential)

	client, err := mongo.Connect(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("creating MongoDB client: %w", err)
	}

	return client.Database("admin"), nil
}
