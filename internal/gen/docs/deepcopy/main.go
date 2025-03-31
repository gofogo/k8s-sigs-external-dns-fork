package main

import (
	// "k8s.io/gengo/args"
	// "k8s.io/gengo/examples/deepcopy-gen/generators"
	//
	// "github.com/spf13/pflag"

	"os"

	"k8s.io/klog/v2"
)

// https://github.com/kubernetes/gengo/blob/1244d31929d7/examples/deepcopy-gen/main.go

func main() {
	klog.InitFlags(nil)
	klog.SetOutput(os.Stdout)
	// arguments := args.Default()
	//
	// // Override defaults.
	// arguments.OutputFileBaseName = "deepcopy_generated"
	//
	// // Custom args.
	// customArgs := &generators.CustomArgs{}
	// pflag.CommandLine.StringSliceVar(&customArgs.BoundingDirs, "bounding-dirs", customArgs.BoundingDirs,
	// 	"Comma-separated list of import paths which bound the types for which deep-copies will be generated.")
	// arguments.CustomArgs = customArgs
	//
	// // Run it.
	// if err := arguments.Execute(
	// 	generators.NameSystems(),
	// 	generators.DefaultNameSystem(),
	// 	generators.Packages,
	// ); err != nil {
	// 	klog.Fatalf("Error: %v", err)
	// }
	// klog.V(2).Info("Completed successfully.")
}
