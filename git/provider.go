package git

import (
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/terraform/helper/schema"
	gogit "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type Config struct {
	cloneDir   string
	repoLock   sync.Mutex
	repository *gogit.Repository
	signature  *object.Signature
}

func Provider() *schema.Provider {
	p := schema.Provider{
		DataSourcesMap: map[string]*schema.Resource{
			"git_file": dataSourceGitFile(),
		},
		ResourcesMap: map[string]*schema.Resource{
			"git_file": resourceGitFile(),
		},
		Schema: map[string]*schema.Schema{
			"repository_url": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("GIT_REPOSITORY_URL", nil),
			},
			"author_name": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "Terraform Git Provider",
			},
			"author_email": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "tf@fourplusone.de",
			},
		},
	}

	p.ConfigureFunc = configureProviderFunc(&p)
	return &p
}

func configureProviderFunc(p *schema.Provider) schema.ConfigureFunc {
	return func(r *schema.ResourceData) (interface{}, error) {
		options := &gogit.CloneOptions{
			URL:          r.Get("repository_url").(string),
			Progress:     os.Stdout,
			SingleBranch: true,
			NoCheckout:   true,
		}

		cloneDir, err := ioutil.TempDir("", "terraform-git")

		if err != nil {
			return nil, err
		}

		repo, err := gogit.PlainClone(cloneDir, false, options)
		if err != nil {
			return nil, err
		}

		config := &Config{
			cloneDir:   cloneDir,
			repository: repo,
			signature: &object.Signature{
				Name:  r.Get("author_name").(string),
				Email: r.Get("author_email").(string),
				When:  time.Now(),
			},
		}

		return config, nil
	}
}
