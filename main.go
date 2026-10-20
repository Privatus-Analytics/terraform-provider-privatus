// Command terraform-provider-privatus is the Terraform provider for
// Privatus Analytics (https://privatusanalytics.com).
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/privatus-analytics/terraform-provider-privatus/internal/provider"
)

// version is set by goreleaser (-ldflags "-X main.version=…").
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/privatus-analytics/privatus",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
