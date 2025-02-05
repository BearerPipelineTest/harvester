package volumesnapshot

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	"github.com/rancher/apiserver/pkg/apierror"
	ctlcorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctlsnapshotv1 "github.com/harvester/harvester/pkg/generated/controllers/snapshot.storage.k8s.io/v1beta1"
	"github.com/harvester/harvester/pkg/util"
)

type ActionHandler struct {
	pvcs          ctlcorev1.PersistentVolumeClaimClient
	pvcCache      ctlcorev1.PersistentVolumeClaimCache
	snapshotCache ctlsnapshotv1.VolumeSnapshotCache
}

func (h ActionHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if err := h.do(rw, req); err != nil {
		status := http.StatusInternalServerError
		if e, ok := err.(*apierror.APIError); ok {
			status = e.Code.Status
		}
		rw.WriteHeader(status)
		_, _ = rw.Write([]byte(err.Error()))
		return
	}
	rw.WriteHeader(http.StatusNoContent)
}

func (h *ActionHandler) do(rw http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	action := vars["action"]
	snapshotName := vars["name"]
	snapshotNamespace := vars["namespace"]

	switch action {
	case actionRestore:
		var input RestoreSnapshotInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			return apierror.NewAPIError(validation.InvalidBodyContent, "Failed to decode request body: %v "+err.Error())
		}
		if input.Name == "" {
			return apierror.NewAPIError(validation.InvalidBodyContent, "Parameter `name` is required")
		}
		return h.restore(r.Context(), snapshotNamespace, snapshotName, input.Name)
	default:
		return apierror.NewAPIError(validation.InvalidAction, "Unsupported action")
	}
}

func (h *ActionHandler) restore(ctx context.Context, snapshotNamespace, snapshotName, newPVCName string) error {
	volumeSnapshot, err := h.snapshotCache.Get(snapshotNamespace, snapshotName)
	if err != nil {
		return err
	}

	apiGroup := snapshotv1.GroupName
	volumeMode := corev1.PersistentVolumeBlock
	accessModes := []corev1.PersistentVolumeAccessMode{
		corev1.ReadWriteMany,
	}
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceStorage: *volumeSnapshot.Status.RestoreSize,
		},
	}
	storageClassName := volumeSnapshot.Annotations[util.AnnotationStorageClassName]
	newPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        newPVCName,
			Namespace:   snapshotNamespace,
			Annotations: map[string]string{},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      accessModes,
			Resources:        resources,
			StorageClassName: &storageClassName,
			VolumeMode:       &volumeMode,
			DataSource: &corev1.TypedLocalObjectReference{
				Kind:     "VolumeSnapshot",
				Name:     snapshotName,
				APIGroup: &apiGroup,
			},
		},
	}

	if imageID := volumeSnapshot.Annotations[util.AnnotationImageID]; imageID != "" {
		newPVC.Annotations[util.AnnotationImageID] = imageID
	}

	if _, err = h.pvcs.Create(newPVC); err != nil {
		logrus.Errorf("failed to restore volume snapshot %s/%s to new pvc %s", snapshotNamespace, snapshotName, newPVCName)
		return err
	}

	return nil
}
