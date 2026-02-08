package engine

import (
	"asmroner/internal/model"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEngineManager_AuthLogin(t *testing.T) {
	// Use a relative path from the project root
	configDir := filepath.Join(os.Getenv("HOME"), ".asmroner-data")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Skip("Config directory not found, skipping integration test")
	}
	_, err := model.LoadConfig(configDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed, err: %v", err)
	}
	manager, err := NewEngineManager(0.5, 1, 200, 400)
	if err != nil {
		t.Fatalf("NewEngineManager() failed, err: %v", err)
	}
	if err := manager.AuthLogin(context.TODO()); err != nil {
		t.Fatalf("AuthLogin() failed, err: %v", err)
	}
	if manager.JWTToken == "" {
		t.Error("AuthLogin() failed, JWTToken is empty")
	}
}
