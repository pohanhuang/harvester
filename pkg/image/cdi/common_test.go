package cdi

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	"github.com/harvester/harvester/pkg/util"
)

func TestGenerateDVTargetStorageUsesExactImageSize(t *testing.T) {
	vmi := &harvesterv1.VirtualMachineImage{
		Spec: harvesterv1.VirtualMachineImageSpec{
			TargetStorageClassName: "test-storage-class",
		},
		Status: harvesterv1.VirtualMachineImageStatus{
			VirtualSize: 2361393152,
			Size:        735465984,
		},
	}

	storage, err := generateDVTargetStorage(vmi)
	require.NoError(t, err)
	require.Equal(t, "test-storage-class", *storage.StorageClassName)
	request := storage.Resources.Requests[corev1.ResourceStorage]
	require.Equal(t, int64(2361393152), request.Value())
}

func TestGenerateDVTargetStorageForUpgradeRepo(t *testing.T) {
	vmi := &harvesterv1.VirtualMachineImage{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				util.AnnotationUpgradeImage: "True",
			},
		},
		Spec: harvesterv1.VirtualMachineImageSpec{
			TargetStorageClassName: "test-storage-class",
		},
		Status: harvesterv1.VirtualMachineImageStatus{
			VirtualSize: 2361393152,
		},
	}

	storage, err := generateDVTargetStorage(vmi)
	require.NoError(t, err)
	require.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, storage.AccessModes)
	require.NotNil(t, storage.VolumeMode)
	require.Equal(t, corev1.PersistentVolumeFilesystem, *storage.VolumeMode)
}
