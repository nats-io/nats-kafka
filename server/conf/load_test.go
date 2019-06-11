/*
 * Copyright 2019 The NATS Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package conf

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type SimpleConf struct {
	Name    string
	Age     int64
	OptOut  bool
	Balance float64
}

func TestLoadFromString(t *testing.T) {
	configString := `
	Name: "stephen"
	Age: 28
	OptOut: true
	Balance: 5.5
	`

	config := SimpleConf{}

	err := LoadConfigFromString(configString, &config, false)
	require.NoError(t, err)
	require.Equal(t, "stephen", config.Name)
	require.Equal(t, int64(28), config.Age)
	require.Equal(t, true, config.OptOut)
	require.Equal(t, 5.5, config.Balance)
}

func TestLoadFromFile(t *testing.T) {
	file, err := ioutil.TempFile(os.TempDir(), "prefix")
	require.NoError(t, err)

	configString := `
	Name: "stephen"
	Age: 28
	OptOut: true
	Balance: 5.5
	`

	fullPath, err := ValidateFilePath(file.Name())
	require.NoError(t, err)

	err = ioutil.WriteFile(fullPath, []byte(configString), 0644)
	require.NoError(t, err)

	config := SimpleConf{}

	err = LoadConfigFromFile(fullPath, &config, false)
	require.NoError(t, err)
	require.Equal(t, "stephen", config.Name)
	require.Equal(t, int64(28), config.Age)
	require.Equal(t, true, config.OptOut)
	require.Equal(t, 5.5, config.Balance)
}

func TestLoadFromMissingFile(t *testing.T) {
	config := SimpleConf{}
	err := LoadConfigFromFile("/foo/bar/baz", &config, false)
	require.Error(t, err)
}

type MapConf struct {
	One map[string]interface{}
	Two map[string]interface{}
}

func TestLoadFromMap(t *testing.T) {
	configString := `
	One: {
	Name: "stephen"
	Age: 28
	OptOut: true
	Balance: 5.5
	}, Two: {
	Name: "zero"
	Age: 32
	OptOut: false
	Balance: 7.7
	}
	`

	config := MapConf{}

	err := LoadConfigFromString(configString, &config, false)
	require.NoError(t, err)

	one := SimpleConf{}
	two := SimpleConf{}

	err = LoadConfigFromMap(config.One, &one, false)
	require.NoError(t, err)
	require.Equal(t, "stephen", one.Name)
	require.Equal(t, int64(28), one.Age)
	require.Equal(t, true, one.OptOut)
	require.Equal(t, 5.5, one.Balance)

	err = LoadConfigFromMap(config.Two, &two, false)
	require.NoError(t, err)
	require.Equal(t, "zero", two.Name)
	require.Equal(t, int64(32), two.Age)
	require.Equal(t, false, two.OptOut)
	require.Equal(t, 7.7, two.Balance)
}
