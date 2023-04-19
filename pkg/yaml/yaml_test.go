package yaml

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

type EmptyYamlConfig struct {
}

type SimpleYamlConfig struct {
	Value string `json:"value"`
}

func TestReadYamlFile(t *testing.T) {
	// Setup variables
	existingEmptyYamlFileName := "file_existing_empty.yaml"
	var existingEmptyYamlConf EmptyYamlConfig
	nonExistingEmptyYamlFileName := "non_existing_empty.yaml"
	var nonExistingEmptyYamlConf EmptyYamlConfig

	// Setup temporary file
	path := filepath.Join(t.TempDir(), existingEmptyYamlFileName)
	os.WriteFile(path, nil, 0600)

	//Read existing empty yaml file
	existingEmptyYamlErr := ReadYamlFile(path, &existingEmptyYamlConf)
	assert.NoError(t, existingEmptyYamlErr, "Can't read empty yaml file: %s", path)

	//Read non-existing empty yaml file
	nonExistingEmptyYamlErr := ReadYamlFile(nonExistingEmptyYamlFileName, &nonExistingEmptyYamlConf)
	assert.Error(t, nonExistingEmptyYamlErr, "Should throw an error because %s doesn't exist", nonExistingEmptyYamlFileName)
}

func TestReadYamlAllFile(t *testing.T) {
	// Setup variables
	twoDocsYamlContent := `value: anyValue1
---
value: anyValue2
`
	expectedTwoDocsYaml := []any{
		map[string]any{
			"value": "anyValue1",
		},
		map[string]any{
			"value": "anyValue2",
		},
	}
	existingEmptyYamlFileName := "file_existing_empty.yaml"
	existingTwoDocsYamlFileName := "file_existing_two_docs.yaml"
	nonExistingEmptyYamlFileName := "non_existing_empty.yaml"

	// Setup temporary file
	path := filepath.Join(t.TempDir(), existingEmptyYamlFileName)
	os.WriteFile(path, nil, 0600)

	//Read existing empty yaml file
	existingEmptyYamlAllFileResult, existingEmptyYamlAllFileErr := ReadYamlAllFile(path)
	assert.NoError(t, existingEmptyYamlAllFileErr, "Can't read empty yaml file: %s", path)
	assert.Empty(t, existingEmptyYamlAllFileResult, "Empty YAML stream read incorrectly. Value should be empty")

	//Read existing empty yaml file with two documents
	path = filepath.Join(t.TempDir(), existingTwoDocsYamlFileName)
	os.WriteFile(path, []byte(twoDocsYamlContent), 0600)

	twoDocsYamlAllFileResult, twoDocsYamlAllFileErr := ReadYamlAllFile(path)
	assert.NoError(t, twoDocsYamlAllFileErr, "Can't read yaml file: %s", path)
	assert.Equal(t, expectedTwoDocsYaml, twoDocsYamlAllFileResult)

	//Read non-existing empty yaml file
	nonExistingEmptyYamlAllFileResult, nonExistingEmptyYamlAllFileErr := ReadYamlAllFile(nonExistingEmptyYamlFileName)
	assert.Error(t, nonExistingEmptyYamlAllFileErr, "Should throw an error because %s doesn't exist", nonExistingEmptyYamlFileName)
	assert.Nil(t, nonExistingEmptyYamlAllFileResult, "Empty YAML stream read incorrectly. Value should be nil because file doesn't exist.")
}

func TestReadYamlStream(t *testing.T) {
	// Setup variables
	simpleYamlDataString := `
    value: anyValue
    `
	simpleYamlDataReader := bytes.NewReader([]byte(simpleYamlDataString))
	var existingSimpleYamlConf SimpleYamlConfig

	existingReadYamlStreamErr := ReadYamlStream(simpleYamlDataReader, &existingSimpleYamlConf)
	assert.NoError(t, existingReadYamlStreamErr, "Can't read simple yaml stream: %s", existingReadYamlStreamErr)

	//Check if it can handle errors
	errorReadYamlStreamErr := ReadYamlStream(iotest.ErrReader(errors.New("timeout")), &EmptyYamlConfig{})
	assert.Error(t, errorReadYamlStreamErr, "It should throw an error because of a timeout")
}

func TestReadYamlString(t *testing.T) {
	// Setup variables
	simpleYamlDataString := `
    value: anyValue
    `
	var existingSimpleYamlConf SimpleYamlConfig

	existingReadYamlStringErr := ReadYamlString(simpleYamlDataString, &existingSimpleYamlConf)
	assert.NoError(t, existingReadYamlStringErr, "Can't read simple yaml stream: %s", existingReadYamlStringErr)
}

func TestReadYamlAllString(t *testing.T) {
	// Setup variables
	expectedSimpleYamlAllString := []any{
		map[string]any{
			"value": "anyValue",
		},
	}
	simpleYamlDataString := `
    value: anyValue
    `
	simpleYamlAllStringResult, simpleYamlAllStringErr := ReadYamlAllString(simpleYamlDataString)
	assert.NoError(t, simpleYamlAllStringErr, "Can't read simple yaml stream: %s", simpleYamlAllStringErr)

	assert.Equal(t, expectedSimpleYamlAllString, simpleYamlAllStringResult, "Simple YAML stream read incorrectly")
}

func TestReadYamlBytes(t *testing.T) {
	// Setup variables
	simpleYamlDataString := `
    value: anyValue
    `
	var existingSimpleYamlConf SimpleYamlConfig
	simpleYamlDataBytes := []byte(simpleYamlDataString)
	existingReadYamlBytesErr := ReadYamlBytes(simpleYamlDataBytes, &existingSimpleYamlConf)
	assert.NoError(t, existingReadYamlBytesErr, "Can't read simple yaml stream: %s", existingReadYamlBytesErr)
}

func TestReadYamlAllBytes(t *testing.T) {
	// Setup variables
	expectedSimpleYamlAllBytes := []any{
		map[string]any{
			"value": "anyValue",
		},
	}
	simpleYamlDataString := `
    value: anyValue
    `
	simpleYamlDataBytes := []byte(simpleYamlDataString)
	simpleYamlAllBytesResult, simpleYamlAllBytesErr := ReadYamlAllBytes(simpleYamlDataBytes)

	assert.NoError(t, simpleYamlAllBytesErr, "Can't read simple yaml stream: %s", simpleYamlAllBytesErr)
	assert.Equal(t, expectedSimpleYamlAllBytes, simpleYamlAllBytesResult, "Simple YAML stream read incorrectly")
}

func TestReadYamlAllStream(t *testing.T) {
	expectedSimpleYamlAllStream := []any{
		map[string]any{
			"value": "anyValue",
		},
	}
	// Setup variables
	simpleYamlDataReader := bytes.NewReader([]byte("value: 'anyValue'"))
	emptyYamlDataReader := bytes.NewReader([]byte(""))

	//Read empty yaml from stream
	emptyYamlAllStreamResult, emptyYamlAllStreamResultErr := ReadYamlAllStream(emptyYamlDataReader)
	assert.NoError(t, emptyYamlAllStreamResultErr, "Can't read empty yaml stream: %s", emptyYamlAllStreamResultErr)
	assert.Nil(t, emptyYamlAllStreamResult, "Empty YAML stream read incorrectly. Value should be nil")

	//Read simple yaml from stream
	simpleYamlAllStreamResult, simpleYamlAllStreamResultErr := ReadYamlAllStream(simpleYamlDataReader)
	assert.NoError(t, simpleYamlAllStreamResultErr, "Can't read simple yaml stream: %s", simpleYamlAllStreamResultErr)

	assert.Equal(t, expectedSimpleYamlAllStream, simpleYamlAllStreamResult, "Simple YAML stream read incorrectly")

	//Check if it can handle errors
	_, errorReadYamlAllStreamErr := ReadYamlAllStream(iotest.ErrReader(errors.New("timeout")))
	assert.Error(t, errorReadYamlAllStreamErr, "It should throw an error because of a timeout")
}

func TestWriteYamlAllFile(t *testing.T) {
	// Setup variables
	yamlFileName := "file.yaml"
	var yaml []any
	yaml = append(yaml, SimpleYamlConfig{
		Value: "anyValue1",
	}, SimpleYamlConfig{
		Value: "anyValue2",
	})
	expectedString := `value: anyValue1
---
value: anyValue2
`

	// Setup temporary file
	path := filepath.Join(t.TempDir(), yamlFileName)
	os.WriteFile(path, nil, 0600)

	//Check if writing multiple YAML works
	writeYamlAllFileErr := WriteYamlAllFile(path, yaml)
	assert.NoError(t, writeYamlAllFileErr, "Error while trying to write YAML to file")

	b, err := os.ReadFile(path)
	assert.NoError(t, err, "Error while reading file for evaluation")
	assert.Equal(t, expectedString, string(b), "Yaml not written correctly")
}

func TestWriteYamlFile(t *testing.T) {
	// Setup variables
	yamlFileName := "file.yaml"
	yaml := SimpleYamlConfig{
		Value: "anyValue1",
	}
	expectedString := `value: anyValue1
`

	// Setup temporary file
	path := filepath.Join(t.TempDir(), yamlFileName)

	//Check if writing a single YAML works
	writeYamlFileErr := WriteYamlFile(path, yaml)
	assert.NoError(t, writeYamlFileErr, "Error while trying to write YAML to file.")

	b, err := os.ReadFile(path)
	assert.NoError(t, err, "Error while reading file for evaluation")
	assert.Equal(t, expectedString, string(b), "Yaml not written correctly.")
}

func TestWriteYamlString(t *testing.T) {
	// Setup variables
	yaml := SimpleYamlConfig{
		Value: "anyValue1",
	}
	expectedString := `value: anyValue1
`
	// Write YAML to String
	writeYamlStringResult, writeYamlStringErr := WriteYamlString(yaml)
	assert.NoError(t, writeYamlStringErr, "Can't write simple yaml string: %s", writeYamlStringErr)
	assert.Equal(t, expectedString, writeYamlStringResult, "Yaml not written correctly.")
}

func TestWriteYamlAllString(t *testing.T) {
	// Setup variables
	var yaml []any
	yaml = append(yaml, SimpleYamlConfig{
		Value: "anyValue1",
	}, SimpleYamlConfig{
		Value: "anyValue2",
	})
	expectedString := `value: anyValue1
---
value: anyValue2
`
	// Write multiple YAML to String
	writeYamlAllStringResult, writeYamlAllStringErr := WriteYamlAllString(yaml)
	assert.NoError(t, writeYamlAllStringErr, "Can't write simple yaml string: %s", writeYamlAllStringErr)
	assert.Equal(t, expectedString, writeYamlAllStringResult, "Yaml not written correctly.")
}

func TestWriteYamlBytes(t *testing.T) {
	// Setup variables
	yaml := SimpleYamlConfig{
		Value: "anyValue1",
	}
	expectedString := `value: anyValue1
`
	expectedBytes := []byte(expectedString)
	// Write YAML to bytes
	writeYamlBytesResult, writeYamlBytesErr := WriteYamlBytes(yaml)
	assert.NoError(t, writeYamlBytesErr, "Can't write simple yaml string: %s", writeYamlBytesErr)
	assert.Equal(t, expectedBytes, writeYamlBytesResult, "Yaml not written correctly.")
}

func TestWriteYamlAllBytes(t *testing.T) {
	// Setup variables
	var yaml []any
	yaml = append(yaml, SimpleYamlConfig{
		Value: "anyValue1",
	}, SimpleYamlConfig{
		Value: "anyValue2",
	})
	expectedString := `value: anyValue1
---
value: anyValue2
`
	expectedBytes := []byte(expectedString)
	// Write multiple YAML to bytes
	writeYamlAllBytesResult, writeYamlAllBytesErr := WriteYamlAllBytes(yaml)
	assert.NoError(t, writeYamlAllBytesErr, "Can't write simple yaml string: %s", writeYamlAllBytesErr)
	assert.Equal(t, expectedBytes, writeYamlAllBytesResult, "Yaml not written correctly.")
}

func TestWriteYamlAllStream(t *testing.T) {
	// Setup variables
	var yaml []any
	yaml = append(yaml, SimpleYamlConfig{
		Value: "anyValue1",
	}, SimpleYamlConfig{
		Value: "anyValue2",
	})
	expectedString := `value: anyValue1
---
value: anyValue2
`
	var buffer bytes.Buffer
	// Write multiple YAML to stream
	writeYamlAllStreamErr := WriteYamlAllStream(&buffer, yaml)
	assert.NoError(t, writeYamlAllStreamErr, "Can't write simple yaml string: %s", writeYamlAllStreamErr)
	assert.Equal(t, expectedString, buffer.String(), "Yaml not written correctly.")
}

func TestWriteJsonString(t *testing.T) {
	yaml := SimpleYamlConfig{
		Value: "anyValue1",
	}
	expectedJson := `{"value":"anyValue1"}`
	writeJsonStringResult, writeJsonStringErr := WriteJsonString(yaml)
	assert.NoError(t, writeJsonStringErr, "Can't write yaml to json string: %s", writeJsonStringErr)
	assert.Equal(t, expectedJson, writeJsonStringResult, "Yaml not converted correctly.")
}

func TestRemoveDuplicateFields(t *testing.T) {
	// Check if duplicate field gets removed
	duplicateYaml := `value: anyValue1
value: anyValue2
`
	expectedDuplicateYaml := `value: anyValue2
`
	expectedDuplicateYamlBytes := []byte(expectedDuplicateYaml)
	duplicateFieldYamlDataReader := bytes.NewReader([]byte(duplicateYaml))
	removeDuplicateFieldsResult, removeDuplicateFieldsErr := RemoveDuplicateFields(duplicateFieldYamlDataReader)
	assert.NoError(t, removeDuplicateFieldsErr, "Can't remove duplicate fields: %s", removeDuplicateFieldsErr)
	assert.Equal(t, expectedDuplicateYamlBytes, removeDuplicateFieldsResult, "Duplicate fields not removed correctly.")

	// Check if non-duplicate fields are untouched
	nonDuplicateYaml := `value1: anyValue
value2: anyValue
`
	expectedNonDuplicateYaml := `value1: anyValue
value2: anyValue
`
	expectedNonDuplicateYamlBytes := []byte(expectedNonDuplicateYaml)

	nonDuplicateFieldYamlDataReader := bytes.NewReader([]byte(nonDuplicateYaml))
	removeNonDuplicateFieldsResult, removeNonDuplicateFieldsErr := RemoveDuplicateFields(nonDuplicateFieldYamlDataReader)
	assert.NoError(t, removeNonDuplicateFieldsErr, "Can't remove duplicate fields: %s", removeNonDuplicateFieldsErr)
	assert.Equal(t, expectedNonDuplicateYamlBytes, removeNonDuplicateFieldsResult, "Duplicate fields not removed correctly.")
}

func TestFixPathExt(t *testing.T) {
	// Check if *.yaml gets converted
	path := filepath.Join(t.TempDir(), "fix_path_ext.yml")
	os.WriteFile(path, nil, 0600)

	yamlFileName := fmt.Sprintf("%s.yaml", strings.TrimSuffix(path, filepath.Ext(path)))
	fixedYamlFileName := FixPathExt(yamlFileName)
	assert.Equal(t, path, fixedYamlFileName, "Fix of path extension failed!")

	// Check if *.yml gets converted
	path = filepath.Join(t.TempDir(), "fix_path_ext.yaml")
	os.WriteFile(path, nil, 0600)

	ymlFileName := fmt.Sprintf("%s.yml", strings.TrimSuffix(path, filepath.Ext(path)))
	fixedYmlFileName := FixPathExt(ymlFileName)
	assert.Equal(t, path, fixedYmlFileName, "Fix of path extension failed!")
}
