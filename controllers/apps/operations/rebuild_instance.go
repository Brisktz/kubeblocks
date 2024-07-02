/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package operations

import (
	"fmt"
	"reflect"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

const (
	rebuildFromAnnotation  = "apps.kubeblocks.io/rebuild-from"
	rebuildTmpPVCNameLabel = "apps.kubeblocks.io/rebuild-tmp-pvc"

	waitingForInstanceReadyMessage   = "Waiting for the rebuilding instance to be ready"
	waitingForPostReadyRestorePrefix = "Waiting for postReady Restore"

	ignoreRoleCheckAnnotationKey = "kubeblocks.io/ignore-role-check"
)

type rebuildInstanceWrapper struct {
	replicas int32
	insNames []string
}

type rebuildInstanceOpsHandler struct{}

var _ OpsHandler = rebuildInstanceOpsHandler{}

func init() {
	rebuildInstanceBehaviour := OpsBehaviour{
		FromClusterPhases: []appsv1alpha1.ClusterPhase{appsv1alpha1.AbnormalClusterPhase, appsv1alpha1.FailedClusterPhase, appsv1alpha1.UpdatingClusterPhase},
		ToClusterPhase:    appsv1alpha1.UpdatingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        rebuildInstanceOpsHandler{},
	}
	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.RebuildInstanceType, rebuildInstanceBehaviour)
}

// ActionStartedCondition the started condition when handle the rebuild-instance request.
func (r rebuildInstanceOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewInstancesRebuildingCondition(opsRes.OpsRequest), nil
}

func (r rebuildInstanceOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	for _, v := range opsRes.OpsRequest.Spec.RebuildFrom {
		compStatus, ok := opsRes.Cluster.Status.Components[v.ComponentName]
		if !ok {
			continue
		}
		// check if the component has matched the `Phase` condition
		if !opsRes.OpsRequest.Spec.Force && !slices.Contains([]appsv1alpha1.ClusterComponentPhase{appsv1alpha1.FailedClusterCompPhase,
			appsv1alpha1.AbnormalClusterCompPhase, appsv1alpha1.UpdatingClusterCompPhase}, compStatus.Phase) {
			return intctrlutil.NewFatalError(fmt.Sprintf(`the phase of component "%s" can not be %s`, v.ComponentName, compStatus.Phase))
		}
		var (
			synthesizedComp *component.SynthesizedComponent
			err             error
		)
		for _, ins := range v.Instances {
			targetPod := &corev1.Pod{}
			if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: ins.Name, Namespace: opsRes.Cluster.Namespace}, targetPod); err != nil {
				return err
			}
			synthesizedComp, err = r.buildSynthesizedComponent(reqCtx, cli, opsRes.Cluster, targetPod.Labels[constant.KBAppComponentLabelKey])
			if err != nil {
				return err
			}
			isAvailable, _ := instanceIsAvailable(synthesizedComp, targetPod, "")
			if !opsRes.OpsRequest.Spec.Force && isAvailable {
				return intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" is availabled, can not rebuild it`, ins.Name))
			}
		}
		if !v.InPlace {
			if synthesizedComp.Name != v.ComponentName {
				return intctrlutil.NewFatalError("sharding cluster only supports to rebuild instance in place")
			}
			// validate when rebuilding instance with horizontal scaling
			if err = r.validateRebuildInstanceWithHScale(reqCtx, cli, opsRes, synthesizedComp); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r rebuildInstanceOpsHandler) validateRebuildInstanceWithHScale(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	synthesizedComp *component.SynthesizedComponent) error {
	// rebuild instance by horizontal scaling
	pods, err := component.ListOwnedPods(reqCtx.Ctx, cli, opsRes.Cluster.Namespace, opsRes.Cluster.Name, synthesizedComp.Name)
	if err != nil {
		return err
	}
	for _, v := range pods {
		available, _ := instanceIsAvailable(synthesizedComp, v, "")
		if !available {
			continue
		}
		if len(synthesizedComp.Roles) == 0 {
			return nil
		}
		for _, role := range synthesizedComp.Roles {
			// existing readWrite instance
			if role.Writable && v.Labels[constant.RoleLabelKey] == role.Name {
				return nil
			}
		}
	}
	return intctrlutil.NewFatalError("Due to insufficient available instances, horizontal scaling cannot be used for rebuilding instances." +
		"but you can rebuild instances in place with backup by set 'inPlace' to 'true'.")
}

func (r rebuildInstanceOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.RebuildFrom)
	getLastComponentInfo := func(compSpec appsv1alpha1.ClusterComponentSpec, comOps ComponentOpsInteface) appsv1alpha1.LastComponentConfiguration {
		lastCompConfiguration := appsv1alpha1.LastComponentConfiguration{
			Replicas:         pointer.Int32(compSpec.Replicas),
			Instances:        compSpec.Instances,
			OfflineInstances: compSpec.OfflineInstances,
		}
		return lastCompConfiguration
	}
	compOpsHelper.saveLastConfigurations(opsRes, getLastComponentInfo)
	return nil
}

func (r rebuildInstanceOpsHandler) getInstanceProgressDetail(compStatus appsv1alpha1.OpsRequestComponentStatus, instance string) appsv1alpha1.ProgressStatusDetail {
	objectKey := getProgressObjectKey(constant.PodKind, instance)
	progressDetail := findStatusProgressDetail(compStatus.ProgressDetails, objectKey)
	if progressDetail != nil {
		return *progressDetail
	}
	return appsv1alpha1.ProgressStatusDetail{
		ObjectKey: objectKey,
		Status:    appsv1alpha1.ProcessingProgressStatus,
		Message:   fmt.Sprintf("Start to rebuild pod %s", instance),
	}
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for restart opsRequest.
func (r rebuildInstanceOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		oldOpsRequest   = opsRes.OpsRequest.DeepCopy()
		oldCluster      = opsRes.Cluster.DeepCopy()
		opsRequestPhase = opsRes.OpsRequest.Status.Phase
		expectCount     int
		completedCount  int
		failedCount     int
		err             error
	)
	if opsRes.OpsRequest.Status.Components == nil {
		opsRes.OpsRequest.Status.Components = map[string]appsv1alpha1.OpsRequestComponentStatus{}
	}
	for _, v := range opsRes.OpsRequest.Spec.RebuildFrom {
		compStatus := opsRes.OpsRequest.Status.Components[v.ComponentName]
		var (
			subCompletedCount int
			subFailedCount    int
		)
		if v.InPlace {
			// rebuild instances in place.
			if subCompletedCount, subFailedCount, err = r.rebuildInstancesInPlace(reqCtx, cli, opsRes, v, &compStatus); err != nil {
				return opsRequestPhase, 0, err
			}
		} else {
			// rebuild instances with horizontal scaling
			if subCompletedCount, subFailedCount, err = r.rebuildInstancesWithHScaling(reqCtx, cli, opsRes, v, &compStatus); err != nil {
				return opsRequestPhase, 0, err
			}
		}
		expectCount += len(v.Instances)
		completedCount += subCompletedCount
		failedCount += subFailedCount
		opsRes.OpsRequest.Status.Components[v.ComponentName] = compStatus
	}
	if !reflect.DeepEqual(oldCluster.Spec, opsRes.Cluster.Spec) {
		if err = cli.Update(reqCtx.Ctx, opsRes.Cluster); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	if err = syncProgressToOpsRequest(reqCtx, cli, opsRes, oldOpsRequest, completedCount, expectCount); err != nil {
		return opsRequestPhase, 0, err
	}
	// check if the ops has been finished.
	if completedCount != expectCount {
		return opsRequestPhase, 0, nil
	}
	if failedCount == 0 {
		return appsv1alpha1.OpsSucceedPhase, 0, r.cleanupTmpResources(reqCtx, cli, opsRes)
	}
	return appsv1alpha1.OpsFailedPhase, 0, nil
}

func (r rebuildInstanceOpsHandler) rebuildInstancesWithHScaling(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	rebuildInstance appsv1alpha1.RebuildInstance,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (int, int, error) {
	var (
		completedCount int
		failedCount    int
	)
	if len(compStatus.ProgressDetails) == 0 {
		// 1. scale out the required instances
		r.scaleOutRequiredInstances(opsRes, rebuildInstance, compStatus)
		return 0, 0, nil
	}
	// 2. waiting for the necessary instances to complete the scaling-out process.

	// 3. offline the instances that require rebuilding when the new pod scaling out successfully.
	return 0, 0, nil
}

// getRebuildInstanceWrapper assembles the corresponding replicas and instances based on the template
func (r rebuildInstanceOpsHandler) getRebuildInstanceWrapper(opsRes *OpsResource, rebuildInstance appsv1alpha1.RebuildInstance) map[string]*rebuildInstanceWrapper {
	rebuildInsWrapper := map[string]*rebuildInstanceWrapper{}
	for _, ins := range rebuildInstance.Instances {
		insTplName := appsv1alpha1.GetInstanceTemplateName(opsRes.Cluster.Name, rebuildInstance.ComponentName, ins.Name)
		if _, ok := rebuildInsWrapper[insTplName]; !ok {
			rebuildInsWrapper[insTplName] = &rebuildInstanceWrapper{replicas: 1, insNames: []string{ins.Name}}
		} else {
			rebuildInsWrapper[insTplName].replicas += 1
			rebuildInsWrapper[insTplName].insNames = append(rebuildInsWrapper[insTplName].insNames, ins.Name)
		}
	}
	return rebuildInsWrapper
}

func (r rebuildInstanceOpsHandler) scaleOutRequiredInstances(opsRes *OpsResource,
	rebuildInstance appsv1alpha1.RebuildInstance,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) {
	// 1. sort the instances
	slices.SortFunc(rebuildInstance.Instances, func(a, b appsv1alpha1.Instance) bool {
		return a.Name < b.Name
	})

	// 2. assemble the corresponding replicas and instances based on the template
	rebuildInsWrapper := r.getRebuildInstanceWrapper(opsRes, rebuildInstance)

	// 3. update component spec to scale out required instances.
	compName := rebuildInstance.ComponentName
	lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[compName]
	scaleOutInsMap := map[string]string{}
	setScaleOutInsMap := func(workloadName, templateName string,
		replicas int32, offlineInstances []string, wrapper *rebuildInstanceWrapper) {
		insNames := instanceset.GenerateInstanceNamesFromTemplate(workloadName, "", replicas, offlineInstances)
		for i, insName := range wrapper.insNames {
			scaleOutInsMap[insName] = insNames[int(replicas-wrapper.replicas)+i]
		}
	}
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		compSpec := &opsRes.Cluster.Spec.ComponentSpecs[i]
		if compSpec.Name != compName {
			continue
		}
		if *lastCompConfiguration.Replicas != compSpec.Replicas {
			// means the componentSpec has been updated, ignore it.
			continue
		}
		workloadName := constant.GenerateWorkloadNamePattern(opsRes.Cluster.Name, compName)
		var allTemplateReplicas int32
		for j := range compSpec.Instances {
			insTpl := &compSpec.Instances[j]
			if wrapper, ok := rebuildInsWrapper[insTpl.Name]; ok {
				insTpl.Replicas = pointer.Int32(insTpl.GetReplicas() + wrapper.replicas)
				setScaleOutInsMap(workloadName, insTpl.Name, *insTpl.Replicas, compSpec.OfflineInstances, wrapper)
			}
			allTemplateReplicas += insTpl.GetReplicas()
		}
		compSpec.Replicas += int32(len(rebuildInstance.Instances))
		if wrapper, ok := rebuildInsWrapper[""]; ok {
			setScaleOutInsMap(workloadName, "", compSpec.Replicas-allTemplateReplicas, compSpec.OfflineInstances, wrapper)
		}
		break
	}
	// 4. set progress details
	for _, ins := range rebuildInstance.Instances {
		scaleOutInsName := scaleOutInsMap[ins.Name]
		setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails,
			appsv1alpha1.ProgressStatusDetail{
				ObjectKey: getProgressObjectKey(constant.PodKind, ins.Name),
				Status:    appsv1alpha1.ProcessingProgressStatus,
				Message:   fmt.Sprintf("Start to scale out a new pod %s", scaleOutInsName),
			})
	}
}

func (r rebuildInstanceOpsHandler) rebuildInstancesInPlace(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	rebuildInstance appsv1alpha1.RebuildInstance,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (int, int, error) {
	// rebuild instances in place.
	var (
		completedCount int
		failedCount    int
	)
	for i, instance := range rebuildInstance.Instances {
		progressDetail := r.getInstanceProgressDetail(*compStatus, instance.Name)
		if isCompletedProgressStatus(progressDetail.Status) {
			completedCount += 1
			if progressDetail.Status == appsv1alpha1.FailedProgressStatus {
				failedCount += 1
			}
			continue
		}
		// rebuild instance
		completed, err := r.rebuildInstance(reqCtx, cli, opsRes, &progressDetail, rebuildInstance, instance, i)
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
			// If a fatal error occurs, this instance rebuilds failed.
			progressDetail.SetStatusAndMessage(appsv1alpha1.FailedProgressStatus, err.Error())
			setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
			continue
		}
		if err != nil {
			return 0, 0, err
		}
		if completed {
			// if the pod has been rebuilt, set progressDetail phase to Succeed.
			progressDetail.SetStatusAndMessage(appsv1alpha1.SucceedProgressStatus,
				fmt.Sprintf("Rebuild pod %s successfully", instance.Name))
		}
		setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
	}
	return completedCount, failedCount, nil
}

// rebuildInstance rebuilds the instance.
func (r rebuildInstanceOpsHandler) rebuildInstance(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	progressDetail *appsv1alpha1.ProgressStatusDetail,
	rebuildFrom appsv1alpha1.RebuildInstance,
	instance appsv1alpha1.Instance,
	index int) (bool, error) {
	inPlaceHelper, err := r.prepareInplaceRebuildHelper(reqCtx, cli, opsRes, rebuildFrom.RestoreEnv,
		instance, rebuildFrom.BackupName, index)
	if err != nil {
		return false, err
	}

	if rebuildFrom.BackupName == "" {
		return inPlaceHelper.rebuildInstanceWithNoBackup(reqCtx, cli, opsRes, progressDetail)
	}
	return inPlaceHelper.rebuildInstanceWithBackup(reqCtx, cli, opsRes, progressDetail)
}

func (r rebuildInstanceOpsHandler) buildSynthesizedComponent(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentName string) (*component.SynthesizedComponent, error) {
	compSpec := getComponentSpecOrShardingTemplate(cluster, componentName)
	if compSpec.ComponentDef == "" {
		// TODO: remove after 0.9
		return component.BuildSynthesizedComponentWrapper(reqCtx, cli, cluster, compSpec)
	}
	comp, compDef, err := component.GetCompNCompDefByName(reqCtx.Ctx, cli, cluster.Namespace, constant.GenerateClusterComponentName(cluster.Name, componentName))
	if err != nil {
		return nil, err
	}
	return component.BuildSynthesizedComponent(reqCtx, cli, cluster, compDef, comp)
}

func (r rebuildInstanceOpsHandler) prepareInplaceRebuildHelper(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	envForRestore []corev1.EnvVar,
	instance appsv1alpha1.Instance,
	backupName string,
	index int) (*inplaceRebuildHelper, error) {
	var (
		backup          *dpv1alpha1.Backup
		actionSet       *dpv1alpha1.ActionSet
		synthesizedComp *component.SynthesizedComponent
		err             error
	)
	if backupName != "" {
		// prepare backup infos
		backup = &dpv1alpha1.Backup{}
		if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: backupName, Namespace: opsRes.Cluster.Namespace}, backup); err != nil {
			return nil, err
		}
		if backup.Labels[dptypes.BackupTypeLabelKey] != string(dpv1alpha1.BackupTypeFull) {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`the backup "%s" is not a Full backup`, backupName))
		}
		if backup.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`the backup "%s" phase is not Completed`, backupName))
		}
		if backup.Status.BackupMethod == nil {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`the backupMethod of the backup "%s" can not be empty`, backupName))
		}
		actionSet, err = dputils.GetActionSetByName(reqCtx, cli, backup.Status.BackupMethod.ActionSetName)
		if err != nil {
			return nil, err
		}
	}
	targetPod := &corev1.Pod{}
	if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: instance.Name, Namespace: opsRes.Cluster.Namespace}, targetPod); err != nil {
		return nil, err
	}
	synthesizedComp, err = r.buildSynthesizedComponent(reqCtx, cli, opsRes.Cluster, targetPod.Labels[constant.KBAppComponentLabelKey])
	if err != nil {
		return nil, err
	}
	rebuildPrefix := fmt.Sprintf("rebuild-%s", opsRes.OpsRequest.UID[:8])
	pvcMap, volumes, volumeMounts, err := r.getPVCMapAndVolumes(opsRes, synthesizedComp, targetPod, rebuildPrefix, index)
	if err != nil {
		return nil, err
	}
	return &inplaceRebuildHelper{
		index:           index,
		backup:          backup,
		instance:        instance,
		actionSet:       actionSet,
		synthesizedComp: synthesizedComp,
		pvcMap:          pvcMap,
		volumes:         volumes,
		targetPod:       targetPod,
		volumeMounts:    volumeMounts,
		rebuildPrefix:   rebuildPrefix,
		envForRestore:   envForRestore,
	}, nil
}

// getPVCMapAndVolumes gets the pvc map and the volume infos.
func (r rebuildInstanceOpsHandler) getPVCMapAndVolumes(opsRes *OpsResource,
	synthesizedComp *component.SynthesizedComponent,
	targetPod *corev1.Pod,
	rebuildPrefix string,
	index int) (map[string]*corev1.PersistentVolumeClaim, []corev1.Volume, []corev1.VolumeMount, error) {
	var (
		volumes      []corev1.Volume
		volumeMounts []corev1.VolumeMount
		// key: source pvc name, value: tmp pvc
		pvcMap       = map[string]*corev1.PersistentVolumeClaim{}
		volumePVCMap = map[string]string{}
	)
	for _, volume := range targetPod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			volumePVCMap[volume.Name] = volume.PersistentVolumeClaim.ClaimName
		}
	}
	// backup's ready, then start to check restore
	workloadName := constant.GenerateWorkloadNamePattern(opsRes.Cluster.Name, synthesizedComp.Name)
	templateName, _, err := component.GetTemplateNameAndOrdinal(workloadName, targetPod.Name)
	if err != nil {
		return nil, nil, nil, err
	}
	// TODO: create pvc by the volumeClaimTemplates of instance template if it is necessary.
	for i, vct := range synthesizedComp.VolumeClaimTemplates {
		sourcePVCName := volumePVCMap[vct.Name]
		if sourcePVCName == "" {
			return nil, nil, nil, intctrlutil.NewFatalError("")
		}
		pvcLabels := getWellKnownLabels(synthesizedComp)
		tmpPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-%d", rebuildPrefix, common.CutString(synthesizedComp.Name+"-"+vct.Name, 30), index),
				Namespace: targetPod.Namespace,
				Labels:    pvcLabels,
				Annotations: map[string]string{
					rebuildFromAnnotation: opsRes.OpsRequest.Name,
				},
			},
			Spec: vct.Spec,
		}
		factory.BuildPersistentVolumeClaimLabels(synthesizedComp, tmpPVC, vct.Name, templateName)
		pvcMap[sourcePVCName] = tmpPVC
		// build volumes and volumeMount
		volumes = append(volumes, corev1.Volume{
			Name: vct.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: tmpPVC.Name,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      vct.Name,
			MountPath: fmt.Sprintf("/kb-tmp/%d", i),
		})
	}
	return pvcMap, volumes, volumeMounts, nil
}

// cleanupTmpResources clean up the temporary resources generated during the process of rebuilding the instance.
func (r rebuildInstanceOpsHandler) cleanupTmpResources(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource) error {
	matchLabels := client.MatchingLabels{
		constant.OpsRequestNameLabelKey:      opsRes.OpsRequest.Name,
		constant.OpsRequestNamespaceLabelKey: opsRes.OpsRequest.Namespace,
	}
	// TODO: need to delete the restore CR?
	// Pods are limited in k8s, so we need to release them if they are not needed.
	return intctrlutil.DeleteOwnedResources(reqCtx.Ctx, cli, opsRes.OpsRequest, matchLabels, generics.PodSignature)
}
