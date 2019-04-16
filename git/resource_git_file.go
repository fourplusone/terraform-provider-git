package git

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/filemode"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage"
)

func resourceGitFile() *schema.Resource {
	return &schema.Resource{
		Create: resourceGitFileCreate,
		Update: resourceGitFileCreate,
		Read:   resourceGitFileRead,
		Delete: resourceGitFileDelete,

		Schema: map[string]*schema.Schema{
			"contents": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},

			"path": &schema.Schema{
				Type:     schema.TypeString,
				ForceNew: true,
				Required: true,
			},
		},
	}
}

func dataSourceGitFile() *schema.Resource {
	return &schema.Resource{
		Read: resourceGitFileRead,

		Schema: map[string]*schema.Schema{
			"contents": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"path": &schema.Schema{
				Type:     schema.TypeString,
				ForceNew: true,
				Required: true,
			},
		},
	}
}

func resourceGitFileCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	pushReq, pushRes := config.pushCombiner.Announce()
	defer close(pushReq)

	config.repoLock.Lock()
	err := resourceGitFileCreateNoLock(d, m)
	config.repoLock.Unlock()

	if err != nil {
		return err
	}

	pushReq <- 1

	err, _ = (<-pushRes).(error)
	return err
}

func resourceGitFileRead(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)

	config.repoLock.Lock()
	defer config.repoLock.Unlock()

	return resourceGitFileReadNoLock(d, m)
}

func resourceGitFileDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	pushReq, pushRes := config.pushCombiner.Announce()
	defer close(pushReq)

	config.repoLock.Lock()
	err := resourceGitFileDeleteNoLock(d, m)
	config.repoLock.Unlock()

	if err != nil {
		return err
	}

	pushReq <- 1

	err, _ = (<-pushRes).(error)
	return err
}

func resourceGitFileCreateNoLock(d *schema.ResourceData, m interface{}) error {

	config := m.(*Config)

	raw := []byte(d.Get("contents").(string))

	repo := config.repository

	head, err := repo.Head()
	if err != nil {
		return err
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return err
	}

	tree, err := commit.Tree()
	if err != nil {
		return err
	}

	s := repo.Storer
	obj := s.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	obj.SetSize(int64(len(raw)))

	writer, err := obj.Writer()
	if err != nil {
		return err
	}

	writer.Write(raw)

	s.SetEncodedObject(obj)

	path := d.Get("path").(string)
	hash := obj.Hash()

	outHash, err := writeFileToTree(path, hash, tree, repo.Storer)
	if err != nil {
		return err
	}

	// Committing
	message := fmt.Sprintf("Add/Update %s", path)

	newCommit := &object.Commit{
		Author:       *config.signature,
		Committer:    *config.signature,
		Message:      message,
		TreeHash:     *outHash,
		ParentHashes: []plumbing.Hash{commit.Hash},
	}

	o := s.NewEncodedObject()
	if err := newCommit.Encode(o); err != nil {
		return err
	}
	s.SetEncodedObject(o)

	commitHash := o.Hash()

	// Update HEAD
	headRef, err := s.Reference(plumbing.HEAD)
	if err != nil {
		return err
	}

	name := plumbing.HEAD
	if headRef.Type() != plumbing.HashReference {
		name = headRef.Target()
	}

	ref := plumbing.NewHashReference(name, commitHash)
	err = s.SetReference(ref)
	if err != nil {
		return err
	}
	///////////////

	d.SetId(obj.Hash().String())

	return resourceGitFileReadNoLock(d, m)

}

func resourceGitFileReadNoLock(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	repo := config.repository
	path := d.Get("path").(string)

	head, err := repo.Head()
	if err != nil {
		return err
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return err
	}

	tree, err := commit.Tree()
	if err != nil {
		return err
	}

	file, err := tree.File(path)
	if err == object.ErrFileNotFound {
		d.SetId("")
		return nil
	} else if err != nil {
		return err
	}

	content, err := file.Contents()
	if err != nil {
		return err
	}

	d.Set("content", content)

	return nil
}

func resourceGitFileDeleteNoLock(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	repo := config.repository
	path := d.Get("path").(string)
	s := repo.Storer

	head, err := repo.Head()
	if err != nil {
		return err
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return err
	}

	tree, err := commit.Tree()
	if err != nil {
		return err
	}

	outHash, err := removeFileFromTree(path, tree, repo.Storer)

	// Committing
	message := fmt.Sprintf("Delete %s", path)

	newCommit := &object.Commit{
		Author:       *config.signature,
		Committer:    *config.signature,
		Message:      message,
		TreeHash:     *outHash,
		ParentHashes: []plumbing.Hash{commit.Hash},
	}

	o := s.NewEncodedObject()
	if err := newCommit.Encode(o); err != nil {
		return err
	}
	s.SetEncodedObject(o)

	commitHash := o.Hash()

	// Update HEAD
	headRef, err := s.Reference(plumbing.HEAD)
	if err != nil {
		return err
	}

	name := plumbing.HEAD
	if headRef.Type() != plumbing.HashReference {
		name = headRef.Target()
	}

	ref := plumbing.NewHashReference(name, commitHash)
	err = s.SetReference(ref)
	if err != nil {
		return err
	}
	///////////////

	return nil
}

func writeFileToTree(path string, hash plumbing.Hash, root *object.Tree, s storage.Storer) (*plumbing.Hash, error) {

	var tree = root
	var err error
	pathParts := strings.Split(path, "/")

	for i := len(pathParts) - 1; i >= 0; i-- {
		path := filepath.Join(pathParts[:i]...)
		var currentTree *object.Tree

		if i == 0 {
			currentTree, err = tree, nil
		} else {
			currentTree, err = tree.Tree(path)
		}

		if err == object.ErrDirectoryNotFound {
			currentTree = &object.Tree{}
		} else if err != nil {
			return nil, err
		}

		entry, err := currentTree.FindEntry(pathParts[i])

		var mode = filemode.Dir
		if i == len(pathParts)-1 {
			mode = filemode.Regular
		}

		if err == object.ErrEntryNotFound {
			currentTree.Entries = append(currentTree.Entries, object.TreeEntry{
				Name: pathParts[i],
				Hash: hash,
				Mode: mode,
			})
			sort.Sort(sortableEntries(currentTree.Entries))
		} else if err != nil {
			return nil, err
		} else {
			if entry.Mode != filemode.Regular && i == len(pathParts)-1 {
				return nil, fmt.Errorf("Destiantion is not a file")
			}
			entry.Hash = hash
		}

		o := s.NewEncodedObject()
		if err := currentTree.Encode(o); err != nil {
			return nil, err
		}
		hash = o.Hash()

		_, err = s.SetEncodedObject(o)
		if err != nil {
			return nil, err
		}
	}
	return &hash, nil
}

func removeFileFromTree(path string, root *object.Tree, s storage.Storer) (*plumbing.Hash, error) {

	var tree = root
	var err error
	pathParts := strings.Split(path, "/")

	var hash plumbing.Hash

	for i := len(pathParts) - 1; i >= 0; i-- {
		path := filepath.Join(pathParts[:i]...)

		var currentTree *object.Tree

		if i == 0 {
			currentTree, err = tree, nil
		} else {
			currentTree, err = tree.Tree(path)
		}

		if err == object.ErrDirectoryNotFound {
			currentTree = &object.Tree{}
		} else if err != nil {
			return nil, err
		}

		if i == len(pathParts)-1 {
			var entries []object.TreeEntry
			for _, entry := range currentTree.Entries {
				if entry.Name != pathParts[i] {
					entries = append(entries, entry)
				}
			}

			currentTree.Entries = entries
		} else {
			entry, err := currentTree.FindEntry(pathParts[i])
			if err != nil {
				return nil, err
			}
			entry.Hash = hash
		}

		o := s.NewEncodedObject()
		if err := currentTree.Encode(o); err != nil {
			return nil, err
		}
		hash = o.Hash()

		_, err = s.SetEncodedObject(o)
		if err != nil {
			return nil, err
		}
	}
	return &hash, nil
}

type sortableEntries []object.TreeEntry

func (sortableEntries) sortName(te object.TreeEntry) string {
	if te.Mode == filemode.Dir {
		return te.Name + "/"
	}
	return te.Name
}
func (se sortableEntries) Len() int               { return len(se) }
func (se sortableEntries) Less(i int, j int) bool { return se.sortName(se[i]) < se.sortName(se[j]) }
func (se sortableEntries) Swap(i int, j int)      { se[i], se[j] = se[j], se[i] }
