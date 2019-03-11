# terraform-provider-git

[![Build Status](https://travis-ci.com/fourplusone/terraform-provider-git.svg?token=Uozu8nkCaipWW1259XXD&branch=master)](https://travis-ci.com/fourplusone/terraform-provider-git)

## Rationale

Keep your git repositories in sync with your infrastructure. 

## Configuration

- repository_url - (Required) The URL of the remote repository
- author_name - (Optional) Name of the committer
- author_email - (Optional) Email of the committer

### Configuration Example

```hcl
provider "git" {
    repository_url = "git@github.com:fourplusone/tf-target.git"
    author_name = "Matthias Bartelmeß - Terraform"
    author_email = "mba@fourplusone.de"
}
```

## Resource

The following arguments are supported:

- content - (Required) The content of file to create.

- filename - (Required) The path of the file to create.

Any required parent directories will be created automatically, and any existing file with the given name will be overwritten.

### Resource Example

```hcl
resource "git_file" "demo_out_1" {
  contents = "hello"
  path = "hello/world.txt"
}
```
