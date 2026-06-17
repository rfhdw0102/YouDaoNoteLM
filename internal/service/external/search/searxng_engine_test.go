package search

import (
	"testing"
)

func TestSearXNGEngine_Name(t *testing.T) {
	engine := NewSearXNGEngine("http://localhost:8888")
	if engine.Name() != "searxng" {
		t.Errorf("expected 'searxng', got '%s'", engine.Name())
	}
}

func TestCustomEngine_ImplementsInterface(t *testing.T) {
	var _ = NewCustomEngine("test", "http://localhost", "key")
}

func TestSearXNGEngine_ImplementsInterface(t *testing.T) {
	var _ = NewSearXNGEngine("http://localhost")
}
