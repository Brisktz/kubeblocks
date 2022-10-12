/*
Copyright 2022 The KubeBlocks Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package operations

import (
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

// ReconcileActionWithCluster it will be performed when action is done and loop util OpsRequest.status.phase is Succeed.
// if OpsRequest.spec.clusterOps is not null, you can use it to OpsBehaviour.ReconcileAction.
func ReconcileActionWithCluster(opsRes *OpsResource) error {
	var (
		opsRequest = opsRes.OpsRequest
		isChanged  bool
	)
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]dbaasv1alpha1.OpsRequestStatusComponent{}
	}
	for k, v := range opsRes.Cluster.Status.Components {
		// the operation occurs in the cluster, such as upgrade.
		// However, it is also possible that only the corresponding components in the cluster have changed,
		// and the phase is updating So we need to monitor these components and send the corresponding event
		if statusComponent, ok := opsRequest.Status.Components[k]; (!ok && v.Phase == dbaasv1alpha1.UpdatingPhase) || statusComponent.Phase != v.Phase {
			isChanged = true
			opsRequest.Status.Components[k] = dbaasv1alpha1.OpsRequestStatusComponent{Phase: v.Phase}
			sendEventWhenComponentPhaseChanged(opsRes, k, v.Phase)
		}
	}
	if isChanged {
		if err := opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch); err != nil {
			return err
		}
	}
	if opsRes.Cluster.Status.Phase != dbaasv1alpha1.RunningPhase {
		return fmt.Errorf("opsRequest is not completed")
	}
	return nil
}

// ReconcileActionWithComponentOps it will be performed when action is done and loop util OpsRequest.status.phase is Succeed.
// if OpsRequest.spec.componentOps is not null, you can use it to OpsBehaviour.ReconcileAction.
func ReconcileActionWithComponentOps(opsRes *OpsResource) error {
	var (
		opsRequest = opsRes.OpsRequest
		isOk       = true
		isChanged  bool
	)
	componentNameMap := getAllComponentsNameMap(opsRequest)
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]dbaasv1alpha1.OpsRequestStatusComponent{}
	}
	for k, v := range opsRes.Cluster.Status.Components {
		if _, ok := componentNameMap[k]; !ok {
			continue
		}
		if v.Phase != dbaasv1alpha1.RunningPhase {
			isOk = false
		}
		if statusComponent, ok := opsRequest.Status.Components[k]; !ok || statusComponent.Phase != v.Phase {
			isChanged = true
			opsRequest.Status.Components[k] = dbaasv1alpha1.OpsRequestStatusComponent{Phase: v.Phase}
			sendEventWhenComponentPhaseChanged(opsRes, k, v.Phase)
		}
	}
	if isChanged {
		if err := opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch); err != nil {
			return err
		}
	}
	if !isOk {
		return fmt.Errorf("opsRequest is not completed")
	}
	return nil
}

// sendEventWhenComponentStatusChanged send an event when OpsRequest.status.components[*].phase is changed
func sendEventWhenComponentPhaseChanged(opsRes *OpsResource, componentName string, phase dbaasv1alpha1.Phase) {
	var (
		tip    string
		reason string
	)
	if phase == dbaasv1alpha1.RunningPhase {
		tip = "Successfully"
		reason = dbaasv1alpha1.ReasonSuccessful
	} else {
		reason = dbaasv1alpha1.ReasonStarting
	}
	message := fmt.Sprintf("%s %s component: %s in Cluster: %s",
		tip, opsRes.OpsRequest.Spec.Type, componentName, opsRes.OpsRequest.Spec.ClusterRef)
	opsRes.Recorder.Event(opsRes.OpsRequest, corev1.EventTypeNormal, reason, message)
}

// PatchOpsStatus patch OpsRequest.status
func PatchOpsStatus(opsRes *OpsResource,
	phase dbaasv1alpha1.Phase,
	condition ...*metav1.Condition) error {

	opsRequest := opsRes.OpsRequest
	patch := client.MergeFrom(opsRequest.DeepCopy())
	for _, v := range condition {
		if v == nil {
			continue
		}
		opsRequest.SetStatusCondition(*v)
		// provide an event
		eventType := corev1.EventTypeNormal
		if phase == dbaasv1alpha1.FailedPhase {
			eventType = corev1.EventTypeWarning
		}
		opsRes.Recorder.Event(opsRequest, eventType, v.Reason, v.Message)
	}
	if slices.Index([]dbaasv1alpha1.Phase{dbaasv1alpha1.SucceedPhase, dbaasv1alpha1.FailedPhase}, phase) != -1 {
		opsRequest.Status.CompletionTimestamp = &metav1.Time{Time: time.Now()}
		// when OpsRequest is completed, do it
		if err := deleteOpsRequestAnnotationInCluster(opsRes); err != nil {
			return err
		}
	}
	if phase == dbaasv1alpha1.RunningPhase && opsRequest.Status.Phase != phase {
		opsRequest.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
	}
	opsRequest.Status.Phase = phase
	return opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch)
}

func PatchClusterNotFound(opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.clusterRef %s is not Found", opsRes.OpsRequest.Spec.ClusterRef)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonClusterNotFound, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

func patchOpsBehaviourNotFound(opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.type %s is not supported", opsRes.OpsRequest.Spec.Type)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonOpsTypeNotSupported, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

func patchClusterPhaseMisMatch(opsRes *OpsResource) error {
	message := fmt.Sprintf("can not run the OpsRequest when Cluster.status.phase is %s in spec.clusterRef: %s",
		opsRes.Cluster.Status.Phase, opsRes.Cluster.Name)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonClusterPhaseMisMatch, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

func patchClusterExistOtherOperation(opsRes *OpsResource, opsRequestName string) error {
	message := fmt.Sprintf("spec.clusterRef: %s is running the OpsRequest: %s",
		opsRes.Cluster.Name, opsRequestName)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonClusterExistOtherOperation, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

// getOpsRequestAnnotation get OpsRequest.name from cluster.annotations
func getOpsRequestAnnotation(cluster *dbaasv1alpha1.Cluster, toClusterPhase dbaasv1alpha1.Phase) *string {
	var (
		opsRequestValue string
		opsRequestMap   map[dbaasv1alpha1.Phase]string
		ok              bool
	)
	if cluster.Annotations == nil {
		return nil
	}
	if opsRequestValue, ok = cluster.Annotations[OpsRequestAnnotationKey]; !ok {
		return nil
	}
	// opsRequest annotation value in cluster to map
	if err := json.Unmarshal([]byte(opsRequestValue), &opsRequestMap); err != nil {
		return nil
	}
	if val, ok := opsRequestMap[toClusterPhase]; ok {
		return &val
	}
	return nil
}

// patchOpsRequestToRunning patch OpsRequest.status.phase to Running
func patchOpsRequestToRunning(opsRes *OpsResource, opsBehaviour *OpsBehaviour) error {
	var condition *metav1.Condition
	validatePassCondition := dbaasv1alpha1.NewValidatePassedCondition(opsRes.OpsRequest.Name)
	if opsBehaviour.ActionStartedCondition != nil {
		condition = opsBehaviour.ActionStartedCondition(opsRes.OpsRequest)
	}
	return PatchOpsStatus(opsRes, dbaasv1alpha1.RunningPhase, validatePassCondition, condition)
}

// getAllComponentsNameMap covert spec.componentOps.componentNames list to map
func getAllComponentsNameMap(opsRequest *dbaasv1alpha1.OpsRequest) map[string]*dbaasv1alpha1.ComponentOps {
	if opsRequest.Spec.ComponentOpsList == nil {
		return map[string]*dbaasv1alpha1.ComponentOps{}
	}
	componentNameMap := make(map[string]*dbaasv1alpha1.ComponentOps)
	for _, componentOps := range opsRequest.Spec.ComponentOpsList {
		for _, v := range componentOps.ComponentNames {
			componentNameMap[v] = componentOps
		}
	}
	return componentNameMap
}

// patchClusterStatus update Cluster.status to record cluster and components information
func patchClusterStatus(opsRes *OpsResource, toClusterState dbaasv1alpha1.Phase) error {
	if toClusterState == "" {
		return nil
	}
	componentNameMap := getAllComponentsNameMap(opsRes.OpsRequest)
	patch := client.MergeFrom(opsRes.Cluster.DeepCopy())
	opsRes.Cluster.Status.Phase = toClusterState
	if componentNameMap != nil && opsRes.Cluster.Status.Components != nil {
		for k, v := range opsRes.Cluster.Status.Components {
			if _, ok := componentNameMap[k]; ok {
				v.Phase = toClusterState
			}
		}
	}
	if err := opsRes.Client.Status().Patch(opsRes.Ctx, opsRes.Cluster, patch); err != nil {
		return err
	}
	opsRes.Recorder.Eventf(opsRes.Cluster, corev1.EventTypeNormal, string(opsRes.OpsRequest.Spec.Type),
		"Start %s in Cluster: %s", opsRes.OpsRequest.Spec.Type, opsRes.Cluster.Name)
	return nil
}

// deleteOpsRequestAnnotationInCluster when OpsRequest.status.phase is Succeed or Failed
// we should delete the OpsRequest Annotation in cluster, unlock cluster
func deleteOpsRequestAnnotationInCluster(opsRes *OpsResource) error {
	var (
		opsBehaviour    *OpsBehaviour
		opsRequestValue string
		opsRequestMap   map[dbaasv1alpha1.Phase]string
		ok              bool
	)
	if opsRes.Cluster.Annotations == nil {
		return nil
	}
	if opsRequestValue, ok = opsRes.Cluster.Annotations[OpsRequestAnnotationKey]; !ok {
		return nil
	}
	if err := json.Unmarshal([]byte(opsRequestValue), &opsRequestMap); err != nil {
		return err
	}
	if opsBehaviour, ok = GetOpsManager().OpsMap[opsRes.OpsRequest.Spec.Type]; !ok {
		return nil
	}
	if val, ok := opsRequestMap[opsBehaviour.ToClusterPhase]; !ok || val != opsRes.OpsRequest.Name {
		return nil
	}
	// delete the opsRequest information in Cluster.annotations
	delete(opsRequestMap, opsBehaviour.ToClusterPhase)
	if err := patchClusterPhaseWhenExistsOtherOps(opsRes, opsRequestMap); err != nil {
		return err
	}
	return patchClusterAnnotations(opsRes, opsRequestMap)
}

// addOpsRequestAnnotationToCluster when OpsRequest.phase is Running, we should add the OpsRequest Annotation to Cluster.metadata.Annotations
func addOpsRequestAnnotationToCluster(opsRes *OpsResource, toClusterPhase dbaasv1alpha1.Phase) error {
	var (
		opsRequestMap   map[dbaasv1alpha1.Phase]string
		opsRequestValue string
		ok              bool
	)
	if toClusterPhase == "" {
		return nil
	}
	if opsRes.Cluster.Annotations == nil {
		opsRes.Cluster.Annotations = map[string]string{}
	}
	if opsRequestValue, ok = opsRes.Cluster.Annotations[OpsRequestAnnotationKey]; !ok {
		opsRequestValue = "{}"
	}
	if err := json.Unmarshal([]byte(opsRequestValue), &opsRequestMap); err != nil {
		return err
	}
	opsRequestMap[toClusterPhase] = opsRes.OpsRequest.Name
	return patchClusterAnnotations(opsRes, opsRequestMap)
}

// patchClusterAnnotations patch OpsRequest annotation in Cluster.annotations
func patchClusterAnnotations(opsRes *OpsResource, opsRequestMap map[dbaasv1alpha1.Phase]string) error {
	patch := client.MergeFrom(opsRes.Cluster.DeepCopy())
	if len(opsRequestMap) > 0 {
		result, _ := json.Marshal(opsRequestMap)
		opsRes.Cluster.Annotations[OpsRequestAnnotationKey] = string(result)
	} else {
		delete(opsRes.Cluster.Annotations, OpsRequestAnnotationKey)
	}
	return opsRes.Client.Patch(opsRes.Ctx, opsRes.Cluster, patch)
}

// patchClusterPhaseWhenExistsOtherOps
func patchClusterPhaseWhenExistsOtherOps(opsRes *OpsResource, opsRequestMap map[dbaasv1alpha1.Phase]string) error {
	// If there are other OpsRequests running, modify the cluster.status.phase with other opsRequest's ToClusterPhase
	if len(opsRequestMap) == 0 {
		return nil
	}
	patch := client.MergeFrom(opsRes.Cluster.DeepCopy())
	for k := range opsRequestMap {
		opsRes.Cluster.Status.Phase = k
		break
	}
	if err := opsRes.Client.Status().Patch(opsRes.Ctx, opsRes.Cluster, patch); err != nil {
		return err
	}
	return nil
}
