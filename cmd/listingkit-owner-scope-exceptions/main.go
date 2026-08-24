package main

import (
	"context"
	"log"

	"task-processor/internal/app/runtime/listingkitownerexceptions"
)

func main() {
	if err := listingkitownerexceptions.Run(context.Background(), listingkitownerexceptions.ParseFlags()); err != nil {
		log.Fatal(err)
	}
}
