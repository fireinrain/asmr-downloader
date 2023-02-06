package storage

import (
	"testing"
)

func TestGetDbInstance(t *testing.T) {
	instance := GetDbInstance()
	println(instance)
}
