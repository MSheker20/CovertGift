package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Kullanım: go run . <URL> [URL...]")
		fmt.Println(`Örnek:   go run . "https://ty.gl/gkyyduf4wm8u3" "https://amzn.eu/d/08B1hxDx"`)
		return
	}

	for _, u := range os.Args[1:] {
		fmt.Printf("\n==================================================\n")
		fmt.Printf("🔍 %s\n", u)
		fmt.Printf("==================================================\n")

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		p, err := Fetch(ctx, u)
		cancel()
		if err != nil {
			fmt.Printf("❌ %v\n", err)
		}
		printPreview(p)

		// API'nin döneceği gövde
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"success": true, "data": p})
	}
}
