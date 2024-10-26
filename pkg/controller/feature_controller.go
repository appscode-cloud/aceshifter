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

	"go.bytebuilders.dev/aceshifter/pkg/featuresets"
	"go.bytebuilders.dev/aceshifter/pkg/tracker"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	uiapi "kmodules.xyz/resource-metadata/apis/ui/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// FeatureReconciler reconciles a Feature object
type FeatureReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Feature object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *FeatureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var feature uiapi.Feature
	if err := r.Get(ctx, req.NamespacedName, &feature); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	uidStart, _, err := tracker.GetUid(r.Client, feature.Spec.Chart.Namespace)
	if err != nil || uidStart == tracker.UidNone {
		return ctrl.Result{}, err
	}

	cm := core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ace-openshift-scc",
			Namespace: "kubeops",
		},
	}
	result, err := controllerutil.CreateOrPatch(ctx, r.Client, &cm, func() error {
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}

		vals, err := featuresets.Render(feature, uidStart)
		if err != nil {
			cm.Data[feature.Name+".yaml"] = "{}"
		} else {
			cm.Data[feature.Name+".yaml"] = string(vals)
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if result != controllerutil.OperationResultNone {
		log.Info(fmt.Sprintf("%s configmap", result))
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FeatureReconciler) SetupWithManager(mgr ctrl.Manager) error {
	fn := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		log := log.FromContext(ctx)

		var list uiapi.FeatureList
		err := r.List(context.TODO(), &list)
		if err != nil {
			log.Error(err, "unable to list features")
			return nil
		}

		reqs := make([]reconcile.Request, 0, len(list.Items))
		for _, feature := range list.Items {
			if feature.Spec.Chart.Namespace != obj.GetName() {
				continue
			}
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: feature.Name},
			})
		}
		return reqs
	})
	return ctrl.NewControllerManagedBy(mgr).
		For(&uiapi.Feature{}).
		Watches(&core.Namespace{}, fn, builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			_, ok := obj.GetAnnotations()[tracker.KeyUid]
			return ok
		}))).
		Complete(r)
}
