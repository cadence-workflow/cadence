package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Hello from Cadence sample!")
	
	// Print environment variables to verify container configuration
	fmt.Printf("CADENCE_HOST=%s\n", os.Getenv("CADENCE_HOST"))
	fmt.Printf("CADENCE_PORT=%s\n", os.Getenv("CADENCE_PORT"))
}