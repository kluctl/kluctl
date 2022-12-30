package yaml

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

type EmptyYamlConfig struct {
}

type SimpleYamlConfig struct {
	Value string `yaml:"value"`
}

func TestReadYamlFile(t *testing.T) {
	// Setup variables
	existingEmptyYamlFileNamePattern := "test_read_yaml_file_existing_empty_*.yaml"
	var existingEmptyYamlConf EmptyYamlConfig
	nonExistingEmptyYamlFileName := "test_read_yaml_file_non_existing_empty.yaml"
	var nonExistingEmptyYamlConf EmptyYamlConfig

	// Setup temporary file
	file, err := CreateTempFile(t, existingEmptyYamlFileNamePattern)
	defer file.Close()
	if err != nil {
		t.Errorf("Can't create file: %s", file.Name())
	}

	//Read existing empty yaml file
	existingEmptyYamlErr := ReadYamlFile(file.Name(), &existingEmptyYamlConf)
	if existingEmptyYamlErr != nil {
		t.Errorf("Can't read empty yaml file: %s", file.Name())
	}
	//Read non-existing empty yaml file
	nonExistingEmptyYamlErr := ReadYamlFile(nonExistingEmptyYamlFileName, &nonExistingEmptyYamlConf)
	if nonExistingEmptyYamlErr == nil {
		t.Errorf("Should throw an error because %s doesn't exist", nonExistingEmptyYamlFileName)
	}
}

func TestReadYamlAllFileFs(t *testing.T) {
	// Setup variables
	existingEmptyYamlFileNamePattern := "test_read_yaml_all_file_existing_empty_*.yaml"
	nonExistingEmptyYamlFileName := "nonExistingEmpty.yaml"

	// Setup temporary file
	file, err := CreateTempFile(t, existingEmptyYamlFileNamePattern)
	defer file.Close()
	if err != nil {
		t.Errorf("Can't create file: %s", file.Name())
	}

	//Read existing empty yaml file
	existingEmptyYamlAllFileResult, existingEmptyYamlAllFileErr := ReadYamlAllFile(file.Name())
	if existingEmptyYamlAllFileErr != nil {
		t.Errorf("Can't read empty yaml file: %s", file.Name())
	}

	if existingEmptyYamlAllFileResult != nil {
		t.Errorf("Empty YAML stream read incorrectly. Value should be nil")
	}

	//Read non-existing empty yaml file
	nonExistingEmptyYamlAllFileResult, nonExistingEmptyYamlAllFileErr := ReadYamlAllFile(nonExistingEmptyYamlFileName)
	if nonExistingEmptyYamlAllFileErr == nil {
		t.Errorf("Should throw an error because %s doesn't exist", nonExistingEmptyYamlFileName)
	}

	if nonExistingEmptyYamlAllFileResult != nil {
		t.Errorf("Empty YAML stream read incorrectly. Value should be nil because file doesn't exist.")
	}
}

func TestReadYamlStream(t *testing.T) {
	// Setup variables
	simpleYamlDataString := `
    value: anyValue
    `
	simpleYamlDataReader := bytes.NewReader([]byte(simpleYamlDataString))
	var existingSimpleYamlConf SimpleYamlConfig

	existingReadYamlStreamErr := ReadYamlStream(simpleYamlDataReader, &existingSimpleYamlConf)
	if existingReadYamlStreamErr != nil {
		t.Errorf("Can't read simple yaml stream: %s", existingReadYamlStreamErr)
	}

	//Check if it can handle errors
	errorReadYamlStreamErr := ReadYamlStream(iotest.ErrReader(errors.New("timeout")), &EmptyYamlConfig{})
	if errorReadYamlStreamErr == nil {
		t.Errorf("It should throw an error because of a timeout")
	}
}

func TestReadYamlString(t *testing.T) {
	// Setup variables
	simpleYamlDataString := `
    value: anyValue
    `
	var existingSimpleYamlConf SimpleYamlConfig

	existingReadYamlStringErr := ReadYamlString(simpleYamlDataString, &existingSimpleYamlConf)
	if existingReadYamlStringErr != nil {
		t.Errorf("Can't read simple yaml stream: %s", existingReadYamlStringErr)
	}
}

func TestReadYamlAllString(t *testing.T) {
	// Setup variables
	simpleYamlDataString := `
    value: anyValue
    `
	simpleYamlAllStringResult, simpleYamlAllStringErr := ReadYamlAllString(simpleYamlDataString)
	if simpleYamlAllStringErr != nil {
		t.Errorf("Can't read simple yaml stream: %s", simpleYamlAllStringErr)
	}
	simpleYamlAllStringResultMap := simpleYamlAllStringResult[0].(map[string]interface{})
	if simpleYamlAllStringResultMap["value"] != "anyValue" {
		t.Errorf("Simple YAML stream read incorrectly. Value should be 'anyValue' but is %s", simpleYamlAllStringResultMap["value"])
	}
}

func TestReadYamlBytes(t *testing.T) {
	// Setup variables
	simpleYamlDataString := `
    value: anyValue
    `
	var existingSimpleYamlConf SimpleYamlConfig
	simpleYamlDataBytes := []byte(simpleYamlDataString)
	existingReadYamlBytesErr := ReadYamlBytes(simpleYamlDataBytes, &existingSimpleYamlConf)
	if existingReadYamlBytesErr != nil {
		t.Errorf("Can't read simple yaml stream: %s", existingReadYamlBytesErr)
	}
}

func TestReadYamlAllBytes(t *testing.T) {
	// Setup variables
	simpleYamlDataString := `
    value: anyValue
    `
	simpleYamlDataBytes := []byte(simpleYamlDataString)
	simpleYamlAllBytesResult, simpleYamlAllBytesErr := ReadYamlAllBytes(simpleYamlDataBytes)
	if simpleYamlAllBytesErr != nil {
		t.Errorf("Can't read simple yaml stream: %s", simpleYamlAllBytesErr)
	}
	simpleYamlAllBytesResultMap := simpleYamlAllBytesResult[0].(map[string]interface{})
	if simpleYamlAllBytesResultMap["value"] != "anyValue" {
		t.Errorf("Simple YAML stream read incorrectly. Value should be 'anyValue' but is %s", simpleYamlAllBytesResultMap["value"])
	}
}

func TestReadYamlAllStream(t *testing.T) {
	// Setup variables
	simpleYamlDataReader := bytes.NewReader([]byte("value: 'anyValue'"))
	emptyYamlDataReader := bytes.NewReader([]byte(""))

	//Read empty yaml from stream
	emptyYamlAllStreamResult, emptyYamlAllStreamResultErr := ReadYamlAllStream(emptyYamlDataReader)
	if emptyYamlAllStreamResultErr != nil {
		t.Errorf("Can't read empty yaml stream: %s", emptyYamlAllStreamResultErr)
	}
	if emptyYamlAllStreamResult != nil {
		t.Errorf("Empty YAML stream read incorrectly. Value should be nil")
	}

	//Read simple yaml from stream
	simpleYamlAllStreamResult, simpleYamlAllStreamResultErr := ReadYamlAllStream(simpleYamlDataReader)
	if simpleYamlAllStreamResultErr != nil {
		t.Errorf("Can't read simple yaml stream: %s", simpleYamlAllStreamResultErr)
	}
	simpleYamlAllStreamResultMap := simpleYamlAllStreamResult[0].(map[string]interface{})
	if simpleYamlAllStreamResultMap["value"] != "anyValue" {
		t.Errorf("Simple YAML stream read incorrectly. Value should be 'anyValue' but is %s", simpleYamlAllStreamResultMap["value"])
	}

	//Check if it can handle errors
	_, errorReadYamlAllStreamErr := ReadYamlAllStream(iotest.ErrReader(errors.New("timeout")))
	if errorReadYamlAllStreamErr == nil {
		t.Errorf("It should throw an error because of a timeout")
	}
}

func TestWriteYamlAllFile(t *testing.T) {
	// Setup variables
	yamlFileNamePattern := "test_write_yaml_all_file_*.yaml"
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
	file, err := CreateTempFile(t, yamlFileNamePattern)
	defer file.Close()
	if err != nil {
		t.Errorf("Can't create file: %s", file.Name())
	}

	//Check if writing multiple YAML works
	writeYamlAllFileErr := WriteYamlAllFile(file.Name(), yaml)
	if writeYamlAllFileErr != nil {
		t.Errorf("Error while trying to write YAML to file")
	}
	b, err := os.ReadFile(file.Name())
	if err != nil {
		t.Errorf("Error while reading file for evaluation")
	}
	if string(b) != expectedString {
		t.Errorf("Yaml not written correctly. Should be \n%s\nbut is:\n%s\n", expectedString, string(b))
	}
}

func TestWriteYamlFile(t *testing.T) {
	// Setup variables
	yamlFileNamePattern := "test_write_yaml_file_*.yaml"
	yaml := SimpleYamlConfig{
		Value: "anyValue1",
	}
	expectedString := `value: anyValue1
`

	// Setup temporary file
	file, err := CreateTempFile(t, yamlFileNamePattern)
	defer file.Close()
	if err != nil {
		t.Errorf("Can't create file: %s", file.Name())
	}

	//Check if writing a single YAML works
	writeYamlFileErr := WriteYamlFile(file.Name(), yaml)
	if writeYamlFileErr != nil {
		t.Errorf("Error while trying to write YAML to file")
	}
	b, err := os.ReadFile(file.Name())
	if err != nil {
		t.Errorf("Error while reading file for evaluation")
	}
	if string(b) != expectedString {
		t.Errorf("Yaml not written correctly. Should be \n%s\nbut is:\n%s\n", expectedString, string(b))
	}
}

func TestWriteYamlString(t *testing.T) {
	// Setup variables
	yaml := SimpleYamlConfig{
		Value: "anyValue1",
	}
	expectedString := `value: anyValue1
`
	// Write YAML to String
	writeYamlAllStringResult, writeYamlAllStringErr := WriteYamlString(yaml)
	if writeYamlAllStringErr != nil {
		t.Errorf("Can't write simple yaml string: %s", writeYamlAllStringErr)
	}
	if writeYamlAllStringResult != expectedString {
		t.Errorf("Yaml not written correctly. Should be \n%s\nbut is:\n%s\n", expectedString, writeYamlAllStringResult)
	}
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
	if writeYamlAllStringErr != nil {
		t.Errorf("Can't write simple yaml string: %s", writeYamlAllStringErr)
	}
	if writeYamlAllStringResult != expectedString {
		t.Errorf("Yaml not written correctly. Should be \n%s\nbut is:\n%s\n", expectedString, writeYamlAllStringResult)
	}
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
	writeYamlAllBytesResult, writeYamlAllBytesErr := WriteYamlBytes(yaml)
	if writeYamlAllBytesErr != nil {
		t.Errorf("Can't write simple yaml string: %s", writeYamlAllBytesErr)
	}
	if bytes.Compare(writeYamlAllBytesResult, expectedBytes) != 0 {
		t.Errorf("Yaml not written correctly. Should be \n%s\nbut is:\n%s\n", expectedBytes, writeYamlAllBytesResult)
	}
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
	if writeYamlAllBytesErr != nil {
		t.Errorf("Can't write simple yaml string: %s", writeYamlAllBytesErr)
	}
	if bytes.Compare(writeYamlAllBytesResult, expectedBytes) != 0 {
		t.Errorf("Yaml not written correctly. Should be \n%s\nbut is:\n%s\n", expectedBytes, writeYamlAllBytesResult)
	}
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
	if writeYamlAllStreamErr != nil {
		t.Errorf("Can't write simple yaml string: %s", writeYamlAllStreamErr)
	}
	if buffer.String() != expectedString {
		t.Errorf("Yaml not written correctly. Should be \n%s\nbut is:\n%s\n", expectedString, buffer.String())
	}
}

func TestConvertYamlToJson(t *testing.T) {
	yaml := `value: anyValue1`
	expectedJson := `{"value":"anyValue1"}`
	yamlBytes := []byte(yaml)
	expectedJsonBytes := []byte(expectedJson)
	jsonBytes, convertYamlToJsonErr := ConvertYamlToJson(yamlBytes)
	if convertYamlToJsonErr != nil {
		t.Errorf("Can't convert yaml to json: %s", convertYamlToJsonErr)
	}
	if bytes.Compare(jsonBytes, expectedJsonBytes) != 0 {
		t.Errorf("Yaml not converted correctly. Should be \n%s\nbut is:\n%s\n", expectedJsonBytes, jsonBytes)
	}
}

func TestWriteJsonString(t *testing.T) {
	yaml := SimpleYamlConfig{
		Value: "anyValue1",
	}
	expectedJson := `{"value":"anyValue1"}`
	writeJsonStringResult, writeJsonStringErr := WriteJsonString(yaml)
	if writeJsonStringErr != nil {
		t.Errorf("Can't write yaml to json string: %s", writeJsonStringErr)
	}
	if writeJsonStringResult != expectedJson {
		t.Errorf("Yaml not converted correctly. Should be \n%s\nbut is:\n%s\n", expectedJson, writeJsonStringResult)
	}
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
	if removeDuplicateFieldsErr != nil {
		t.Errorf("Can't remove duplicate fields: %s", removeDuplicateFieldsErr)
	}
	if bytes.Compare(removeDuplicateFieldsResult, expectedDuplicateYamlBytes) != 0 {
		t.Errorf("Duplicate fields not removed correctly. Should be \n%s\nbut is:\n%s\n", expectedDuplicateYamlBytes, removeDuplicateFieldsResult)
	}

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
	if removeNonDuplicateFieldsErr != nil {
		t.Errorf("Can't remove duplicate fields: %s", removeNonDuplicateFieldsErr)
	}
	if bytes.Compare(removeNonDuplicateFieldsResult, expectedNonDuplicateYamlBytes) != 0 {
		t.Errorf("Duplicate fields not removed correctly. Should be \n%s\nbut is:\n%s\n", expectedNonDuplicateYamlBytes, removeNonDuplicateFieldsResult)
	}
}

func TestFixPathExt(t *testing.T) {
	// Check if *.yaml gets converted
	file, err := CreateTempFile(t, "test_fix_path_ext_*.yml")
	defer file.Close()
	if err != nil {
		t.Errorf("Can't create file: %s", file.Name())
	}

	yamlFileName := fmt.Sprintf("%s.yaml", strings.TrimSuffix(file.Name(), filepath.Ext(file.Name())))
	fixedYamlFileName := FixPathExt(yamlFileName)
	if fixedYamlFileName != file.Name() {
		t.Errorf("Fix of path extension failed! Should be %s but is %s", file.Name(), fixedYamlFileName)
	}

	// Check if *.yml gets converted
	file, err = CreateTempFile(t, "test_fix_path_ext_*.yaml")
	defer file.Close()
	if err != nil {
		t.Errorf("Can't create file: %s", file.Name())
	}

	ymlFileName := fmt.Sprintf("%s.yml", strings.TrimSuffix(file.Name(), filepath.Ext(file.Name())))
	fixedYmlFileName := FixPathExt(ymlFileName)
	if fixedYmlFileName != file.Name() {
		t.Errorf("Fix of path extension failed! Should be %s but is %s", file.Name(), fixedYmlFileName)
	}
}

func CreateTempFile(t *testing.T, pattern string) (*os.File, error) {
	file, err := os.CreateTemp(t.TempDir(), pattern)
	if err != nil {
		return nil, err
	}
	return file, nil
}
