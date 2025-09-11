package main

import (
    "log"

    "github.com/interpretive-systems/diffium/internal/cli"
)

func main() {
    if err := cli.Execute(); err != nil {
        log.Fatal(err)
    }
}

