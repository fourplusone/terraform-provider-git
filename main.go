package main

import (
	terraformGit "github.com/fourplusone/terraform-provider-git/git"
	"github.com/hashicorp/terraform/plugin"
	"github.com/hashicorp/terraform/terraform"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: func() terraform.ResourceProvider {
			return terraformGit.Provider()
		},
	})

}
