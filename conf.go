package godynconf

import (
	"fmt"
	"log"
	"os"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

type ConfWatcher struct {
	w  *fsnotify.Watcher
	cm map[string]IConf
	hl bool
}

type Conf[T any] struct {
	p  string
	vp *atomic.Pointer[*T]
}

type IConf interface {
	Path() string
	Reload() error
	ToString() string
}

// --- ConfWatcher
func NewConfWatcher(opts ...func(*ConfWatcher)) *ConfWatcher {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalln(err)
	}
	cw := &ConfWatcher{
		w:  w,
		cm: make(map[string]IConf),
		hl: false,
	}
	for _, v := range opts {
		v(cw)
	}
	return cw
}

func WithLog() func(*ConfWatcher) {
	return func(cw *ConfWatcher) {
		cw.hl = true
	}
}

func (cw *ConfWatcher) Start() error {
	go func() {
		for {
			select {
			case event, ok := <-cw.w.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					if c, ok := cw.cm[event.Name]; ok {
						err := c.Reload()
						if err != nil {
							log.Println("godynconf fail to reload due to reading file", err)
						}
						if cw.hl {
							log.Println("godynconf reload conf", c.ToString())
						}
					}
				}
			case err, ok := <-cw.w.Errors:
				if !ok {
					return
				}
				log.Println("godynconf fnotify has error", err)
			}
		}
	}()
	return nil
}

func (cw *ConfWatcher) Close() error {
	return cw.w.Close()
}

func (cw *ConfWatcher) Add(c IConf) {
	err := c.Reload()
	if err != nil {
		log.Fatalln("godynconf fail to add", err)
	}
	err = cw.w.Add(c.Path())
	if err != nil {
		log.Fatalln("godynconf fail to add", err)
	}
	cw.cm[c.Path()] = c
}

// --- Conf
func NewConf[T any](path string) *Conf[T] {
	return &Conf[T]{
		p:  path,
		vp: &atomic.Pointer[*T]{},
	}
}

func (c *Conf[T]) Reload() error {
	bs, err := os.ReadFile(c.p)
	if err != nil {
		return err
	}
	r := new(T)
	err = yaml.Unmarshal(bs, r)
	if err != nil {
		return err
	}
	c.vp.Store(&r)
	return nil
}

func (c *Conf[T]) Path() string {
	return c.p
}

func (c *Conf[T]) ToString() string {
	return fmt.Sprintf("Conf, file: %s, value: %+v", c.p, c.Get())
}

func (c *Conf[T]) Get() *T {
	return *c.vp.Load()
}
