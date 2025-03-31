//go:build tools
// +build tools

// This package contains import references to packages required only for the
// build process.

// https://go.dev/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

// package tools

// import (
// 	_ "github.com/elastic/crd-ref-docs"
// 	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
// 	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
// 	_ "k8s.io/code-generator/cmd/client-gen"
// 	_ "k8s.io/code-generator/cmd/deepcopy-gen"
// 	_ "k8s.io/code-generator/cmd/informer-gen"
// 	_ "k8s.io/code-generator/cmd/lister-gen"
// 	_ "k8s.io/code-generator/cmd/register-gen"
// 	_ "k8s.io/kube-openapi/cmd/openapi-gen"
// 	_ "sigs.k8s.io/controller-runtime/pkg/scheme"
// 	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
// )
