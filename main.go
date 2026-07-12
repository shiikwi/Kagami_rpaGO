package main

import (
	"bufio"
	"fmt"
	"kagami_rpago/internal/extractor"
	"kagami_rpago/internal/rpa"
	"os"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("Usage: kagami_rpago.exe <path to xp3.rpa>")
		return
	}
	karpa, err := rpa.Open(args[0])
	if err != nil {
		fmt.Println(err)
	}

	err = extractor.Extract(karpa, args[0])
	if err != nil {
		fmt.Println(err)
	}

	fmt.Print("Press Enter to exit...")
	_, _ = bufio.NewReader(os.Stdin).ReadBytes('\n')
}
