package engine

import (
	"asmroner/internal/model"
	"testing"
)

func TestEngineManager_AuthLogin(t *testing.T) {
	_, err := model.LoadConfig("/Users/sunrise/CodeGround/GolandProjects/asmroner/.asmroner-data")
	if err != nil {
		t.Errorf("LoadConfig() failed, err: %v", err)
	}
	manager := NewEngineManager()
	manager.AuthLogin()
	if manager.JWTToken == "" {
		t.Errorf("AuthLogin() failed, JWTToken is empty")
	}
}
