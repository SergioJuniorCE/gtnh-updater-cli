package main

import (
    "bufio"
    "fmt"
    "os"
    "strings"
)

func main() {
    reader := bufio.NewReader(os.Stdin)
    fmt.Println("Simple Go CLI (type 'exit' to quit)")

    for {
        fmt.Print("> ")
        input, err := reader.ReadString('\n')
        if err != nil {
            fmt.Fprintf(os.Stderr, "error reading input: %v\n", err)
            os.Exit(1)
        }

        input = strings.TrimSpace(input)
        if input == "" {
            continue
        }

        switch strings.ToLower(input) {
        case "exit", "quit", "q":
            fmt.Println("Goodbye!")
            return
        case "help":
            fmt.Println("Commands: help, exit")
        default:
            fmt.Printf("You typed: %s\n", input)
        }
    }
}


