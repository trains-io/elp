/*
Copyright 2026 trains-io.
*/

// Package v1alpha1 contains API Schema definitions for the z21 v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=z21.trains.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupVersion = schema.GroupVersion{Group: "z21.trains.io", Version: "v1alpha1"}

	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	AddToScheme = SchemeBuilder.AddToScheme
)
