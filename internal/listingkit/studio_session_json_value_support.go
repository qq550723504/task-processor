package listingkit

import "database/sql/driver"

type SheinStudioSelectionVariants []SheinStudioSelectionVariant

func (value SheinStudioSelectionVariants) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioSelectionVariants) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioSelectionSnapshot SheinStudioSelection

func (value SheinStudioSelectionSnapshot) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioSelectionSnapshot) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioInt64List []int64

func (value SheinStudioInt64List) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioInt64List) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioStringList []string

func (value SheinStudioStringList) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioStringList) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioProductImagePromptList []SheinStudioProductImagePrompt

func (value SheinStudioProductImagePromptList) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioProductImagePromptList) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioSelectedSDSImageList []SheinStudioSelectedSDSImageRecord

func (value SheinStudioSelectedSDSImageList) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioSelectedSDSImageList) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioCreatedTaskList []SheinStudioCreatedTask

func (value SheinStudioCreatedTaskList) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioCreatedTaskList) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioFailedTaskList []SheinStudioFailedTask

func (value SheinStudioFailedTaskList) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioFailedTaskList) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioGenerationJobList []SheinStudioGenerationJob

func (value SheinStudioGenerationJobList) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioGenerationJobList) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioGroupedSelectionList []SheinStudioGroupedSelection

func (value SheinStudioGroupedSelectionList) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioGroupedSelectionList) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}
