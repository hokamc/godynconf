package godynconf_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hokamc/godynconf"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

type TestYaml struct {
	Name   string `yaml:"name"`
	Age    int    `yaml:"age"`
	Height int    `yaml:"height"`
}

func TestConf(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.yaml")

	nty := &TestYaml{
		Age:    99,
		Height: 280,
		Name:   "hello word",
	}
	writeYaml(t, nty, p)

	cw := godynconf.NewConfWatcher(godynconf.WithLog())
	tc := godynconf.NewConf[TestYaml](p)
	cw.Add(tc)

	assert.NotNil(t, tc.Get())
	assert.Equal(t, 99, tc.Get().Age)
	assert.Equal(t, 280, tc.Get().Height)
	assert.Equal(t, "hello word", tc.Get().Name)
	assert.NoError(t, cw.Start())

	nty = &TestYaml{
		Age:    98,
		Height: 281,
		Name:   "hello word",
	}
	writeYaml(t, nty, p)
	time.Sleep(10 * time.Millisecond)

	assert.NotNil(t, tc.Get())
	assert.Equal(t, 98, tc.Get().Age)
	assert.Equal(t, 281, tc.Get().Height)
	assert.Equal(t, "hello word", tc.Get().Name)
	assert.NoError(t, cw.Close())
}

func writeYaml(t *testing.T, nty *TestYaml, p string) {
	out, err := yaml.Marshal(nty)
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(p, out, 0644))
}
