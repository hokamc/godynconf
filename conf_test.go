package godynconf_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hokamc/godynconf"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type TestYaml struct {
	Name   string `yaml:"name"`
	Age    int    `yaml:"age"`
	Height int    `yaml:"height"`
}

type TestTransform struct {
	NameByAge map[string]int
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
	tfc := godynconf.NewTfConf(tc, func(t *TestYaml) *TestTransform {
		return &TestTransform{
			NameByAge: map[string]int{
				t.Name: t.Age,
			},
		}
	})

	cw.Add(tc)

	require.NotNil(t, tc.Get())
	require.Equal(t, 99, tc.Get().Age)
	require.Equal(t, 280, tc.Get().Height)
	require.Equal(t, "hello word", tc.Get().Name)
	require.Equal(t, 99, tfc.Get().NameByAge["hello word"])
	require.NoError(t, cw.Start())

	nty = &TestYaml{
		Age:    98,
		Height: 281,
		Name:   "hello word",
	}
	writeYaml(t, nty, p)
	time.Sleep(10 * time.Millisecond)

	require.NotNil(t, tc.Get())
	require.Equal(t, 98, tc.Get().Age)
	require.Equal(t, 281, tc.Get().Height)
	require.Equal(t, "hello word", tc.Get().Name)
	require.Equal(t, 98, tfc.Get().NameByAge["hello word"])
	require.NoError(t, cw.Close())
}

func writeYaml(t *testing.T, nty *TestYaml, p string) {
	out, err := yaml.Marshal(nty)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(p, out, 0644))
}
