package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type NscReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *NscReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Reconciling Namespace", "namespace", req.NamespacedName.Name)

	var namespace corev1.Namespace
	if err := r.Get(ctx, req.NamespacedName, &namespace); err != nil {
		logger.Error(err, "Unable to fetch Namespace")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if r.isExcludedNamespace(req.NamespacedName.Name) {
		logger.Info("Skipping excluded namespace", "namespace", req.NamespacedName.Name)
		return ctrl.Result{}, nil
	}

	if err := r.reconcileHTTPRoute(ctx, &namespace); err != nil {
		logger.Error(err, "Failed to reconcile HTTPRoute")
		return ctrl.Result{}, err
	}

	if err := r.reconcileReferenceGrant(ctx, &namespace); err != nil {
		logger.Error(err, "Failed to reconcile ReferenceGrant")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *NscReconciler) isExcludedNamespace(name string) bool {
	excludedNamespaces := []string{"kube-system", "default", "kube-public", "monitoring"}
	for _, ns := range excludedNamespaces {
		if name == ns {
			return true
		}
	}
	return false
}

func (r *NscReconciler) reconcileHTTPRoute(ctx context.Context, namespace *corev1.Namespace) error {
	httpRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      fmt.Sprintf("%s-admin-route", namespace.Name),
			Namespace: "kong-system",
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Kind:      (*gatewayv1.Kind)(ptr.To("Gateway")),
						Name:      gatewayv1.ObjectName("admin-gateway"),
						Namespace: (*gatewayv1.Namespace)(ptr.To("kong-system")),
					},
				},
			},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Value: ptr.To(fmt.Sprintf("/%s", namespace.Name)),
							},
						},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name:      gatewayv1.ObjectName(fmt.Sprintf("%s-gateway-admin", namespace.Name)),
									Port:      ptr.To(gatewayv1.PortNumber(8444)),
									Namespace: (*gatewayv1.Namespace)(ptr.To(namespace.Name)),
								},
							},
						},
					},
					Filters: []gatewayv1.HTTPRouteFilter{
						{
							Type: gatewayv1.HTTPRouteFilterURLRewrite,
							URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
								Path: &gatewayv1.HTTPPathModifier{
									Type:               gatewayv1.PrefixMatchHTTPPathModifier,
									ReplacePrefixMatch: ptr.To("/"),
								},
							},
						},
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(namespace, httpRoute, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, httpRoute, func() error {
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create or update HTTPRoute: %w", err)
	}

	return nil
}

func (r *NscReconciler) reconcileReferenceGrant(ctx context.Context, namespace *corev1.Namespace) error {
	referenceGrant := &gatewayv1beta1.ReferenceGrant{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      "allow-kong-system-routes",
			Namespace: namespace.Name,
		},
		Spec: gatewayv1beta1.ReferenceGrantSpec{
			From: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group("gateway.networking.k8s.io"),
					Kind:      gatewayv1beta1.Kind("HTTPRoute"),
					Namespace: gatewayv1beta1.Namespace("kong-system"),
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

	if err := ctrl.SetControllerReference(namespace, referenceGrant, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, referenceGrant, func() error {
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create or update ReferenceGrant: %w", err)
	}

	return nil
}

func (r *NscReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Named("namespace-controller").
		Complete(r)
}
