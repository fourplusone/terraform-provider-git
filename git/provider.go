package git

import (
	"context"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/fourplusone/go-batches"
	"github.com/hashicorp/terraform/helper/schema"
	gogit "gopkg.in/src-d/go-git.v4"

	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// Config Git Provider Configuration
type Config struct {
	cloneDir     string
	repoLock     sync.Mutex
	pushCombiner batches.Combiner
	repository   *gogit.Repository
	signature    *object.Signature
}

// Provider returns the Git Provider
func Provider(ctx context.Context, wg *sync.WaitGroup) *schema.Provider {
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
	p.ConfigureFunc = configureProviderFunc(ctx, &p, wg)

	return &p
}

func configureProviderFunc(ctx context.Context, p *schema.Provider, wg *sync.WaitGroup) schema.ConfigureFunc {
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

		config.pushCombiner = batches.Combiner{
			CombineFunc: func([]batches.In) batches.Out {
				config.repoLock.Lock()
				defer config.repoLock.Unlock()

				err := config.repository.Push(&gogit.PushOptions{})
				if err == gogit.NoErrAlreadyUpToDate {
					err = nil
				}

				return err
			},
			Input: make(chan batches.Item),
		}

		go func() {
			// Start the Combiner
			config.pushCombiner.Process()

			// Add self to waitgroup to prevent program from terminating
			wg.Add(1)

			// Wait for context to be finished
			<-ctx.Done()

			os.RemoveAll(cloneDir)
			config.pushCombiner.Close()

			wg.Done()
		}()

		return config, nil
	}
}
