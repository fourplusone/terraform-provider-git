package main

import (
	"context"
	"sync"

	terraformGit "github.com/fourplusone/terraform-provider-git/git"
	"github.com/hashicorp/terraform/plugin"
	"github.com/hashicorp/terraform/terraform"
)

func main() {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: func() terraform.ResourceProvider {
			return terraformGit.Provider(ctx, &wg)
		},
	})

	// Cancel the Plugin Context
	cancel()

	// Wait until all providers cleaned up their copies
	wg.Wait()

}
