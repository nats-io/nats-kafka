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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilePath(t *testing.T) {
	file, err := ioutil.TempFile(os.TempDir(), "prefix")
	require.NoError(t, err)

	path, err := ValidateFilePath(file.Name())
	require.NoError(t, err)
	require.NotEqual(t, "", path)

	_, err = ValidateDirPath(file.Name())
	require.Error(t, err)
}

func TestDirPath(t *testing.T) {
	path, err := ioutil.TempDir(os.TempDir(), "prefix")
	require.NoError(t, err)

	abspath, err := ValidateDirPath(path)
	require.NoError(t, err)
	require.NotEqual(t, "", abspath)

	_, err = ValidateFilePath(path)
	require.Error(t, err)
}

func TestPathDoesntExist(t *testing.T) {
	path, err := ioutil.TempDir(os.TempDir(), "prefix")
	require.NoError(t, err)

	path = filepath.Join(path, "foo")

	_, err = ValidateDirPath(path)
	require.Error(t, err)
}

func TestEmptyPath(t *testing.T) {
	_, err := ValidateFilePath("")
	require.Error(t, err)

	_, err = ValidateDirPath("")
	require.Error(t, err)
}

func TestBadPath(t *testing.T) {
	_, err := ValidateFilePath("//foo\\br//#!90")
	require.Error(t, err)

	_, err = ValidateDirPath("//foo\\br//#!90")
	require.Error(t, err)
}
