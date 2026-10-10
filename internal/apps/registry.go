package apps

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/util"
)

type App struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Name        string            `json:"name"`
	Version     string            `json:"version,omitempty"`
	Directory   string            `json:"directory,omitempty"`
	Port        int               `json:"port,omitempty"`
	Service     string            `json:"service,omitempty"`
	Status      string            `json:"status,omitempty"`
	InstalledAt time.Time         `json:"installed_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type registry struct {
	Apps []App `json:"apps"`
}

func path() string { return filepath.Join(util.StateDir(), "apps.json") }
func List() ([]App, error) {
	var r registry
	if err := util.ReadJSON(path(), &r); err != nil {
		if util.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	sort.Slice(r.Apps, func(i, j int) bool { return r.Apps[i].InstalledAt.Before(r.Apps[j].InstalledAt) })
	return r.Apps, nil
}
func Save(a App) error {
	return update(func(list []App) []App {
		for i := range list {
			if list[i].ID == a.ID {
				list[i] = a
				return list
			}
		}
		return append(list, a)
	})
}

func update(change func([]App) []App) error {
	if err := os.MkdirAll(filepath.Dir(path()), 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(path()+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	list, err := List()
	if err != nil {
		return err
	}
	return util.WriteJSON(path(), registry{change(list)}, 0600)
}
func Get(id string) (App, error) {
	list, err := List()
	if err != nil {
		return App{}, err
	}
	for _, a := range list {
		if a.ID == id || a.Name == id {
			return a, nil
		}
	}
	return App{}, fmt.Errorf("app %q not found", id)
}
func Delete(id string) error {
	return update(func(list []App) []App {
		out := list[:0]
		for _, a := range list {
			if a.ID != id && a.Name != id {
				out = append(out, a)
			}
		}
		return out
	})
}
