package godynconf

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

const ENCRYPT_PREFIX = "{encrypted}"

type ConfWatcher struct {
	w      *fsnotify.Watcher
	cm     map[string]IConf
	hl     bool
	aesKey []byte
	iv     []byte
}

type Conf[T any] struct {
	p  string
	vp *atomic.Pointer[*T]
	tf []IRConf
}

type TfConf[T, U any] struct {
	c  *Conf[T]
	tf func(*T) *U
	vp *atomic.Pointer[*U]
}

type IConf interface {
	IRConf
	Path() string
	ToString() string
}

type IRConf interface {
	Reload(cw *ConfWatcher) error
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
		if v != nil {
			v(cw)
		}
	}
	return cw
}

func WithLog() func(*ConfWatcher) {
	return func(cw *ConfWatcher) {
		cw.hl = true
	}
}

func WithEncrypt(aesKeyHex, ivHex string) func(*ConfWatcher) {
	if aesKeyHex == "" || ivHex == "" {
		return nil
	}

	aesKey, err := hex.DecodeString(aesKeyHex)
	if err != nil {
		log.Fatalln(err)
	}

	iv, err := hex.DecodeString(ivHex)
	if err != nil {
		log.Fatalln(err)
	}

	return func(cw *ConfWatcher) {
		cw.aesKey = aesKey
		cw.iv = iv
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
						err := c.Reload(cw)
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
	err := c.Reload(cw)
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
		tf: make([]IRConf, 0),
	}
}

func (c *Conf[T]) Reload(cw *ConfWatcher) error {
	bs, err := os.ReadFile(c.p)
	if err != nil {
		return err
	}

	r := new(T)
	if cw.aesKey != nil && cw.iv != nil {
		var data map[interface{}]interface{}
		err = yaml.Unmarshal(bs, &data)
		if err != nil {
			return err
		}
		for key, value := range data {
			strValue, ok := value.(string)
			if ok && strings.HasPrefix(strValue, ENCRYPT_PREFIX) {
				out, err := decryptAES256CBC(strings.TrimPrefix(strValue, ENCRYPT_PREFIX), cw.aesKey, cw.iv)
				if err != nil {
					return err
				}
				data[key] = out
			}
		}
		bs, err = yaml.Marshal(data)
		if err != nil {
			return err
		}
	}

	if err := yaml.Unmarshal(bs, r); err != nil {
		return err
	}

	c.vp.Store(&r)
	for _, tfc := range c.tf {
		tfc.Reload(cw)
	}
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

// --- TfConf
func NewTfConf[T, U any](c *Conf[T], tf func(*T) *U) *TfConf[T, U] {
	tfc := &TfConf[T, U]{
		c:  c,
		tf: tf,
		vp: &atomic.Pointer[*U]{},
	}
	c.tf = append(c.tf, tfc)
	return tfc
}

func (tfc *TfConf[T, U]) Reload(cw *ConfWatcher) error {
	r := tfc.tf(tfc.c.Get())
	tfc.vp.Store(&r)
	return nil
}

func (tfc *TfConf[T, U]) Get() *U {
	return *tfc.vp.Load()
}

// --- AES
func decryptAES256CBC(encryptedBase64 string, aesKey []byte, iv []byte) (string, error) {
	encryptedData, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plainText := make([]byte, len(encryptedData))
	mode.CryptBlocks(plainText, encryptedData)
	return string(pKCS7PaddingRemove(plainText)), nil
}

func pKCS7PaddingRemove(data []byte) []byte {
	padding := int(data[len(data)-1])
	return data[:len(data)-padding]
}
