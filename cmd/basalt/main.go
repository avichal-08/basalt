package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/avichal-08/basalt/internal/store"
)

func main() {
	s, err := store.NewDiskStore("database.aof")
	if err != nil {
		fmt.Println("Failed to initialize database:", err)
		os.Exit(1)
	}
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Basalt KV Store")
	fmt.Println("Type HELP to see available commands")

	for {
		fmt.Print("basalt> ")

		if !scanner.Scan() {
			fmt.Println("BYE!")
			return
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		command := strings.ToUpper(parts[0])

		switch command {

		case "SET":
			if len(parts) != 3 {
				fmt.Println("usage:SET <key> <value>")
				continue
			}
			key := parts[1]
			value := parts[2]

			s.Set(key, value)
			fmt.Println("DONE!")

		case "GET":
			if len(parts) != 2 {
				fmt.Println("usage: GET <key>")
				continue
			}

			key := parts[1]
			value, ok := s.Get(key)

			if !ok {
				fmt.Println("key not found")
				continue
			}
			fmt.Println(value)

		case "DELETE":
			if len(parts) != 2 {
				fmt.Println("usage: DELETE <key>")
				continue
			}

			key := parts[1]
			ok := s.Delete(key)
			if !ok {
				fmt.Println("key not found")
				continue
			}
			fmt.Println("DONE!")

		case "HELP":
			fmt.Println("Available Commands:")
			fmt.Println("  SET <key> <value>")
			fmt.Println("  GET <key>")
			fmt.Println("  DELETE <key>")
			fmt.Println("  HELP")
			fmt.Println("  EXIT")

		case "EXIT":
			fmt.Println("BYE!")
			return

		default:
			fmt.Println("unknown command")
		}
	}
}
