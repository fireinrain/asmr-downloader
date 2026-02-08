package engine

import (
	"asmroner/internal/model"
	"context"
	"testing"
)

func TestEngineManager_AuthLogin(t *testing.T) {
	_, err := model.LoadConfig("/Users/sunrise/CodeGround/GolandProjects/asmroner/.asmroner-data")
	if err != nil {
		t.Errorf("LoadConfig() failed, err: %v", err)
	}
	manager, err := NewEngineManager(0.5, 1, 200, 400)
	if err != nil {
		t.Errorf("NewEngineManager() failed, err: %v", err)
	}
	manager.AuthLogin(context.TODO())
	if manager.JWTToken == "" {
		t.Errorf("AuthLogin() failed, JWTToken is empty")
	}
}
