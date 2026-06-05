package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/avichal-08/basalt/internal/store"
)

func main() {

	s := store.New()

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Basalt KV Store")
	fmt.Println("Type HELP to see available commands")

	for {
		fmt.Println("basalt>")

		if !scanner.Scan() {
			fmt.Println("bye!!")
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

			fmt.Println("done!")

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

			s.Delete(key)

			fmt.Println("OK")

		case "HELP":
			fmt.Println("Available Commands:")
			fmt.Println("  SET <key> <value>")
			fmt.Println("  GET <key>")
			fmt.Println("  DELETE <key>")
			fmt.Println("  HELP")
			fmt.Println("  EXIT")

		case "EXIT":
			fmt.Println("bye!")
			return

		default:
			fmt.Println("unknown command")

		}
	}
}
