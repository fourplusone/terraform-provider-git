provider "git" {
    repository_url = "git@github.com:fourplusone/tf-target.git"
}

resource "git_file" "demo_out_1" {
  contents = "hello"
  path = "hello/world.txt"
}

resource "git_file" "demo_out_2" {
  contents = "Hello Github"
  path = "hello/world2.txt"
}


