package main

import (
	"context"
	"fmt"
	"os"

	"ironclaw/internal/brain"
	"ironclaw/internal/memory"
)

// echoProvider echoes the prompt it receives (shows what the LLM would see)
type echoProvider struct{}

func (e *echoProvider) Generate(_ context.Context, prompt string) (string, error) {
	return prompt, nil
}

func main() {
	dir := "/tmp/ironclaw-brain-test"
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	store := memory.NewFileMemoryStore(dir)
	store.Remember("My favorite color is blue.")

	b := brain.NewBrain(&echoProvider{}, brain.WithMemory(store))
	result, _ := b.Generate(context.Background(), "What is my favorite color?")
	fmt.Println("=== Prompt sent to LLM ===")
	fmt.Println(result)
}
