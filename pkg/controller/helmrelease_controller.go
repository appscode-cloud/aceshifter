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

	helmapi "github.com/fluxcd/helm-controller/api/v2"
	core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// HelmReleaseReconciler reconciles a Feature object
type HelmReleaseReconciler struct {
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
func (r *HelmReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var hr helmapi.HelmRelease
	if err := r.Get(ctx, req.NamespacedName, &hr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var feature uiapi.Feature
	var filename string
	if err := r.Get(ctx, client.ObjectKey{Name: hr.Name}, &feature); err != nil {
		if apierrors.IsNotFound(err) && hr.Name == "ace" {
			filename = hr.Name + ".yaml"
		} else {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	} else {
		filename = fmt.Sprintf("%s/%s.yaml", feature.Spec.FeatureSet, feature.Name)
	}

	ns := core.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: hr.Spec.TargetNamespace,
		},
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(&ns), &ns); apierrors.IsNotFound(err) {
		if err := r.Create(ctx, &ns); err != nil {
			return ctrl.Result{}, client.IgnoreAlreadyExists(err)
		}
	}

	uidStart, _, err := tracker.GetUid(r.Client, ns.Name)
	if err != nil || uidStart == tracker.UidNone {
		return ctrl.Result{}, err
	}

	cm := core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ace-openshift-scc",
			Namespace: "kubeops",
		},
	}
	configKey := hr.Name + ".yaml"
	result, err := controllerutil.CreateOrPatch(ctx, r.Client, &cm, func() error {
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}

		vals, err := featuresets.Render(filename, uidStart)
		if err != nil {
			cm.Data[configKey] = "{}"
		} else {
			cm.Data[configKey] = string(vals)
		}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if result != controllerutil.OperationResultNone {
		log.Info(fmt.Sprintf("%s configmap key %s", result, configKey))
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mapNamespaceToHelmRelease := func(ctx context.Context, obj client.Object) []reconcile.Request {
		log := log.FromContext(ctx)

		var list helmapi.HelmReleaseList
		err := r.List(context.TODO(), &list)
		if err != nil {
			log.Error(err, "unable to list features")
			return nil
		}

		reqs := make([]reconcile.Request, 0, len(list.Items))
		for _, hr := range list.Items {
			if hr.Spec.TargetNamespace != obj.GetName() {
				continue
			}
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: hr.Name},
			})
		}
		return reqs
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&helmapi.HelmRelease{}).
		Watches(
			&core.Namespace{},
			handler.EnqueueRequestsFromMapFunc(mapNamespaceToHelmRelease),
			builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
				_, ok := obj.GetAnnotations()[tracker.KeyUid]
				return ok
			}))).
		Complete(r)
}
