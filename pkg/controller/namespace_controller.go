/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Community License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Community-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	"go.bytebuilders.dev/aceshifter/pkg/tracker"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	uiapi "kmodules.xyz/resource-metadata/apis/ui/v1alpha1"
	clusterv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/yaml"
)

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Namespace object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var ns core.Namespace
	if err := r.Get(ctx, req.NamespacedName, &ns); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	_, ok := ns.Annotations[tracker.KeyUid]
	if !ok {
		return ctrl.Result{}, nil
	}

	nsMap := map[string]int64{}

	var list uiapi.FeatureList
	err := r.List(context.TODO(), &list)
	if err != nil {
		return ctrl.Result{}, nil
	}
	for _, feature := range list.Items {
		ns := feature.Spec.Chart.Namespace
		_, ok := nsMap[ns]
		if !ok {
			uidStart, _, err := tracker.GetUid(r.Client, ns)
			if err != nil {
				return ctrl.Result{}, nil
			}
			nsMap[ns] = uidStart
		}
	}

	data, err := yaml.Marshal(nsMap)
	if err != nil {
		return ctrl.Result{}, nil
	}

	cc := &clusterv1alpha1.ClusterClaim{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: tracker.OpenShiftClusterClaim,
		},
	}
	result, err := controllerutil.CreateOrPatch(context.Background(), r.Client, cc, func() error {
		cc.Spec.Value = string(data)
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if result != controllerutil.OperationResultNone {
		log.Info(fmt.Sprintf("ClusterClaim %s %s", cc.Name, result))
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&core.Namespace{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			_, ok := obj.GetAnnotations()[tracker.KeyUid]
			if !ok {
				return false
			}

			var list uiapi.FeatureList
			err := r.List(context.TODO(), &list)
			if err != nil {
				return false
			}
			for _, feature := range list.Items {
				if feature.Spec.Chart.Namespace == obj.GetName() {
					return true
				}
			}
			return false
		}))).
		Complete(r)
}
