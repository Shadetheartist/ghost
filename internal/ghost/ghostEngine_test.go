package ghost_test

import (
	"internal/ghost"
	"testing"
	"time"
)

func createTestEngine() *ghost.Engine {
	return ghost.NewEngine(10, 5, time.Second)
}

func TestHalt(t *testing.T) {
	engine := createTestEngine()

	go engine.Run()

	engine.RegisterRequest(ghost.NewRequest())
	engine.RegisterRequest(ghost.NewRequest())
	engine.RegisterRequest(ghost.NewRequest())

	engine.Halt()
}
