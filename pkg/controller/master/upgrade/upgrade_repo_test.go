package upgrade

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	"github.com/harvester/harvester/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/harvester/pkg/util/fakeclients"
)

func TestRepoCreateDeploymentUsesSingleRecreateReplica(t *testing.T) {
	clientset := fake.NewSimpleClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}})
	handler := &upgradeHandler{
		nodeCache:        fakeclients.NodeCache(clientset.CoreV1().Nodes),
		deploymentClient: fakeclients.DeploymentClient(clientset.AppsV1().Deployments),
	}
	repo := NewUpgradeRepo(context.Background(), &harvesterv1.Upgrade{
		ObjectMeta: metav1.ObjectMeta{Name: "upgrade-1"},
	}, handler)

	deployment, err := repo.CreateDeployment(&harvesterv1.VirtualMachineImage{
		ObjectMeta: metav1.ObjectMeta{Name: "upgrade-image"},
	})
	require.NoError(t, err)
	require.NotNil(t, deployment.Spec.Replicas)
	require.EqualValues(t, 1, *deployment.Spec.Replicas)
	require.Equal(t, appsv1.RecreateDeploymentStrategyType, deployment.Spec.Strategy.Type)
}
