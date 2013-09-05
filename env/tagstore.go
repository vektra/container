package env

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/arch-reactor/container/utils"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

const DEFAULTTAG = "latest"

type Entries map[string]*Image

type Repository map[string]string

type TagStore struct {
	Path         string
	Entries      Entries
	Repositories map[string]Repository
}

func (i *TagStore) CopyTo(o *TagStore, id string, clobber bool) {
	for name, r := range i.Repositories {
		for tag, hash := range r {
			if hash == id {
				if o.Repositories == nil {
					o.Repositories = make(map[string]Repository)
				}

				if o.Repositories[name] == nil {
					o.Repositories[name] = make(Repository)
				}

				if !clobber {
					_, ok := o.Repositories[name][tag]

					if !ok {
						o.Repositories[name][tag] = hash
					}
				} else {
					o.Repositories[name][tag] = hash
				}
			}
		}
	}
}

func (store *TagStore) Lookup(name string) (string, error) {
	repoName, tag := ParseRepositoryTag(name)
	if tag == "" {
		tag = DEFAULTTAG
	}

	tags, ok := store.Repositories[repoName]

	if !ok {
		return "", errors.New(fmt.Sprintf("No repo named '%s'", repoName))
	}

	id, ok := tags[tag]

	if !ok {
		return "", errors.New(fmt.Sprintf("No tag named '%s' in repo '%s'", tag, repoName))
	}

	return id, nil
}

func (store *TagStore) LookupImage(name string) (*Image, error) {
	repoName, tag := ParseRepositoryTag(name)
	if tag == "" {
		tag = DEFAULTTAG
	}

	tags, ok := store.Repositories[repoName]

	if !ok {
		return nil, fmt.Errorf("No repo")
	}

	id, ok := tags[tag]

	if !ok {
		return nil, fmt.Errorf("No repo")
	}

	img, ok := store.Entries[id]

	if !ok {
		return nil, fmt.Errorf("Image not on disk")
	}

	return img, nil
}

func (ts *TagStore) Add(repo, tag, hash string) {
	if ts.Repositories[repo] == nil {
		ts.Repositories[repo] = make(Repository)
	}

	ts.Repositories[repo][tag] = hash
}

func (ts *TagStore) Find(id string) (repo string, tag string) {
	for repo, tags := range ts.Repositories {
		for tag, hash := range tags {
			if id == hash {
				return repo, tag
			}
		}
	}

	return "", ""
}

func (ts *TagStore) Remove(id string) bool {
	deleted := false
	for repo, tags := range ts.Repositories {
		for tag, hash := range tags {
			if id == hash {
				delete(tags, tag)
				deleted = true
			}
		}

		if len(tags) == 0 {
			delete(ts.Repositories, repo)
		}
	}

	return deleted
}

func (ts *TagStore) RemoveByPrefix(id string) bool {
	deleted := false
	for repo, tags := range ts.Repositories {
		for tag, hash := range tags {
			if strings.HasPrefix(hash, id) {
				delete(tags, tag)
				deleted = true
			}
		}

		if len(tags) == 0 {
			delete(ts.Repositories, repo)
		}
	}

	return deleted
}

func (ts *TagStore) RemoveTag(repo, tag string) bool {
	tags, ok := ts.Repositories[repo]

	if !ok {
		return false
	}

	delete(tags, tag)

	if len(tags) == 0 {
		delete(ts.Repositories, repo)
	}

	return true
}

func (ts *TagStore) Flush() error {
	if ts.Path == "" {
		return errors.New("No path set on tag store to flush")
	}

	data, err := json.Marshal(ts)

	if err != nil {
		return err
	}

	ioutil.WriteFile(ts.Path, data, 0644)

	return nil
}

// Get a repos name and returns the right reposName + tag
// The tag can be confusing because of a port in a repository name.
//     Ex: localhost.localdomain:5000/samalba/hipache:latest
func ParseRepositoryTag(repos string) (string, string) {
	n := strings.LastIndex(repos, ":")
	if n < 0 {
		return repos, DEFAULTTAG
	}
	if tag := repos[n+1:]; !strings.Contains(tag, "/") {
		return repos[:n], tag
	}
	return repos, DEFAULTTAG
}

func ReadRepoFile(path string) (*TagStore, error) {
	data, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, err
	}

	tags := &TagStore{Path: path}

	err = json.Unmarshal(data, &tags)

	if err != nil {
		return nil, err
	}

	return tags, nil
}

func LoadTagStore(root string) (*TagStore, error) {
	tags, err := ReadRepoFile(path.Join(root, "repositories"))

	if err != nil {
		return nil, err
	}

	dir, err := ioutil.ReadDir(path.Join(root, "graph"))

	if err != nil {
		return nil, err
	}

	tags.Entries = make(Entries)

	for _, ent := range dir {
		id := ent.Name()

		img := &Image{}
		img.ID = id

		data, err := ioutil.ReadFile(path.Join(root, "graph", id, "json"))

		if err != nil {
			continue
		}

		err = json.Unmarshal(data, &img)

		if err != nil {
			return nil, err
		}

		tags.Entries[id] = img
	}

	for _, img := range tags.Entries {
		if img.Parent != "" {
			par, ok := tags.Entries[img.Parent]
			if ok {
				img.parentImage = par
			} else {
				fmt.Fprintf(os.Stderr, "Unable to find parent image %s\n",
					utils.TruncateID(img.Parent))
			}
		}
	}

	return tags, nil
}

func DefaultTagStore() (*TagStore, error) {
	return LoadTagStore(DIR)
}
