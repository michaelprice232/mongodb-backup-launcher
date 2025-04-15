package service

import (
	"testing"

	"github.com/michaelprice232/mongodb-backup-launcher/config"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_createJob(t *testing.T) {
	k8sClient := fake.NewClientset()

	conf := config.Config{
		K8sClient: k8sClient,
	}

	s, err := NewService(conf)
	assert.Nil(t, err)

	mongoDBHost := "mongodb-0.mongodb.database.svc.cluster.local:27017"
	az := "eu-west-1a"
	namespace := "database"

	job, err := s.createJob(mongoDBHost, az, namespace)
	assert.Nil(t, err)
	assert.NotNil(t, job)
	assert.Equal(t, job.Namespace, namespace)
	assert.Contains(t, job.Spec.Template.Annotations, "karpenter.sh/do-not-disrupt")

	expressions := job.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions
	var foundAZAffinity bool
	var foundNodePoolAffinity bool
	for _, e := range expressions {
		if e.Key == "topology.kubernetes.io/zone" {
			for _, zone := range e.Values {
				if zone == az {
					foundAZAffinity = true
				}
			}
		}

		if e.Key == "karpenter.sh/nodepool" {
			for _, pool := range e.Values {
				if pool == "backups" {
					foundNodePoolAffinity = true
				}
			}
		}
	}
	assert.True(t, foundAZAffinity, "expected the job to have a node affinity to the input availability zone")
	assert.True(t, foundNodePoolAffinity, "expected the job to have a node affinity to the backups Nodepool")

	assert.Equal(t, job.Spec.Template.Spec.Tolerations[0].Key, "mongodb-backups", "expected the job to have a toleration set")
}

func Test_availabilityZoneToTarget(t *testing.T) {
	k8sClient := fake.NewClientset(
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mongodb-0",
				Namespace: "database",
			},
			Spec: v1.PodSpec{
				NodeName: "node1",
			},
		},

		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mongodb-1",
				Namespace: "database",
			},
			Spec: v1.PodSpec{
				NodeName: "no-az-label",
			},
		},

		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mongodb-2",
				Namespace: "database",
			},
			Spec: v1.PodSpec{
				NodeName: "missing-node",
			},
		},

		&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
				Labels: map[string]string{
					"topology.kubernetes.io/zone": "eu-west-1a",
				},
			},
		},

		&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "no-az-label",
			},
		},
	)

	conf := config.Config{
		K8sClient: k8sClient,
	}

	s, err := NewService(conf)
	assert.Nil(t, err)

	targetAZ, targetNamespace, err := s.availabilityZoneToTarget("mongodb-0.mongodb.database.svc.cluster.local")
	assert.Nil(t, err)
	assert.Equal(t, "eu-west-1a", targetAZ)
	assert.Equal(t, "database", targetNamespace)

	_, _, err = s.availabilityZoneToTarget("mongodb-1.mongodb.database.svc.cluster.local")
	assert.NotNilf(t, err, "expected an error as the AZ label is missing from the node")

	_, _, err = s.availabilityZoneToTarget("mongodb-2.mongodb.database.svc.cluster.local")
	assert.NotNilf(t, err, "expected an error as the node the pod was listed as being scheduled on is missing")

	_, _, err = s.availabilityZoneToTarget("mongodb-2.mongodb.bad-namespace.svc.cluster.local")
	assert.NotNilf(t, err, "expected an error as namespace does not exist")

	_, _, err = s.availabilityZoneToTarget("bad-pod.mongodb.database.svc.cluster.local")
	assert.NotNilf(t, err, "expected an error as the pod does not exist")

	_, _, err = s.availabilityZoneToTarget("mongodb-1.mongodb")
	assert.NotNilf(t, err, "expected an error as the parameter FQDN does not have enough parts")
}
