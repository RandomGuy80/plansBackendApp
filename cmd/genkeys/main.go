package main

import (
	"fmt"
	"log"
	"plans-backend/internal/handlers"
)

func main() {
	pub, priv, err := handlers.GenerateVAPIDKeys()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Добавь в .env:")
	fmt.Println()
	fmt.Printf("VAPID_PUBLIC_KEY=%s\n", pub)
	fmt.Printf("VAPID_PRIVATE_KEY=%s\n", priv)
}
