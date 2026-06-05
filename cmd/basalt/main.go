package main

import (
	"fmt"

	"github.com/avichal-08/basalt/internal/store"
)

func main() {
	s := store.New()

	s.Set("name", "avichal")
	s.Set("city", "lucknow")

	name, ok := s.Get("name")
	if ok {
		fmt.Printf("name = %s\n", name)
	}

	s.Set("name", "aditya")

	updatedName, ok := s.Get("name")
	if ok {
		fmt.Printf("updated name = %s\n", updatedName)
	}

	s.Delete("city")

	_, ok = s.Get("city")
	if !ok {
		fmt.Println("city key not found")
	}
}
