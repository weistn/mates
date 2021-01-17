package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kylelemons/go-gypsy/yaml"
	"github.com/spf13/afero"
)

/**************************************************
*
* YAML
*
***************************************************/

func loadYamlFile(fs afero.Fs, name string) (map[string]interface{}, error) {
	_, err := fs.Stat(name)
	if err != nil {
		// The file seems to not exist
		return nil, nil
	}
	data, err := afero.ReadFile(fs, name)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(data)
	node, err := yaml.Parse(reader)
	if err != nil {
		return nil, fmt.Errorf("Error loading %v: %v", name, err)
	}
	if node != nil {
		if nodeMap, ok := node.(yaml.Map); ok {
			return yamlToMap(nodeMap), nil
		}
	}
	return nil, os.ErrNotExist
}

func yamlToMap(y yaml.Map) map[string]interface{} {
	result := make(map[string]interface{})
	for k, yv := range y {
		switch v := yv.(type) {
		case yaml.Map:
			result[k] = yamlToMap(v)
		case yaml.List:
			result[k] = yamlToList(v)
		case yaml.Scalar:
			result[k] = string(v)
		}
	}
	return result
}

func yamlToList(y yaml.List) []interface{} {
	result := make([]interface{}, 0, len(y))
	for _, e := range y {
		switch v := e.(type) {
		case yaml.Map:
			result = append(result, yamlToMap(v))
		case yaml.List:
			result = append(result, yamlToList(v))
		case yaml.Scalar:
			result = append(result, string(v))
		}
	}
	return result
}

func yamlMap(k string, v interface{}, filename string) (map[string]interface{}, error) {
	if m, ok := v.(map[string]interface{}); ok {
		return m, nil
	}
	return nil, fmt.Errorf("Expected attribute "+k+" to be a map in file %v", filename)
}

func yamlString(k string, v interface{}, filename string) (string, error) {
	if str, ok := v.(string); ok {
		return str, nil
	}
	return "", fmt.Errorf("Expected attribute "+k+" to be a string in file %v", filename)
}

func yamlStrings(k string, v interface{}, filename string) ([]string, error) {
	var result []string
	if list, ok := v.([]interface{}); ok {
		for _, e := range list {
			if str, ok := e.(string); ok {
				result = append(result, str)
			} else {
				return nil, fmt.Errorf("Expected attribute "+k+" to be a list of strings in file %v", filename)
			}
		}
	} else {
		return nil, fmt.Errorf("Expected attribute "+k+" to be a list of strings in file %v", filename)
	}
	return result, nil
}

func yamlStringOrStrings(k string, v interface{}, filename string) ([]string, error) {
	str, err := yamlString(k, v, filename)
	if err == nil {
		return []string{str}, nil
	}
	return yamlStrings(k, v, filename)
}

/**************************************************
*
* IO
*
***************************************************/

// CopyFile copies a file from one Fs to another Fs.
// It creates all necessary directories on the way.
func copyFile(fromFs afero.Fs, from string, toFs afero.Fs, to string) (err error) {
	dir, _ := filepath.Split(to)
	ospath := filepath.FromSlash(dir)

	if ospath != "" {
		err = toFs.MkdirAll(ospath, 0775)
		if err != nil {
			if err != os.ErrExist {
				return err
			}
		}
	}

	fromFile, err := fromFs.Open(from)
	if err != nil {
		return err
	}
	defer fromFile.Close()
	toFile, err := toFs.Create(to)
	if err != nil {
		return err
	}
	defer toFile.Close()

	_, err = io.Copy(toFile, fromFile)
	return err
}

func stripSuffix(filename string) string {
	i := strings.LastIndex(filename, ".")
	if i != -1 {
		return filename[:i]
	}
	return filename
}

/**************************************************
*
* Resources
*
***************************************************/

func resolveContentResource(b *Builder, res *Resource) error {
	if res.Resolved {
		return nil
	}
	res.Resolved = true
	if res.URL.IsAbs() {
		return nil
	}
	// TODO: Convert URL Path to OS-specific filesystem path.
	p := filepath.Clean(filepath.Join(string(filepath.Separator), res.URL.Path))
	res.URL.Path = filepath.ToSlash(p)
	res.SourcePath = p
	res.SourceFs = b.contentFs
	// TODO: Convert URL Path to OS-specific filesystem path.
	res.DestPath = res.URL.Path
	// println("CONTENT", res.SourcePath, res.DestPath, res.URL.Path)
	return nil
}
