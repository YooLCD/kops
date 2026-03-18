/*
Copyright 2026 The Kubernetes Authors.

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

package validation

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/kubemanifest"
)

func parseCluster(data []byte) *kops.Cluster {
	objs, err := kubemanifest.LoadObjectsFrom(data)
	if err != nil || len(objs) == 0 {
		return nil
	}
	raw, err := objs[0].ToYAML()
	if err != nil {
		return nil
	}
	c := &kops.Cluster{}
	if err := yaml.Unmarshal(raw, c); err != nil {
		return nil
	}
	return c
}

func parseInstanceGroup(data []byte) *kops.InstanceGroup {
	objs, err := kubemanifest.LoadObjectsFrom(data)
	if err != nil || len(objs) == 0 {
		return nil
	}
	raw, err := objs[0].ToYAML()
	if err != nil {
		return nil
	}
	ig := &kops.InstanceGroup{}
	if err := yaml.Unmarshal(raw, ig); err != nil {
		return nil
	}
	return ig
}

func fuzzFieldPath() *field.Path {
	return field.NewPath("object")
}

var seedClusterAWS = []byte(`
apiVersion: kops.k8s.io/v1alpha2
kind: Cluster
metadata:
  name: fuzz.k8s.local
spec:
  cloudProvider:
    aws: {}
  kubernetesVersion: "1.28.0"
  networkCIDR: "10.0.0.0/16"
  subnets:
  - name: us-east-1a
    type: Public
    zone: us-east-1a
    cidr: "10.0.1.0/24"
  etcdClusters:
  - name: main
    members:
    - name: a
      instanceGroup: master-us-east-1a
  - name: events
    members:
    - name: a
      instanceGroup: master-us-east-1a
  topology:
    masters: public
    nodes: public
  networking:
    cni: {}
`)

var seedClusterGCE = []byte(`
apiVersion: kops.k8s.io/v1alpha2
kind: Cluster
metadata:
  name: fuzz-gce.k8s.local
spec:
  cloudProvider:
    gce:
      project: fuzz-project
  kubernetesVersion: "1.28.0"
  networkCIDR: "10.0.0.0/16"
  subnets:
  - name: us-central1
    type: Public
    zone: us-central1-a
    cidr: "10.0.1.0/24"
  etcdClusters:
  - name: main
    members:
    - name: a
      instanceGroup: master-us-central1-a
  - name: events
    members:
    - name: a
      instanceGroup: master-us-central1-a
  topology:
    masters: public
    nodes: public
  networking:
    cni: {}
`)

var seedIGControlPlane = []byte(`
apiVersion: kops.k8s.io/v1alpha2
kind: InstanceGroup
metadata:
  name: master-us-east-1a
  labels:
    kops.k8s.io/cluster: fuzz.k8s.local
spec:
  role: ControlPlane
  machineType: m5.large
  minSize: 1
  maxSize: 1
  subnets:
  - us-east-1a
`)

var seedIGNode = []byte(`
apiVersion: kops.k8s.io/v1alpha2
kind: InstanceGroup
metadata:
  name: nodes
  labels:
    kops.k8s.io/cluster: fuzz.k8s.local
spec:
  role: Node
  machineType: m5.xlarge
  minSize: 2
  maxSize: 5
  subnets:
  - us-east-1a
`)

var seedIGBastion = []byte(`
apiVersion: kops.k8s.io/v1alpha2
kind: InstanceGroup
metadata:
  name: bastion
  labels:
    kops.k8s.io/cluster: fuzz.k8s.local
spec:
  role: Bastion
  machineType: t3.micro
  minSize: 1
  maxSize: 1
  subnets:
  - us-east-1a
`)

func FuzzValidateCluster(f *testing.F) {
	f.Add(seedClusterAWS)
	f.Add(seedClusterGCE)
	f.Add([]byte(""))
	f.Add([]byte("{}"))
	f.Add([]byte("kind: Cluster"))
	f.Add([]byte("apiVersion: kops.k8s.io/v1alpha2\nkind: Cluster\nmetadata:\n  name: x.k8s.local"))

	f.Fuzz(func(t *testing.T, data []byte) {
		c := parseCluster(data)
		if c == nil {
			return
		}
		_ = ValidateCluster(c, false, nil)
		_ = ValidateCluster(c, true, nil)
	})
}

func FuzzValidateClusterUpdate(f *testing.F) {
	f.Add(seedClusterAWS, seedClusterAWS)
	f.Add(seedClusterAWS, seedClusterGCE)
	f.Add(seedClusterGCE, seedClusterAWS)
	f.Add(seedClusterGCE, seedClusterGCE)
	// old=nil triggers a nil pointer dereference at cluster.go:39; fixed in accompanying PR.
	f.Add(seedClusterAWS, []byte(""))

	f.Fuzz(func(t *testing.T, dataNew []byte, dataOld []byte) {
		newCluster := parseCluster(dataNew)
		if newCluster == nil {
			return
		}
		// oldCluster may be nil; ValidateClusterUpdate must handle this gracefully.
		oldCluster := parseCluster(dataOld)
		_ = ValidateClusterUpdate(newCluster, nil, oldCluster, nil)
	})
}

func FuzzValidateInstanceGroup(f *testing.F) {
	f.Add(seedIGControlPlane)
	f.Add(seedIGNode)
	f.Add(seedIGBastion)
	f.Add([]byte(""))
	f.Add([]byte("{}"))
	f.Add([]byte("kind: InstanceGroup"))

	f.Fuzz(func(t *testing.T, data []byte) {
		ig := parseInstanceGroup(data)
		if ig == nil {
			return
		}
		_ = ValidateInstanceGroup(ig, nil, false)
		_ = ValidateInstanceGroup(ig, nil, true)
	})
}

func FuzzValidateControlPlaneInstanceGroup(f *testing.F) {
	f.Add(seedIGControlPlane, seedClusterAWS)
	f.Add(seedIGControlPlane, seedClusterGCE)

	f.Fuzz(func(t *testing.T, igData []byte, clusterData []byte) {
		ig := parseInstanceGroup(igData)
		if ig == nil {
			return
		}
		cluster := parseCluster(clusterData)
		if cluster == nil {
			return
		}
		_ = ValidateControlPlaneInstanceGroup(ig, cluster)
	})
}

func FuzzDeepValidate(f *testing.F) {
	f.Add(seedClusterAWS, seedIGControlPlane, seedIGNode)
	f.Add(seedClusterGCE, seedIGControlPlane, seedIGNode)
	f.Add(seedClusterAWS, seedIGControlPlane, seedIGBastion)

	f.Fuzz(func(t *testing.T, clusterData []byte, igData1 []byte, igData2 []byte) {
		cluster := parseCluster(clusterData)
		if cluster == nil {
			return
		}
		var groups []*kops.InstanceGroup
		if ig := parseInstanceGroup(igData1); ig != nil {
			groups = append(groups, ig)
		}
		if ig := parseInstanceGroup(igData2); ig != nil {
			groups = append(groups, ig)
		}
		if len(groups) == 0 {
			return
		}
		_ = DeepValidate(cluster, groups, false, nil, nil)
		_ = DeepValidate(cluster, groups, true, nil, nil)
	})
}

func FuzzValidateAdditionalObject(f *testing.F) {
	f.Add([]byte(`{"apiVersion":"kubescheduler.config.k8s.io/v1","kind":"KubeSchedulerConfiguration","clientConnection":{"kubeconfig":""}}`))
	f.Add([]byte(`{"apiVersion":"kubescheduler.config.k8s.io/v1","kind":"KubeSchedulerConfiguration","clientConnection":{"kubeconfig":"/etc/kubernetes/scheduler.conf"}}`))
	f.Add([]byte(`{"apiVersion":"v1","kind":"ConfigMap"}`))
	f.Add([]byte("{}"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(data, &obj.Object); err != nil {
			return
		}
		if obj.Object == nil {
			return
		}
		_ = ValidateAdditionalObject(context.Background(), fuzzFieldPath(), obj)
	})
}