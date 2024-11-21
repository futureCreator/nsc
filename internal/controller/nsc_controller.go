package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NscReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *NscReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	fmt.Printf("Namespace created: %s\n", req.NamespacedName.Name)

	var namespace corev1.Namespace
	if err := r.Get(ctx, req.NamespacedName, &namespace); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Create ReferenceGrant
	referenceGrant := &gatewayv1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-kong-system-routes",
			Namespace: req.NamespacedName.Name,
		},
		Spec: gatewayv1beta1.ReferenceGrantSpec{
			From: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group("gateway.networking.k8s.io"),
					Kind:      gatewayv1beta1.Kind("HTTPRoute"),
					Namespace: "kong-system",
				},
			},
			To: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(""),
					Kind:  gatewayv1beta1.Kind("Service"),
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(&namespace, referenceGrant, r.Scheme); err != nil {
        return ctrl.Result{}, err
    }

	// fmt.Printf("HTTPRoute: %+v\n", httpRoute)
	fmt.Printf("ReferenceGrant: %+v\n", referenceGrant)

    if err := r.Create(ctx, referenceGrant); err != nil {
	fmt.Printf("Failed to create ReferenceGrant...: %v", err)
        return ctrl.Result{}, err
    }

	return ctrl.Result{}, nil
}

func (r *NscReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Named("namespace-controller").
		Complete(r)
}
