package util

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	AnnStorageProvisioner     = "volume.kubernetes.io/storage-provisioner"
	AnnBetaStorageProvisioner = "volume.beta.kubernetes.io/storage-provisioner"
)

// GetProvisionedPVCProvisioner do not use this function when the PVC is just created
func GetProvisionedPVCProvisioner(pvc *corev1.PersistentVolumeClaim) string {
	provisioner, ok := pvc.Annotations[AnnBetaStorageProvisioner]
	if !ok {
		provisioner = pvc.Annotations[AnnStorageProvisioner]
	}
	return provisioner
}
