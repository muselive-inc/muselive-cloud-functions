package main

import (
	"context"
	"log"
	"os"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	"github.com/muselive-cloud-functions"
)
func main() {
	ctx := context.Background()
	// Use PORT environment variable, or default to 8080.
	// Cloud functions can be registered only one at a time for a given url.
	if err := funcframework.RegisterHTTPFunctionContext(
		ctx, "/my-show", functions.SendMyShowNoti); err != nil {
		log.Fatalf("funcframework.RegisterHTTPFunctionContext: %v\n", err)
	}

	// if err := funcframework.RegisterHTTPFunctionContext(
	// 	ctx, "/send-to-followers", functions.SendToFollowers); err != nil {
	// 	log.Fatalf("funcframework.RegisterHTTPFunctionContext: %v\n", err)
	// }

	port := "8080"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}
	if err := funcframework.Start(port); err != nil {
		log.Fatalf("funcframework.Start: %v\n", err)
	}
}
