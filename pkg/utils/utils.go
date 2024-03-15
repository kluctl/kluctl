package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/jinzhu/copier"
	"strconv"
)

func Sha256String(data string) string {
	return Sha256Bytes([]byte(data))
}

func Sha256Bytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func DeepCopy(dst interface{}, src interface{}) error {
	return copier.CopyWithOption(dst, src, copier.Option{
		DeepCopy: true,
	})
}

func DeepClone[T any](src *T) (*T, error) {
	ret := new(T)
	err := DeepCopy(ret, src)
	if err != nil {
		return ret, err
	}
	return ret, nil
}

func FindStrInSlice(a []string, s string) int {
	for i, v := range a {
		if v == s {
			return i
		}
	}
	return -1
}

func ParseBoolOrFalsePtr(s *string) bool {
	if s == nil {
		return false
	}
	return ParseBoolOrFalse(*s)
}

func ParseBoolOrFalse(s string) bool {
	return ParseBoolOrDefault(s, false)
}

func ParseBoolOrDefault(s string, def bool) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return def
	}
	return b
}

func Ptr[T any](v T) *T {
	return &v
}

func StrPtrEquals(s1 *string, s2 *string) bool {
	if s1 == nil && s2 == nil {
		return true
	}
	if (s1 == nil) != (s2 == nil) {
		return false
	}
	return *s1 == *s2
}
