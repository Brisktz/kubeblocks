/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ExporterConfig struct {
	ExporterRef string
}

type MetricsCollector struct {
	CollectionInterval time.Duration `json:"collectionInterval"`
	ExporterRefs       []string      `json:"exporterRefs"`
}

type LogsCollector struct {
	ComponentName string
	// reference Clusterdefinition.Spec.[*].LogConfigs[]
	LogFileType []string

	ExporterRef string
}

// AgamottoCollectorSpec defines the desired state of AgamottoCollector
type AgamottoCollectorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// clusterRef references clusterDefinition.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterRef"
	ClusterRef string `json:"clusterRef"`

	MetricsCollector *MetricsCollector `json:"metricsCollector"`
	LogsCollector    []LogsCollector   `json:"logsCollector"`
}

// AgamottoCollectorStatus defines the observed state of AgamottoCollector
type AgamottoCollectorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// AgamottoCollector is the Schema for the agamottocollectors API
type AgamottoCollector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgamottoCollectorSpec   `json:"spec,omitempty"`
	Status AgamottoCollectorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AgamottoCollectorList contains a list of AgamottoCollector
type AgamottoCollectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgamottoCollector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgamottoCollector{}, &AgamottoCollectorList{})
}
