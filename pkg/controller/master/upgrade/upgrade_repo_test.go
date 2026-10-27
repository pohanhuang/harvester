package upgrade

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	lhv1beta2 "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta2"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	"github.com/harvester/harvester/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/harvester/pkg/util"
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

func TestRepoScaleDeployment(t *testing.T) {
	tests := []struct {
		name     string
		existing []runtime.Object
		replicas int32
		expected *int32
	}{
		{
			name:     "scale down the repo deployment",
			existing: []runtime.Object{newRepoDeployment("upgrade-1", 1)},
			replicas: 0,
			expected: ptr.To(int32(0)),
		},
		{
			name:     "scale up the repo deployment",
			existing: []runtime.Object{newRepoDeployment("upgrade-1", 0)},
			replicas: 1,
			expected: ptr.To(int32(1)),
		},
		{
			name:     "no-op if the repo deployment doesn't exist",
			replicas: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(tc.existing...)
			handler := &upgradeHandler{
				deploymentClient: fakeclients.DeploymentClient(clientset.AppsV1().Deployments),
			}
			repo := NewUpgradeRepo(context.Background(), &harvesterv1.Upgrade{
				ObjectMeta: metav1.ObjectMeta{Name: "upgrade-1"},
			}, handler)

			require.NoError(t, repo.ScaleDeployment(tc.replicas))

			deployment, err := clientset.AppsV1().Deployments(upgradeNamespace).Get(context.Background(), repo.GetDeploymentName(), metav1.GetOptions{})
			if tc.expected == nil {
				require.True(t, apierrors.IsNotFound(err))
				return
			}
			require.NoError(t, err)
			require.Equal(t, *tc.expected, *deployment.Spec.Replicas)
		})
	}
}

func TestGetImageCleanupList(t *testing.T) {
	upgrade := &harvesterv1.Upgrade{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "upgrade-1",
			Annotations: map[string]string{},
		},
	}
	upgrade.Annotations[imageCleanupListAnnotation] = `["docker.io/rancher/a:v1","docker.io/rancher/b:v1"]`

	imageList, err := getImageCleanupList(upgrade, NewUpgradeRepo(context.Background(), upgrade, &upgradeHandler{}))
	require.NoError(t, err)
	require.Equal(t, []string{"docker.io/rancher/a:v1", "docker.io/rancher/b:v1"}, imageList)

	// cacheImageCleanupList keeps an existing cache without contacting the repo
	require.NoError(t, cacheImageCleanupList(upgrade, nil))
	require.Equal(t, `["docker.io/rancher/a:v1","docker.io/rancher/b:v1"]`, upgrade.Annotations[imageCleanupListAnnotation])
}

func newRepoDeployment(upgradeName string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      repoServiceNamePrefix + upgradeName,
			Namespace: upgradeNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(replicas),
		},
	}
}

func TestRepoSetVolumeReplicas(t *testing.T) {
	tests := []struct {
		name     string
		volume   *lhv1beta2.Volume
		expected int
	}{
		{
			name:     "reduce replicas of longhorn volume",
			volume:   &lhv1beta2.Volume{ObjectMeta: metav1.ObjectMeta{Name: "pvc-1", Namespace: util.LonghornSystemNamespaceName}, Spec: lhv1beta2.VolumeSpec{NumberOfReplicas: 3}},
			expected: 1,
		},
		{
			name: "no-op if the volume isn't a longhorn volume",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			image := &harvesterv1.VirtualMachineImage{ObjectMeta: metav1.ObjectMeta{Name: "upgrade-image", Namespace: upgradeNamespace}}
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: image.Name, Namespace: image.Namespace},
				Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pvc-1"},
			}
			objs := []runtime.Object{image, pvc}
			if tc.volume != nil {
				objs = append(objs, tc.volume)
			}
			clientset := fake.NewSimpleClientset(objs...)
			handler := &upgradeHandler{
				vmImageCache:   fakeclients.VirtualMachineImageCache(clientset.HarvesterhciV1beta1().VirtualMachineImages),
				pvcClient:      fakeclients.PersistentVolumeClaimClient(clientset.CoreV1().PersistentVolumeClaims),
				lhVolumeClient: fakeclients.LonghornVolumeClient(clientset.LonghornV1beta2().Volumes),
			}
			repo := NewUpgradeRepo(context.Background(), &harvesterv1.Upgrade{
				ObjectMeta: metav1.ObjectMeta{Name: "upgrade-1"},
				Status:     harvesterv1.UpgradeStatus{ImageID: image.Namespace + "/" + image.Name},
			}, handler)

			require.NoError(t, repo.SetVolumeReplicas(1))

			if tc.volume == nil {
				return
			}
			volume, err := clientset.LonghornV1beta2().Volumes(util.LonghornSystemNamespaceName).Get(context.Background(), tc.volume.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, tc.expected, volume.Spec.NumberOfReplicas)
		})
	}
}
