//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/Kishanmp3/breaklog/internal/ai"
	"github.com/Kishanmp3/breaklog/internal/db"
)

func main() {
	database, err := db.Open()
	if err != nil {
		fmt.Println("Error opening DB:", err)
		os.Exit(1)
	}
	defer database.Close()

	apiKey, err := database.GetConfig("anthropic_api_key")
	if err != nil || apiKey == "" {
		fmt.Println("No API key found. Run: breaklog config set api-key sk-ant-...")
		os.Exit(1)
	}

	diff := `--- a/main.go
+++ b/main.go
@@ -1,5 +1,12 @@
 package main

+import "fmt"
+
 func main() {
+	fmt.Println("hello, breaklog!")
 }`

	fmt.Println("Calling AI summary...")
	result, err := ai.SummarizeSession(diff, "test-project", apiKey)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	fmt.Println("\nSummary:")
	fmt.Println(result)
}
