package main

import (
	"context"
	"log"

	"task-processor/internal/app/runtime/listingkitownerreconcile"
)

func main() {
	if err := listingkitownerreconcile.Run(context.Background(), listingkitownerreconcile.ParseFlags()); err != nil {
		log.Fatal(err)
	}
}
